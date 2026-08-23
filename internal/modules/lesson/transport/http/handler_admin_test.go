package http_test

import (
	"bytes"
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
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type adminLessonService struct {
	fakeLessonService
	createdCourse *contract.Course
	updatedActs   []contract.Activity
}

func (a *adminLessonService) CreateCourse(
	_ context.Context, _ uuid.UUID, input service.CreateCourseInput,
) (*contract.Course, error) {
	if a.err != nil {
		return nil, a.err
	}
	c := &contract.Course{
		ID:             uuid.New(),
		Slug:           input.Slug,
		Title:          input.Title,
		Description:    input.Description,
		CEFRFrom:       input.CEFRFrom,
		CEFRTo:         input.CEFRTo,
		Status:         statusDraft,
		EstimatedHours: input.EstimatedHours,
	}
	a.createdCourse = c
	return c, nil
}

func (a *adminLessonService) UpdateActivities(
	_ context.Context, _, lessonID uuid.UUID, activities []domain.ActivityInput,
) ([]contract.Activity, error) {
	if a.err != nil {
		return nil, a.err
	}
	acts := make([]contract.Activity, len(activities))
	for i, act := range activities {
		acts[i] = contract.Activity{
			ID:               uuid.New(),
			LessonID:         lessonID,
			Position:         act.Position,
			Kind:             act.Kind,
			ContentVersionID: act.ContentVersionID,
			Config:           act.Config,
			Weight:           act.Weight,
		}
	}
	a.updatedActs = acts
	return acts, nil
}

func withActor(req *http.Request, userID uuid.UUID, role string) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   role,
	}
	return req.WithContext(httpx.WithActor(req.Context(), actor))
}

func TestHandler_AdminCreateCourse(t *testing.T) {
	svc := &adminLessonService{}
	handler, err := lessonhttp.NewHandler(svc, allowGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.AdminRoutes(r)

	adminID := uuid.New()
	reqBody := lessonhttp.CreateCourseRequest{
		Slug:           "ielts-mastery",
		Title:          "IELTS Mastery 7.5+",
		Description:    "Advanced preparation",
		CEFRFrom:       "B2",
		CEFRTo:         "C1",
		EstimatedHours: 50,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	t.Run("unauthenticated rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/courses", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("authorized succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/courses", bytes.NewReader(bodyBytes))
		req = withActor(req, adminID, "admin")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
		}

		var resp lessonhttp.CourseSummaryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Slug != "ielts-mastery" || resp.Status != statusDraft {
			t.Errorf("unexpected created course: %+v", resp)
		}
	})
}

func TestHandler_AdminUpdateLessonActivities(t *testing.T) {
	svc := &adminLessonService{}
	handler, err := lessonhttp.NewHandler(svc, allowGuard{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	r := chi.NewRouter()
	handler.AdminRoutes(r)

	adminID := uuid.New()
	lessonID := uuid.New()
	vID := uuid.New()

	reqBody := lessonhttp.UpdateActivitiesRequest{
		Activities: []lessonhttp.ActivityInput{
			{Position: 1, Kind: "quiz", ContentVersionID: vID, Weight: 2},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := "/admin/lessons/" + lessonID.String() + "/activities"
	req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(bodyBytes))
	req = withActor(req, adminID, "admin")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp lessonhttp.UpdateActivitiesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Activities) != 1 || resp.Activities[0].LessonID != lessonID {
		t.Errorf("unexpected activities in response: %+v", resp)
	}
}
