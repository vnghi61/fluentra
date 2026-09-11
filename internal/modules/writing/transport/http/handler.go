// Package http provides HTTP endpoints for writing feedback.
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Handler serves writing HTTP endpoints.
type Handler struct {
	reader contract.FeedbackReader
}

// NewHandler constructs a Handler for writing endpoints.
func NewHandler(reader contract.FeedbackReader) *Handler {
	return &Handler{
		reader: reader,
	}
}

// Routes mounts the writing endpoints on the router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/writing/attempts/{id}/feedback", h.getWritingFeedback)
}

func (h *Handler) getWritingFeedback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "attempt id must be a valid UUID"))
		return
	}

	if h.reader == nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.NotFound, "FEEDBACK_NOT_FOUND", "writing feedback not found"))
		return
	}

	feedback, err := h.reader.GetWritingFeedback(ctx, attemptID, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, feedback)
}
