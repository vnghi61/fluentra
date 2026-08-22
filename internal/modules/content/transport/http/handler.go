package http

import (
	"context"
	"net/http"
	"strconv"

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
	EstimateLevel(ctx context.Context, actorID, itemID uuid.UUID) (string, error)
}

// Handler serves HTTP endpoints for the content module.
type Handler struct {
	service ContentService
	guard   Guard
}

// NewHandler constructs a new Handler.
func NewHandler(service ContentService, guard Guard) *Handler {
	return &Handler{
		service: service,
		guard:   guard,
	}
}

// Routes mounts learner-facing content endpoints under the authenticated router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/content", h.browse)
	router.Get("/content/{slug}", h.getBySlug)
}

// AdminRoutes mounts staff/authoring content endpoints under the admin router.
func (h *Handler) AdminRoutes(router chi.Router) {
	router.Post("/admin/content", h.createItem)
	router.Put("/admin/content/{id}/draft", h.updateDraft)
	router.Post("/admin/content/{id}/submit", h.submitForReview)
	router.Post("/admin/content/{id}/review", h.review)
	router.Post("/admin/content/{id}/publish", h.publish)
	router.Post("/admin/content/{id}/archive", h.archive)
	router.Post("/admin/content/{id}/estimate-level", h.estimateLevel)
}

func (h *Handler) browse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentCreate); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentEdit); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentEdit); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentReview); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentPublish); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentPublish); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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

func (h *Handler) estimateLevel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.guard != nil {
		if err := h.guard.Require(ctx, PermContentEdit); err != nil {
			httpx.WriteProblem(w, r, err)
			return
		}
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

	level, err := h.service.EstimateLevel(ctx, actor.UserID, itemID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, EstimateLevelResponse{EstimatedLevel: level})
}
