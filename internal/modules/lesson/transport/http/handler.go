package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Permissions required by lesson operations.
const (
	PermContentReadPublished = "content.read.published"
	PermContentCreate        = "content.create"
	PermContentEdit          = "content.edit"
)

// Guard is the authorization interface required by lesson handlers.
type Guard interface {
	Require(ctx context.Context, permission string) error
}

// LessonService defines the use cases called by HTTP handlers.
type LessonService interface {
	ListCourses(ctx context.Context, level *string, limit, offset int) ([]service.CourseSummaryDTO, int64, error)
	GetCourseDetail(ctx context.Context, slug string, userID uuid.UUID) (*service.CourseDetailDTO, error)
	GetLessonDetail(ctx context.Context, lessonID, userID uuid.UUID) (*service.LessonDetailDTO, error)
	CreateCourse(ctx context.Context, actorID uuid.UUID, input service.CreateCourseInput) (*contract.Course, error)
	UpdateActivities(
		ctx context.Context, actorID, lessonID uuid.UUID, activities []domain.ActivityInput,
	) ([]contract.Activity, error)
}

// Handler serves HTTP endpoints for the lesson module.
type Handler struct {
	service LessonService
	guard   Guard
}

// NewHandler constructs a new Handler. It fails closed if guard is nil.
func NewHandler(service LessonService, guard Guard) (*Handler, error) {
	if guard == nil {
		return nil, apperr.New(apperr.Internal, "GUARD_REQUIRED", "authorization guard is required for lesson handlers")
	}
	return &Handler{
		service: service,
		guard:   guard,
	}, nil
}

// Routes mounts learner-facing lesson endpoints under the authenticated router.
func (h *Handler) Routes(router chi.Router) {
	router.Get("/courses", h.listCourses)
	router.Get("/courses/{slug}", h.getCourseBySlug)
	router.Get("/lessons/{id}", h.getLessonByID)
}

// AdminRoutes mounts staff/authoring lesson endpoints under the admin router.
func (h *Handler) AdminRoutes(router chi.Router) {
	router.Post("/admin/courses", h.createCourse)
	router.Put("/admin/lessons/{id}/activities", h.updateActivities)
}

func (h *Handler) listCourses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	// The three parameters the spec declares, and no others. `level` is the
	// filter GET /courses documents; limit and offset are bounded in the spec
	// and clamped again in the domain, because a query string is not a promise.
	var level *string
	if raw := r.URL.Query().Get("level"); raw != "" {
		level = &raw
	}

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}

	courses, _, err := h.service.ListCourses(ctx, level, limit, offset)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	resp := make([]CourseSummaryResponse, len(courses))
	for i, c := range courses {
		resp[i] = toCourseSummaryResponse(c)
	}

	httpx.WriteJSON(w, r, http.StatusOK, CourseListResponse{
		Courses: resp,
	})
}

func (h *Handler) getCourseBySlug(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		httpx.WriteProblem(w, r, domain.ErrInvalidSlug)
		return
	}

	var userID uuid.UUID
	if actor, ok := httpx.ActorFrom(ctx); ok {
		userID = actor.UserID
	}

	detail, err := h.service.GetCourseDetail(ctx, slug, userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toCourseDetailResponse(detail))
}

func (h *Handler) getLessonByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentReadPublished); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	idStr := chi.URLParam(r, "id")
	lessonID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid lesson ID."))
		return
	}

	var userID uuid.UUID
	if actor, ok := httpx.ActorFrom(ctx); ok {
		userID = actor.UserID
	}

	detail, err := h.service.GetLessonDetail(ctx, lessonID, userID)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusOK, toLessonDetailResponse(detail))
}

func (h *Handler) createCourse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentCreate); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	var req CreateCourseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	created, err := h.service.CreateCourse(ctx, actor.UserID, service.CreateCourseInput{
		Slug:           req.Slug,
		Title:          req.Title,
		Description:    req.Description,
		CEFRFrom:       req.CEFRFrom,
		CEFRTo:         req.CEFRTo,
		EstimatedHours: req.EstimatedHours,
	})
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, CourseSummaryResponse{
		ID:             created.ID,
		Slug:           created.Slug,
		Title:          created.Title,
		Description:    created.Description,
		CEFRFrom:       created.CEFRFrom,
		CEFRTo:         created.CEFRTo,
		Status:         created.Status,
		EstimatedHours: created.EstimatedHours,
	})
}

func (h *Handler) updateActivities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.guard.Require(ctx, PermContentEdit); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		httpx.WriteProblem(w, r, apperr.New(apperr.Unauthenticated, "UNAUTHENTICATED", "Authentication required."))
		return
	}

	idStr := chi.URLParam(r, "id")
	lessonID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.WriteProblem(w, r, apperr.New(apperr.Validation, "INVALID_ID", "Invalid lesson ID."))
		return
	}

	var req UpdateActivitiesRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	inputs := make([]domain.ActivityInput, len(req.Activities))
	for i, a := range req.Activities {
		inputs[i] = domain.ActivityInput{
			Position:         a.Position,
			Kind:             a.Kind,
			ContentVersionID: a.ContentVersionID,
			Config:           a.Config,
			Weight:           a.Weight,
		}
	}

	updated, err := h.service.UpdateActivities(ctx, actor.UserID, lessonID, inputs)
	if err != nil {
		httpx.WriteProblem(w, r, err)
		return
	}

	acts := make([]LessonActivityResponse, len(updated))
	for i, a := range updated {
		acts[i] = LessonActivityResponse{
			ID:               a.ID,
			LessonID:         a.LessonID,
			Position:         a.Position,
			Kind:             a.Kind,
			ContentVersionID: a.ContentVersionID,
			Config:           a.Config,
			Weight:           a.Weight,
		}
	}

	httpx.WriteJSON(w, r, http.StatusOK, UpdateActivitiesResponse{
		Activities: acts,
	})
}
