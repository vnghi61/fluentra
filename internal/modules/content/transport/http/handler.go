package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Permissions required by content operations.
const (
	PermContentReadPublished = "content.read.published"
	PermContentCreate        = "content.create"
	PermContentEdit          = "content.edit"
	PermContentReview        = "content.review"
	PermContentPublish       = "content.publish"
)

// Guard is the authorization interface required by content handlers.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// ContentService defines the use cases called by HTTP handlers.
type ContentService interface {
	GetPublishedVersionBySlug(ctx context.Context, slug string) (*contract.Version, error)
	Browse(ctx context.Context, filter contract.BrowseFilter) ([]*contract.Version, int, error)
	CreateItem(ctx context.Context, actorID uuid.UUID, req service.CreateItemRequest) (domain.Item, domain.Version, error)
	UpdateDraft(ctx context.Context, actorID, itemID uuid.UUID, req service.UpdateDraftRequest) (domain.Version, error)
	SubmitForReview(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error)
	Review(ctx context.Context, reviewerID, itemID uuid.UUID, req service.ReviewDecisionRequest) (domain.Version, error)
	Publish(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error)
	Archive(ctx context.Context, actorID, itemID uuid.UUID) (domain.Item, error)
	ListAdminItems(
		ctx context.Context, status, kind, query *string, limit, offset int,
	) ([]domain.Item, int64, error)
	GetAdminItemDetail(ctx context.Context, id uuid.UUID) (domain.Item, []domain.Version, error)
	ReportItem(ctx context.Context, userID, versionID uuid.UUID, reason domain.ReportReason, note *string) (domain.ItemReport, error)
	ListReportedContent(ctx context.Context, limit, offset int) ([]domain.ReportedVersionSummary, int, error)
}

// Handler serves HTTP endpoints for the content module.
type Handler struct {
	service ContentService
	guard   Guard
}

// NewHandler constructs a new Handler. It fails closed if guard is nil
// so admin authoring endpoints cannot be left unprotected by accident.
func NewHandler(service ContentService, guard Guard) (*Handler, error) {
	if guard == nil {
		return nil, apperr.New(apperr.Internal, "GUARD_REQUIRED", "authorization guard is required for content handlers")
	}
	return &Handler{
		service: service,
		guard:   guard,
	}, nil
}

// Routes mounts learner-facing content endpoints under the authenticated router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/content", h.browse)
	router.Get("/content/{slug}", h.getBySlug)
	router.Post("/content/versions/{id}/reports", h.reportItem)
}

// AdminRoutes mounts staff/authoring content endpoints under the admin router.
func (h *Handler) AdminRoutes(router chi.Router) {
	router.Get("/admin/content", h.adminListContent)
	router.Get("/admin/content/reports", h.adminListReports)
	router.Get("/admin/content/{id}", h.adminGetContent)
	router.Post("/admin/content", h.createItem)
	router.Put("/admin/content/{id}/draft", h.updateDraft)
	router.Post("/admin/content/{id}/submit", h.submitForReview)
	router.Post("/admin/content/{id}/review", h.review)
	router.Post("/admin/content/{id}/publish", h.publish)
	router.Post("/admin/content/{id}/archive", h.archive)
}

func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	filter := contract.BrowseFilter{}
	if kind := r.URL.Query().Get("kind"); kind != "" {
		filter.Kind = &kind
	}
	if level := r.URL.Query().Get("cefr_level"); level != "" {
		filter.CEFRLevel = &level
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	versions, total, err := h.service.Browse(ctx, filter)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	items := make([]ContentVersionResponse, len(versions))
	for i, v := range versions {
		items[i] = toContentVersionResponse(v)
	}

	httpx.WriteJSON(w, r, http.StatusOK, ContentVersionListResponse{
		Items: items,
		Total: total,
	})
}

func (h *Handler) getBySlug(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		httpx.WriteProblem(w, r, domain.ErrInvalidSlug)
		return
	}

	v, err := h.service.GetPublishedVersionBySlug(ctx, slug)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toContentVersionResponse(v))
}

func (h *Handler) createItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentCreate); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	var req CreateContentItemRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	item, _, err := h.service.CreateItem(ctx, actor.UserID, service.CreateItemRequest{
		Kind:      req.Kind,
		Slug:      req.Slug,
		CEFRLevel: req.CEFRLevel,
		Body:      req.Body,
		Tags:      req.Tags,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, toContentItemResponse(item))
}

func (h *Handler) updateDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentEdit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content item ID."))
		return
	}

	var req UpdateDraftRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	ver, err := h.service.UpdateDraft(ctx, actor.UserID, itemID, service.UpdateDraftRequest{
		CEFRLevel: req.CEFRLevel,
		Body:      req.Body,
		Tags:      req.Tags,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toDomainVersionResponse(ver))
}

func (h *Handler) submitForReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentEdit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content item ID."))
		return
	}

	ver, err := h.service.SubmitForReview(ctx, actor.UserID, itemID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toDomainVersionResponse(ver))
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReview); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content item ID."))
		return
	}

	var req ReviewDecisionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	decision, err := domain.ParseReviewDecision(req.Decision)
	if err != nil {
		httpx.WriteProblem(w, r, domain.ErrInvalidReviewDecision)
		return
	}

	ver, err := h.service.Review(ctx, actor.UserID, itemID, service.ReviewDecisionRequest{
		Decision: decision,
		Comments: req.Comments,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toDomainVersionResponse(ver))
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentPublish); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content item ID."))
		return
	}

	ver, err := h.service.Publish(ctx, actor.UserID, itemID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toDomainVersionResponse(ver))
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentPublish); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content item ID."))
		return
	}

	item, err := h.service.Archive(ctx, actor.UserID, itemID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toContentItemResponse(item))
}

// adminPaging reads the page window off the query string. Zero means "not
// supplied"; domain.NormaliseLimit turns that into the documented default and
// bounds the rest.
//
// ParseInt with a 32-bit size, not Atoi — the same choice vocabulary's `paging`
// already makes, and for the same reason: Atoi returns a platform int, so
// `?limit=99999999999` parses cleanly on a 64-bit build and travels two packages
// before anything notices. Enforcing the width where the string is read means an
// oversized value is ignored at its source.
func adminPaging(r *http.Request) (limit, offset int) {
	if val, err := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 32); err == nil && val > 0 {
		limit = int(val)
	}
	if val, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 32); err == nil && val > 0 {
		offset = int(val)
	}
	return limit, offset
}

func (h *Handler) adminListContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentEdit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	var statusPtr *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}
	var kindPtr *string
	if k := r.URL.Query().Get("kind"); k != "" {
		kindPtr = &k
	}
	var queryPtr *string
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		queryPtr = &q
	}
	limit, offset := adminPaging(r)

	items, total, err := h.service.ListAdminItems(ctx, statusPtr, kindPtr, queryPtr, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	respItems := make([]ContentItemResponse, len(items))
	for i, item := range items {
		respItems[i] = toContentItemResponse(item)
	}

	httpx.WriteJSON(w, r, http.StatusOK, AdminContentItemListResponse{
		Items:  respItems,
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
	})
}

func (h *Handler) adminGetContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentEdit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	idStr := chi.URLParam(r, "id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content item ID."))
		return
	}

	item, versions, err := h.service.GetAdminItemDetail(ctx, itemID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	respVersions := make([]ContentVersionResponse, len(versions))
	for i, v := range versions {
		respVersions[i] = toDomainVersionResponse(v)
	}

	httpx.WriteJSON(w, r, http.StatusOK, AdminContentItemDetailResponse{
		ContentItemResponse: toContentItemResponse(item),
		Versions:            respVersions,
	})
}

// reportItem handles POST /content/versions/{id}/reports
func (h *Handler) reportItem(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	versionID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid content version ID."))
		return
	}

	var req struct {
		Reason string  `json:"reason"`
		Note   *string `json:"note"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	if !domain.IsValidReportReason(req.Reason) {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REASON", "Invalid report reason."))
		return
	}

	report, err := h.service.ReportItem(r.Context(), actor.UserID, versionID, domain.ReportReason(req.Reason), req.Note)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, ItemReportResponse{
		ID:               report.ID,
		ContentVersionID: report.ContentVersionID,
		UserID:           report.UserID,
		Reason:           string(report.Reason),
		Note:             report.Note,
		CreatedAt:        report.CreatedAt,
	})
}

// adminListReports handles GET /admin/content/reports
func (h *Handler) adminListReports(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReview); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	items, total, err := h.service.ListReportedContent(ctx, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	respItems := make([]ReportedContentVersionResponse, len(items))
	for i, it := range items {
		respItems[i] = ReportedContentVersionResponse{
			ContentVersionID: it.ContentVersionID,
			ItemID:           it.ItemID,
			Slug:             it.Slug,
			Kind:             it.Kind,
			CEFRLevel:        it.CEFRLevel,
			ItemStatus:       it.ItemStatus,
			ReportCount:      it.ReportCount,
			LastReportedAt:   it.LastReportedAt,
		}
	}

	httpx.WriteJSON(w, r, http.StatusOK, ReportedContentListResponse{
		Items:  respItems,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

