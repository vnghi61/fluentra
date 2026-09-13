package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/exam/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// ExamService defines operations needed by HTTP handlers.
type ExamService interface {
	ListExams(ctx context.Context) ([]service.ExamDTO, error)
	StartSitting(ctx context.Context, userID, examID uuid.UUID, req service.StartAttemptRequest) (*service.ExamAttemptDTO, error)
	GetExamAttempt(ctx context.Context, userID, attemptID uuid.UUID) (*service.ExamAttemptDTO, error)
	AutosaveAnswers(ctx context.Context, userID, attemptID uuid.UUID, req service.SaveAnswersRequest) (*service.SaveAnswersResult, error)
	CompleteSection(ctx context.Context, userID, attemptID uuid.UUID, sectionNum int) (*service.CompleteSectionResult, error)
	SubmitExam(ctx context.Context, userID, attemptID uuid.UUID, submittedBy string) (*service.SubmitExamResult, error)
	GetScoreReport(ctx context.Context, userID, attemptID uuid.UUID) (*service.ScoreReportDTO, error)
	ListUserAttempts(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]service.ExamAttemptDTO, int64, error)
}

// Handler handles HTTP requests for exams.
type Handler struct {
	service ExamService
}

// NewHandler constructs an exam HTTP handler.
func NewHandler(service ExamService) *Handler {
	return &Handler{service: service}
}

// Routes mounts all exam endpoints on the router.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/exams", h.listExams)
	r.Post("/exams/{id}/attempts", h.startSitting)
	r.Get("/exam-attempts", h.listAttempts)
	r.Get("/exam-attempts/{id}", h.getAttempt)
	r.Put("/exam-attempts/{id}/answers", h.saveAnswers)
	r.Post("/exam-attempts/{id}/sections/{n}/complete", h.completeSection)
	r.Post("/exam-attempts/{id}/submit", h.submitExam)
	r.Get("/exam-attempts/{id}/report", h.getReport)
}

func (h *Handler) listExams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	exams, err := h.service.ListExams(ctx)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if exams == nil {
		exams = []service.ExamDTO{}
	}
	httpx.WriteJSON(w, r, http.StatusOK, exams)
}

func (h *Handler) startSitting(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	examIDStr := chi.URLParam(r, "id")
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_EXAM_ID", "Invalid exam ID"))
		return
	}

	var req service.StartAttemptRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	attempt, err := h.service.StartSitting(ctx, actor.UserID, examID, req)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, attempt)
}

func (h *Handler) listAttempts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	limit := int32(20)
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	offset := int32(0)
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	items, total, err := h.service.ListUserAttempts(ctx, actor.UserID, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}
	if items == nil {
		items = []service.ExamAttemptDTO{}
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
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
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "Invalid attempt ID"))
		return
	}

	attempt, err := h.service.GetExamAttempt(ctx, actor.UserID, attemptID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, attempt)
}

func (h *Handler) saveAnswers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "Invalid attempt ID"))
		return
	}

	var req service.SaveAnswersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_REQUEST_BODY", "failed to parse request body"))
		return
	}

	res, err := h.service.AutosaveAnswers(ctx, actor.UserID, attemptID, req)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, res)
}

func (h *Handler) completeSection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "Invalid attempt ID"))
		return
	}

	nStr := chi.URLParam(r, "n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 || n > 4 {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_SECTION_NUMBER", "Invalid section number (must be 1-4)"))
		return
	}

	res, err := h.service.CompleteSection(ctx, actor.UserID, attemptID, n)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, res)
}

func (h *Handler) submitExam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "Invalid attempt ID"))
		return
	}

	res, err := h.service.SubmitExam(ctx, actor.UserID, attemptID, "learner")
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusAccepted, res)
}

func (h *Handler) getReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, ok := httpx.ActorFrom(ctx)
	if !ok || actor.UserID == uuid.Nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHORIZED", "Authentication required"))
		return
	}

	attemptIDStr := chi.URLParam(r, "id")
	attemptID, err := uuid.Parse(attemptIDStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ATTEMPT_ID", "Invalid attempt ID"))
		return
	}

	report, err := h.service.GetScoreReport(ctx, actor.UserID, attemptID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, report)
}
