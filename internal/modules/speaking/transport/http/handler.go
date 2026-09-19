// Package http provides HTTP handlers for the speaking module.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// SpeakingService defines operations needed by the HTTP transport layer.
type SpeakingService interface {
	UploadIntent(ctx context.Context, userID uuid.UUID, contentType string) (*contract.UploadIntentResult, error)
	DeleteAttemptRecording(ctx context.Context, attemptID, userID uuid.UUID) error
	GetSpeakingFeedback(ctx context.Context, attemptID, userID uuid.UUID) (*contract.SpeakingFeedback, error)
	ListSpeakingSubmissions(
		ctx context.Context, userID uuid.UUID, page, pageSize int,
	) (*contract.SpeakingSubmissionList, error)
	GetConsent(ctx context.Context, userID uuid.UUID) (*contract.SpeakingConsent, error)
	RecordConsent(ctx context.Context, userID uuid.UUID) (*contract.SpeakingConsent, error)
}

// Handler serves speaking module HTTP requests.
type Handler struct {
	service SpeakingService
}

// NewHandler constructs a new Handler.
func NewHandler(service SpeakingService) *Handler {
	return &Handler{service: service}
}

// Routes registers speaking endpoints on the router.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/speaking/upload-intent", h.uploadIntent)
	r.Delete("/speaking/attempts/{id}/recording", h.deleteRecording)
	r.Get("/speaking/attempts/{id}/feedback", h.getFeedback)
	r.Get("/speaking/submissions", h.listSubmissions)
	r.Get("/speaking/consent", h.getConsent)
	r.Post("/speaking/consent", h.recordConsent)
}

type uploadIntentRequest struct {
	ContentType string `json:"content_type"`
}

func (h *Handler) uploadIntent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	var req uploadIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_REQUEST_BODY", "failed to parse request body"))
		return
	}

	if req.ContentType == "" {
		req.ContentType = "audio/webm"
	}

	res, err := h.service.UploadIntent(ctx, actor.UserID, req.ContentType)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, res)
}

func (h *Handler) deleteRecording(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.DeleteAttemptRecording(ctx, attemptID, actor.UserID); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getFeedback(w http.ResponseWriter, r *http.Request) {
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

	fb, err := h.service.GetSpeakingFeedback(ctx, attemptID, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, fb)
}

func (h *Handler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	pageSize := 10
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if val, err := strconv.Atoi(ps); err == nil && val > 0 {
			pageSize = val
		}
	}

	list, err := h.service.ListSpeakingSubmissions(ctx, actor.UserID, page, pageSize)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, list)
}

// actorOrUnauthorized writes the 401 and reports whether the caller may proceed.
func actorOrUnauthorized(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	actor, ok := httpx.ActorFrom(r.Context())
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return uuid.Nil, false
	}
	return actor.UserID, true
}

func (h *Handler) getConsent(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorOrUnauthorized(w, r)
	if !ok {
		return
	}
	consent, err := h.service.GetConsent(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, consent)
}

func (h *Handler) recordConsent(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorOrUnauthorized(w, r)
	if !ok {
		return
	}
	consent, err := h.service.RecordConsent(r.Context(), userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, consent)
}
