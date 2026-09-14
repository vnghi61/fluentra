// Package http provides the HTTP endpoints for the listening module.
package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/listening/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// ListeningService defines the service operations required by the HTTP handler.
type ListeningService interface {
	RecordPlay(
		ctx context.Context, userID, versionID uuid.UUID, contextType string, contextID uuid.UUID,
	) (*domain.PlayResult, error)
	GetTranscript(ctx context.Context, userID, versionID, attemptID uuid.UUID) (string, error)
}

// Handler handles HTTP requests for listening exercises.
type Handler struct {
	service ListeningService
}

// NewHandler constructs a new listening HTTP handler.
func NewHandler(service ListeningService) *Handler {
	return &Handler{service: service}
}

// Routes mounts listening endpoints under the given chi router.
func (h *Handler) Routes(router chi.Router) {
	router.Post("/listening/items/{versionId}/plays", h.recordPlay)
	router.Get("/listening/items/{versionId}/transcript", h.getTranscript)
}

type recordPlayRequest struct {
	ContextType string    `json:"context_type"`
	ContextID   uuid.UUID `json:"context_id"`
}

func (h *Handler) recordPlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	versionIDStr := chi.URLParam(r, "versionId")
	versionID, err := uuid.Parse(versionIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_VERSION_ID", "versionId must be a valid UUID"))
		return
	}

	var req recordPlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_REQUEST_BODY", "failed to parse request body"))
		return
	}

	if req.ContextID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_CONTEXT_ID", "context_id must be a non-nil UUID"))
		return
	}

	result, err := h.service.RecordPlay(ctx, actor.UserID, versionID, req.ContextType, req.ContextID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, result)
}

func (h *Handler) getTranscript(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	versionIDStr := chi.URLParam(r, "versionId")
	versionID, err := uuid.Parse(versionIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_VERSION_ID", "versionId must be a valid UUID"))
		return
	}

	attemptIDStr := r.URL.Query().Get("attempt_id")
	if attemptIDStr == "" {
		httpx.WriteProblem(
			w, r, apperr.New(apperr.BadRequest, "ATTEMPT_ID_REQUIRED", "attempt_id query parameter is required"),
		)
		return
	}
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "attempt_id must be a valid UUID"))
		return
	}

	script, err := h.service.GetTranscript(ctx, actor.UserID, versionID, attemptID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]string{
		"script": script,
	})
}
