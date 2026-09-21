package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// ResourceService declares operations needed by the HTTP transport.
type ResourceService interface {
	CreateUploadIntent(
		ctx context.Context, userID uuid.UUID, filename, declaredMIME string,
	) (*contract.UploadIntentResult, error)
	ConfirmUpload(ctx context.Context, userID, resourceID uuid.UUID) (*contract.SubmitResult, error)
	SubmitURL(ctx context.Context, userID uuid.UUID, rawURL, title string) (*contract.SubmitResult, error)
	ListResources(
		ctx context.Context, userID uuid.UUID, status, kind *string, page, pageSize int,
	) (*contract.ResourceList, error)
	GetResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error)
	DeleteResource(ctx context.Context, id, userID uuid.UUID) error
}

// Handler handles HTTP requests for resource intake and management.
type Handler struct {
	service ResourceService
}

// NewHandler constructs a new Handler.
func NewHandler(service ResourceService) *Handler {
	return &Handler{service: service}
}

// Routes mounts the 5 endpoints on the authenticated router.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/me/resources/upload-intent", h.uploadIntent)
	r.Post("/me/resources", h.submit)
	r.Get("/me/resources", h.list)
	r.Get("/me/resources/{id}", h.get)
	r.Delete("/me/resources/{id}", h.delete)
}

func (h *Handler) uploadIntent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	var req createUploadIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_REQUEST_BODY", "Request body must be valid JSON"))
		return
	}

	res, err := h.service.CreateUploadIntent(ctx, actor.UserID, req.Filename, req.ContentType)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, uploadIntentResponse{
		ID:        res.ID,
		UploadURL: res.UploadURL,
		ObjectKey: res.ObjectKey,
		ExpiresAt: res.ExpiresAt,
	})
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	var req submitResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_REQUEST_BODY", "Request body must be valid JSON"))
		return
	}

	var res *contract.SubmitResult
	var err error

	if req.ResourceID != nil && *req.ResourceID != uuid.Nil {
		res, err = h.service.ConfirmUpload(ctx, actor.UserID, *req.ResourceID)
	} else if req.URL != nil && *req.URL != "" {
		title := ""
		if req.Title != nil {
			title = *req.Title
		}
		res, err = h.service.SubmitURL(ctx, actor.UserID, *req.URL, title)
	} else {
		err := apperr.New(apperr.BadRequest, "INVALID_REQUEST", "Either resource_id or url must be provided")
		httpx.WriteProblem(w, r, err)
		return
	}

	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusAccepted, submitResourceResponse{
		ID:     res.ID,
		Status: res.Status,
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	query := r.URL.Query()
	page := 1
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = p
	}
	pageSize := 20
	if ps, err := strconv.Atoi(query.Get("page_size")); err == nil && ps > 0 {
		pageSize = ps
	}

	var statusPtr *string
	if s := query.Get("status"); s != "" {
		statusPtr = &s
	}
	var kindPtr *string
	if k := query.Get("kind"); k != "" {
		kindPtr = &k
	}

	res, err := h.service.ListResources(ctx, actor.UserID, statusPtr, kindPtr, page, pageSize)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toResourceListResponse(res))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_RESOURCE_ID", "Invalid resource ID format"))
		return
	}

	res, err := h.service.GetResource(ctx, id, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toResourceResponse(*res))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_RESOURCE_ID", "Invalid resource ID format"))
		return
	}

	if err := h.service.DeleteResource(ctx, id, actor.UserID); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
