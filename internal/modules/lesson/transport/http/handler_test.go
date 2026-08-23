package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/lesson/domain"
	"github.com/fluentra/fluentra/internal/modules/lesson/service"
	lessonhttp "github.com/fluentra/fluentra/internal/modules/lesson/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	statusDraft     = "draft"
	statusPublished = "published"
)

type allowGuard struct{}

func (allowGuard) Require(_ context.Context, _ string) error {
	return nil
}

type denyGuard struct{}

func (denyGuard) Require(_ context.Context, _ string) error {
	return apperr.New(apperr.Forbidden, "FORBIDDEN", "Permission denied.")
}

type fakeLessonService struct {
	courses      []service.CourseSummaryDTO
	courseDetail *service.CourseDetailDTO
	lessonDetail *service.LessonDetailDTO
	err          error

	// what listCourses parsed out of the query string
	seenLevel  *string
	seenLimit  int
	seenOffset int
}

func (f *fakeLessonService) ListCourses(
	_ context.Context, level *string, limit, offset int,
) ([]service.CourseSummaryDTO, int64, error) {
	f.seenLevel = level
	f.seenLimit = limit
	f.seenOffset = offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.courses, int64(len(f.courses)), nil
}

func (f *fakeLessonService) GetCourseDetail(
	_ context.Context, _ string, _ uuid.UUID,
) (*service.CourseDetailDTO, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.courseDetail, nil
}

func (f *fakeLessonService) GetLessonDetail(
	_ context.Context, _, _ uuid.UUID,
) (*service.LessonDetailDTO, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.lessonDetail, nil
}

func (f *fakeLessonService) CreateCourse(
	_ context.Context, _ uuid.UUID, input service.CreateCourseInput,
) (*contract.Course, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &contract.Course{
		ID: uuid.New(), Slug: input.Slug, Title: input.Title,
		CEFRFrom: input.CEFRFrom, CEFRTo: input.CEFRTo, Status: statusDraft,
		EstimatedHours: input.EstimatedHours,
	}, nil
}

func (f *fakeLessonService) UpdateActivities(
	_ context.Context, _, lessonID uuid.UUID, inputs []domain.ActivityInput,
) ([]contract.Activity, error) {
	if f.err != nil {
		return nil, f.err
	}
	acts := make([]contract.Activity, len(inputs))
	for i, in := range inputs {
		acts[i] = contract.Activity{
			ID: uuid.New(), LessonID: lessonID, Position: in.Position,
			Kind: in.Kind, ContentVersionID: in.ContentVersionID, Weight: in.Weight,
		}
	}
	return acts, nil
}

func (f *fakeLessonService) PublishLesson(
	_ context.Context, _, lessonID uuid.UUID,
) (*service.LessonDetailDTO, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &service.LessonDetailDTO{
		ID:               lessonID,
		UnitID:           uuid.New(),
		Position:         1,
		Title:            "Published Lesson",
		SkillFocus:       "vocabulary",
		EstimatedMinutes: 15,
		Status:           statusPublished,
		Activities:       []service.LessonActivityDTO{},
	}, nil
}

func TestHandler_FailClosedGuard(t *testing.T) {
	_, err := lessonhttp.NewHandler(&fakeLessonService{}, nil)
	if err == nil {
		t.Fatal("expected NewHandler to fail when guard is nil")
	}
}

func TestHandler_PermissionDenied(t *testing.T) {
	handler, err := lessonhttp.NewHandler(&fakeLessonService{}, denyGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/courses", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandler_ListCourses(t *testing.T) {
	courseID := uuid.New()
	svc := &fakeLessonService{
		courses: []service.CourseSummaryDTO{
			{
				ID:             courseID,
				Slug:           "ielts-core",
				Title:          "IELTS Core",
				CEFRFrom:       "B1",
				CEFRTo:         "B2",
				Status:         statusPublished,
				EstimatedHours: 40,
			},
		},
	}

	handler, err := lessonhttp.NewHandler(svc, allowGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/courses?page=1&limit=10", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp lessonhttp.CourseListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Courses) != 1 || resp.Courses[0].ID != courseID {
		t.Errorf("unexpected courses in response: %+v", resp)
	}
}

func TestHandler_GetCourseBySlug(t *testing.T) {
	courseID := uuid.New()
	svc := &fakeLessonService{
		courseDetail: &service.CourseDetailDTO{
			ID:             courseID,
			Slug:           "ielts-foundation",
			Title:          "IELTS Foundation",
			CEFRFrom:       "B1",
			CEFRTo:         "B2",
			Status:         "published",
			EstimatedHours: 30,
			Units:          []service.CourseUnitDTO{},
		},
	}

	handler, err := lessonhttp.NewHandler(svc, allowGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/courses/ielts-foundation", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp lessonhttp.CourseDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != courseID || resp.Slug != "ielts-foundation" {
		t.Errorf("unexpected course detail: %+v", resp)
	}
}

func TestHandler_GetLessonByID_Locked(t *testing.T) {
	lessonID := uuid.New()
	svc := &fakeLessonService{
		err: domain.ErrLessonLocked,
	}

	handler, err := lessonhttp.NewHandler(svc, allowGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/lessons/"+lessonID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (403 Forbidden)", rec.Code, http.StatusForbidden)
	}
}
