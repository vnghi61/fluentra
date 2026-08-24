// Package http provides HTTP endpoints for the learning attempt lifecycle.
package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Guard defines authorization checks for learning handlers.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// LearningService defines the use cases exposed by HTTP handlers.
type LearningService interface {
	StartAttempt(ctx context.Context, userID, activityID uuid.UUID) (*service.StartAttemptDTO, error)
	SubmitAttempt(
		ctx context.Context, userID, attemptID, idempotencyKey uuid.UUID, response json.RawMessage,
	) (*service.SubmitAttemptResultDTO, error)
	GetAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*service.AttemptDetailDTO, error)
}

// Handler serves HTTP endpoints for attempts.
type Handler struct {
	service LearningService
	guard   Guard
}

// NewHandler constructs a Handler.
func NewHandler(service LearningService, guard Guard) (*Handler, error) {
	if guard == nil {
		return nil, apperr.New(apperr.Internal, "GUARD_REQUIRED", "authorization guard is required for learning handlers")
	}
	return &Handler{
		service: service,
		guard:   guard,
	}, nil
}

// Routes mounts the attempt lifecycle endpoints on the router.
func (h *Handler) Routes(router chi.Router) {
	router.Post("/activities/{id}/attempts", h.startAttempt)
	router.Post("/attempts/{id}/submit", h.submitAttempt)
	router.Get("/attempts/{id}", h.getAttempt)
}

func (h *Handler) startAttempt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	activityIDStr := chi.URLParam(r, "id")
	activityID, err := uuid.Parse(activityIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid activity ID format"))
		return
	}

	dto, err := h.service.StartAttempt(ctx, actor.UserID, activityID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, toStartAttemptResponse(dto))
}

func (h *Handler) submitAttempt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid attempt ID format"))
		return
	}

	// Extract and validate Idempotency-Key header
	idempotencyKeyStr := r.Header.Get("Idempotency-Key")
	if idempotencyKeyStr == "" {
		httpx.WriteProblem(w, r, domain.ErrInvalidIdempotencyKey)
		return
	}

	idempotencyKey, err := uuid.Parse(idempotencyKeyStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(
			apperr.Validation, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must be a valid UUID",
		))
		return
	}

	var req SubmitAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY", "Failed to parse request body"))
		return
	}

	dto, err := h.service.SubmitAttempt(ctx, actor.UserID, attemptID, idempotencyKey, req.Response)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	statusCode := http.StatusOK
	if dto.Async {
		statusCode = http.StatusAccepted
	}

	httpx.WriteJSON(w, r, statusCode, toSubmitAttemptResponse(dto))
}

func (h *Handler) getAttempt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid attempt ID format"))
		return
	}

	dto, err := h.service.GetAttempt(ctx, actor.UserID, attemptID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toAttemptDetailResponse(dto))
}
