// Package http provides HTTP endpoints for question bank administration and statistics.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// QuestionbankService defines operations required by questionbank HTTP handlers.
type QuestionbankService interface {
	ListQuestions(ctx context.Context, filter contract.Filter) ([]*contract.Question, int, error)
	GetQuestion(ctx context.Context, id uuid.UUID) (*contract.Question, error)
	GetQuestionStats(ctx context.Context, id uuid.UUID) (*contract.QuestionStats, error)
	GenerateQuestions(ctx context.Context, req contract.GenerateRequest) ([]*contract.Question, error)
	PublishQuestion(ctx context.Context, id uuid.UUID) (*contract.Question, error)
	RetireQuestion(ctx context.Context, id uuid.UUID) (*contract.Question, error)
}

// Handler handles question bank HTTP endpoints.
type Handler struct {
	service QuestionbankService
}

// NewHandler constructs a new question bank Handler.
func NewHandler(service QuestionbankService) *Handler {
	return &Handler{service: service}
}

// AdminRoutes registers the routes only an admin reaches.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Post("/admin/questions/generate", h.generateQuestions)
}

// ReviewRoutes registers the read routes a moderator also reaches: the service
// checks questionbank.read, which WO 19 F.5 grants to moderator.
func (h *Handler) ReviewRoutes(r chi.Router) {
	r.Get("/admin/questions", h.listQuestions)
	r.Get("/admin/questions/{id}/stats", h.getQuestionStats)
}

func (h *Handler) listQuestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	var filter contract.Filter
	if v := q.Get("exam_version"); v != "" {
		filter.ExamVersion = &v
	}
	if v := q.Get("exam_part_id"); v != "" {
		if u, err := uuid.Parse(v); err == nil {
			filter.ExamPartID = &u
		}
	}
	if v := q.Get("kind"); v != "" {
		filter.Kind = &v
	}
	if v := q.Get("cefr_level"); v != "" {
		filter.CEFRLevel = &v
	}
	if v := q.Get("node_code"); v != "" {
		filter.NodeCode = &v
	}
	if v := q.Get("status"); v != "" {
		filter.Status = &v
	}

	limit := 20
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	filter.Limit = limit
	filter.Offset = offset

	items, total, err := h.service.ListQuestions(ctx, filter)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type generateRequestPayload struct {
	ExamVersion *string    `json:"exam_version,omitempty"`
	ExamPartID  *uuid.UUID `json:"exam_part_id,omitempty"`
	Kind        string     `json:"kind"`
	Skill       string     `json:"skill"`
	CEFRLevel   string     `json:"cefr_level"`
	NodeCodes   []string   `json:"node_codes,omitempty"`
	Count       int        `json:"count"`
}

func (h *Handler) generateQuestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req generateRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_REQUEST_BODY", "Malformed request body."))
		return
	}

	if req.Kind == "" {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "KIND_REQUIRED", "Kind is required."))
		return
	}
	if req.CEFRLevel == "" {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "CEFR_REQUIRED", "CEFR level is required."))
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}

	created, err := h.service.GenerateQuestions(ctx, contract.GenerateRequest{
		ExamVersion: req.ExamVersion,
		ExamPartID:  req.ExamPartID,
		Kind:        req.Kind,
		Skill:       req.Skill,
		CEFRLevel:   req.CEFRLevel,
		NodeCodes:   req.NodeCodes,
		Count:       req.Count,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, map[string]any{
		"questions": created,
	})
}

func (h *Handler) getQuestionStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.BadRequest, "INVALID_ID", "Invalid question UUID."))
		return
	}

	stats, err := h.service.GetQuestionStats(ctx, id)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, stats)
}
