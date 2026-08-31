package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// The learner-upload endpoints.
//
// Every one acts on the caller in the access token and never on a user id in
// the path: an upload is somebody's own vocabulary, and there is no route here
// through which one learner can read another's.

// UploadService is what the handler needs from the upload pipeline.
type UploadService interface {
	Submit(ctx context.Context, userID uuid.UUID, rawText string) (service.Upload, error)
	List(ctx context.Context, userID uuid.UUID, limit int32) ([]service.Upload, error)
	Get(ctx context.Context, userID, uploadID uuid.UUID) (service.Upload, error)
}

const (
	defaultUploadListLimit = 20
	maxUploadListLimit     = 100
)

// SubmitUploadRequest is a paste of vocabulary.
type SubmitUploadRequest struct {
	Text string `json:"text"`
}

// UploadRoutes mounts the upload endpoints.
func (h *Handler) UploadRoutes(router chi.Router) {
	if h.uploads == nil {
		return
	}
	router.Post("/me/vocabulary/uploads", h.submitUpload)
	router.Get("/me/vocabulary/uploads", h.listUploads)
	router.Get("/me/vocabulary/uploads/{id}", h.getUpload)
}

func (h *Handler) submitUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	var req SubmitUploadRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	upload, err := h.uploads.Submit(r.Context(), actor.UserID, req.Text)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	// 202, not 201: the words are stored but not yet checked, and a 201 would
	// tell the client the work is done when the verification job has not run.
	httpx.WriteJSON(w, r, http.StatusAccepted, upload)
}

func (h *Handler) listUploads(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	limit := int32(defaultUploadListLimit)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 32); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if limit > maxUploadListLimit {
		limit = maxUploadListLimit
	}

	uploads, err := h.uploads.List(r.Context(), actor.UserID, limit)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{"items": uploads})
}

func (h *Handler) getUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	uploadID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.BadRequest, "INVALID_UPLOAD_ID", "Upload id must be a valid UUID."))
		return
	}

	// The service scopes the read to the caller, so another learner's id is a
	// 404 rather than a 403 — which is the right answer: they should not learn
	// that the upload exists.
	upload, err := h.uploads.Get(r.Context(), actor.UserID, uploadID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, upload)
}
