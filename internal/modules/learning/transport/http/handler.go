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
	Enroll(ctx context.Context, userID, courseID uuid.UUID) (*domain.Enrollment, error)
	StartSession(ctx context.Context, userID uuid.UUID, metadata json.RawMessage) (*domain.LearningSession, error)
	CompleteSession(
		ctx context.Context, userID, sessionID uuid.UUID, activitiesCompleted *int,
	) (*domain.LearningSession, error)
	Dashboard(ctx context.Context, userID uuid.UUID) (*domain.DashboardData, error)
	Progress(ctx context.Context, userID uuid.UUID) (*domain.ProgressData, error)
	GradePreview(
		ctx context.Context, activityID uuid.UUID, response json.RawMessage,
	) (*service.PreviewGradeResultDTO, error)
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
	router.Get("/me/dashboard", h.getDashboard)
	router.Get("/me/progress", h.getProgress)
	router.Post("/courses/{id}/enroll", h.enroll)
	router.Post("/activities/{id}/attempts", h.startAttempt)
	router.Post("/activities/{id}/grade", h.gradePreview)
	router.Post("/attempts/{id}/submit", h.submitAttempt)
	router.Get("/attempts/{id}", h.getAttempt)
	router.Post("/me/sessions", h.startSession)
	router.Post("/me/sessions/{id}/complete", h.completeSession)
}

func (h *Handler) enroll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	courseIDStr := chi.URLParam(r, "id")
	courseID, err := uuid.Parse(courseIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid course ID format"))
		return
	}

	enrollment, err := h.service.Enroll(ctx, actor.UserID, courseID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, toEnrollmentResponse(enrollment))
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	var req StartSessionRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY", "Failed to parse request body"))
			return
		}
	}

	sess, err := h.service.StartSession(ctx, actor.UserID, req.Metadata)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, toLearningSessionResponse(sess))
}

func (h *Handler) completeSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	sessionIDStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid session ID format"))
		return
	}

	var req CompleteSessionRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY", "Failed to parse request body"))
			return
		}
	}

	sess, err := h.service.CompleteSession(ctx, actor.UserID, sessionID, req.ActivitiesCompleted)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toLearningSessionResponse(sess))
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

// gradePreview grades a response for a caller with no account, and records
// nothing.
//
// No actor is required and none is read. That is the whole endpoint: a visitor
// who has not signed up can answer a lesson's activities and be told whether
// they were right, and nothing about it is kept. See ADR-0025.
//
// Deliberately not the attempt flow with the writes switched off. A signed-in
// learner still goes through POST /activities/{id}/attempts and
// POST /attempts/{id}/submit, which is what produces their progress and their
// review cards. Making one handler do both, keyed on whether a token happened
// to be present, is how a learner's work silently stops being saved.
//
// No Idempotency-Key, because there is nothing to make idempotent: replaying
// this changes no state anywhere.
func (h *Handler) gradePreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	activityID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid activity ID format"))
		return
	}

	var req SubmitAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_REQUEST_BODY", "Failed to parse request body"))
		return
	}

	dto, err := h.service.GradePreview(ctx, activityID, req.Response)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toPreviewGradeResponse(dto))
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

func (h *Handler) getDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	data, err := h.service.Dashboard(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toDashboardResponse(data))
}

func (h *Handler) getProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	data, err := h.service.Progress(ctx, actor.UserID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toProgressResponse(data))
}
