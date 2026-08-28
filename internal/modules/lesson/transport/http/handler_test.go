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

	slugIELTSFoundation = "ielts-foundation"
	pathIELTSFoundation = "/courses/" + slugIELTSFoundation
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

// TestHandler_PublishedCurriculumIsPublic is the inverse of the test that used
// to be here.
//
// It asserted that GET /courses answered 403 behind a denying guard. ADR-0025
// makes published curriculum public — a visitor who has not signed up browses
// the catalogue, opens a lesson and works through it — so the assertion is now
// that these three reach the service with no actor in the context at all, and
// with a guard that denies everything.
//
// The guard is denyGuard on purpose. Passing an allowGuard would prove only
// that the handler works when permitted, which was never in doubt; denying
// everything is what proves the handler no longer asks.
func TestHandler_PublishedCurriculumIsPublic(t *testing.T) {
	svc := &fakeLessonService{
		courses: []service.CourseSummaryDTO{},
		courseDetail: &service.CourseDetailDTO{
			ID: uuid.New(), Slug: slugIELTSFoundation, Title: "IELTS Foundation",
			CEFRFrom: "B1", CEFRTo: "B2", Status: statusPublished,
			Units: []service.CourseUnitDTO{},
		},
		lessonDetail: &service.LessonDetailDTO{
			ID: uuid.New(), UnitID: uuid.New(), Position: 1,
			Title: "Morning Routines", SkillFocus: "vocabulary", Status: "published",
			Activities: []service.LessonActivityDTO{},
		},
	}

	handler, err := lessonhttp.NewHandler(svc, denyGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.Routes(r)

	for _, path := range []string{
		"/courses",
		pathIELTSFoundation,
		"/lessons/" + uuid.New().String(),
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s with no actor = %d, want %d — published content is public",
				path, rec.Code, http.StatusOK)
		}
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
			Slug:           slugIELTSFoundation,
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

	req := httptest.NewRequest(http.MethodGet, pathIELTSFoundation, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp lessonhttp.CourseDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != courseID || resp.Slug != slugIELTSFoundation {
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
