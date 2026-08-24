package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	learninghttp "github.com/fluentra/fluentra/internal/modules/learning/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	testStatusGraded = "graded"
	keyResponse      = "response"
)

type fakeGuard struct{}

func (f fakeGuard) Require(_ context.Context, _ string) error {
	return nil
}

type fakeLearningService struct {
	startDTO     *service.StartAttemptDTO
	startErr     error
	submitDTO    *service.SubmitAttemptResultDTO
	submitErr    error
	getDTO       *service.AttemptDetailDTO
	getErr       error
	enrollDTO    *domain.Enrollment
	enrollErr    error
	startSessDTO *domain.LearningSession
	startSessErr error
	compSessDTO  *domain.LearningSession
	compSessErr  error
	dashDTO      *domain.DashboardData
	dashErr      error
	progDTO      *domain.ProgressData
	progErr      error
}

func (f *fakeLearningService) StartAttempt(_ context.Context, _, _ uuid.UUID) (*service.StartAttemptDTO, error) {
	return f.startDTO, f.startErr
}

func (f *fakeLearningService) SubmitAttempt(
	_ context.Context, _, _, _ uuid.UUID, _ json.RawMessage,
) (*service.SubmitAttemptResultDTO, error) {
	return f.submitDTO, f.submitErr
}

func (f *fakeLearningService) GetAttempt(_ context.Context, _, _ uuid.UUID) (*service.AttemptDetailDTO, error) {
	return f.getDTO, f.getErr
}

func (f *fakeLearningService) Enroll(_ context.Context, _, _ uuid.UUID) (*domain.Enrollment, error) {
	return f.enrollDTO, f.enrollErr
}

func (f *fakeLearningService) StartSession(
	_ context.Context, _ uuid.UUID, _ json.RawMessage,
) (*domain.LearningSession, error) {
	return f.startSessDTO, f.startSessErr
}

func (f *fakeLearningService) CompleteSession(
	_ context.Context, _, _ uuid.UUID, _ *int,
) (*domain.LearningSession, error) {
	return f.compSessDTO, f.compSessErr
}

func (f *fakeLearningService) Dashboard(_ context.Context, _ uuid.UUID) (*domain.DashboardData, error) {
	return f.dashDTO, f.dashErr
}

func (f *fakeLearningService) Progress(_ context.Context, _ uuid.UUID) (*domain.ProgressData, error) {
	return f.progDTO, f.progErr
}

const (
	testStateInProgress = "in_progress"
	testStateCompleted  = "completed"
	testSkillGrammar    = "grammar"
)

func withActor(r *http.Request, userID uuid.UUID) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   "user",
	}
	return r.WithContext(httpx.WithActor(r.Context(), actor))
}

func setupTestRouter(svc *fakeLearningService) (chi.Router, error) {
	handler, err := learninghttp.NewHandler(svc, fakeGuard{})
	if err != nil {
		return nil, err
	}
	router := chi.NewRouter()
	handler.Routes(router)
	return router, nil
}

func TestHandler_StartAttempt_Success(t *testing.T) {
	activityID := uuid.New()
	attemptID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeLearningService{
		startDTO: &service.StartAttemptDTO{
			AttemptID:  attemptID,
			ActivityID: activityID,
			Status:     testStateInProgress,
			StartedAt:  now,
		},
	}
	router, err := setupTestRouter(svc)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/activities/"+activityID.String()+"/attempts", nil)
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var resp learninghttp.StartAttemptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.AttemptID != attemptID {
		t.Errorf("got attempt ID %s, want %s", resp.AttemptID, attemptID)
	}
}

func TestHandler_StartAttempt_Unauthorized(t *testing.T) {
	svc := &fakeLearningService{}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/activities/"+uuid.New().String()+"/attempts", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestHandler_SubmitAttempt_Synchronous200(t *testing.T) {
	attemptID := uuid.New()
	score := 100
	maxScore := 100
	correct := true
	fb := "Good job"

	svc := &fakeLearningService{
		submitDTO: &service.SubmitAttemptResultDTO{
			AttemptID: attemptID,
			Status:    testStatusGraded,
			Score:     &score,
			MaxScore:  &maxScore,
			Correct:   &correct,
			Feedback:  &fb,
			Async:     false,
		},
	}
	router, _ := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]any{keyResponse: map[string]any{"selected": 1}})
	req := httptest.NewRequest(http.MethodPost, "/attempts/"+attemptID.String()+"/submit", bytes.NewReader(body))
	req = withActor(req, uuid.New())
	req.Header.Set("Idempotency-Key", uuid.New().String())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp learninghttp.SubmitAttemptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != testStatusGraded || *resp.Score != 100 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHandler_SubmitAttempt_Async202(t *testing.T) {
	attemptID := uuid.New()

	svc := &fakeLearningService{
		submitDTO: &service.SubmitAttemptResultDTO{
			AttemptID: attemptID,
			Status:    "grading",
			Async:     true,
		},
	}
	router, _ := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]any{keyResponse: map[string]any{"audio": "data"}})
	req := httptest.NewRequest(http.MethodPost, "/attempts/"+attemptID.String()+"/submit", bytes.NewReader(body))
	req = withActor(req, uuid.New())
	req.Header.Set("Idempotency-Key", uuid.New().String())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", rec.Code)
	}

	var resp learninghttp.SubmitAttemptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "grading" {
		t.Errorf("got status %s, want grading", resp.Status)
	}
}

func TestHandler_SubmitAttempt_MissingIdempotencyKey(t *testing.T) {
	svc := &fakeLearningService{}
	router, _ := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]any{keyResponse: map[string]any{}})
	req := httptest.NewRequest(http.MethodPost, "/attempts/"+uuid.New().String()+"/submit", bytes.NewReader(body))
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422 for missing Idempotency-Key, got %d", rec.Code)
	}
}

func TestHandler_SubmitAttempt_Conflict(t *testing.T) {
	svc := &fakeLearningService{
		submitErr: domain.ErrAlreadyGraded,
	}
	router, _ := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]any{keyResponse: map[string]any{}})
	req := httptest.NewRequest(http.MethodPost, "/attempts/"+uuid.New().String()+"/submit", bytes.NewReader(body))
	req = withActor(req, uuid.New())
	req.Header.Set("Idempotency-Key", uuid.New().String())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict, got %d", rec.Code)
	}
}

func TestHandler_GetAttempt_Success(t *testing.T) {
	attemptID := uuid.New()
	userID := uuid.New()
	activityID := uuid.New()
	now := time.Now().UTC()
	score := 90
	maxScore := 100

	svc := &fakeLearningService{
		getDTO: &service.AttemptDetailDTO{
			ID:          attemptID,
			ActivityID:  activityID,
			UserID:      userID,
			Status:      testStatusGraded,
			Score:       &score,
			MaxScore:    &maxScore,
			StartedAt:   now,
			CompletedAt: &now,
		},
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/attempts/"+attemptID.String(), nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp learninghttp.AttemptDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != attemptID || resp.Status != testStatusGraded || *resp.Score != 90 {
		t.Errorf("unexpected detail response: %+v", resp)
	}
}

func TestHandler_GetAttempt_NotFound(t *testing.T) {
	svc := &fakeLearningService{
		getErr: domain.ErrAttemptNotFound,
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/attempts/"+uuid.New().String(), nil)
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandler_NilGuardFailsClosed(t *testing.T) {
	_, err := learninghttp.NewHandler(&fakeLearningService{}, nil)
	if err == nil {
		t.Fatal("expected error with nil guard")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "GUARD_REQUIRED" {
		t.Fatalf("expected GUARD_REQUIRED, got: %v", err)
	}
}

func TestHandler_InvalidUUIDsAndBadBodies(t *testing.T) {
	svc := &fakeLearningService{}
	router, _ := setupTestRouter(svc)

	// 1. Invalid activity UUID on start
	req := httptest.NewRequest(http.MethodPost, "/activities/not-a-uuid/attempts", nil)
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422, got %d", rec.Code)
	}

	// 2. Invalid attempt UUID on submit
	req = httptest.NewRequest(http.MethodPost, "/attempts/not-a-uuid/submit", bytes.NewReader([]byte(`{}`)))
	req = withActor(req, uuid.New())
	req.Header.Set("Idempotency-Key", uuid.New().String())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422, got %d", rec.Code)
	}

	// 3. Invalid Idempotency-Key UUID on submit
	req = httptest.NewRequest(
		http.MethodPost, "/attempts/"+uuid.New().String()+"/submit", bytes.NewReader([]byte(`{}`)),
	)
	req = withActor(req, uuid.New())
	req.Header.Set("Idempotency-Key", "not-a-valid-uuid")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422, got %d", rec.Code)
	}

	// 4. Invalid JSON body on submit
	req = httptest.NewRequest(
		http.MethodPost, "/attempts/"+uuid.New().String()+"/submit", bytes.NewReader([]byte(`{bad json`)),
	)
	req = withActor(req, uuid.New())
	req.Header.Set("Idempotency-Key", uuid.New().String())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422, got %d", rec.Code)
	}

	// 5. Submit attempt unauthorized
	req = httptest.NewRequest(
		http.MethodPost, "/attempts/"+uuid.New().String()+"/submit", bytes.NewReader([]byte(`{}`)),
	)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	// 6. Get attempt invalid UUID
	req = httptest.NewRequest(http.MethodGet, "/attempts/not-a-uuid", nil)
	req = withActor(req, uuid.New())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422, got %d", rec.Code)
	}

	// 7. Get attempt unauthorized
	req = httptest.NewRequest(http.MethodGet, "/attempts/"+uuid.New().String(), nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandler_Enroll(t *testing.T) {
	courseID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeLearningService{
		enrollDTO: &domain.Enrollment{
			ID:        uuid.New(),
			UserID:    userID,
			CourseID:  courseID,
			Status:    domain.StatusEnrollmentActive,
			StartedAt: now,
		},
	}
	router, _ := setupTestRouter(svc)

	// 1. Success 201
	req := httptest.NewRequest(http.MethodPost, "/courses/"+courseID.String()+"/enroll", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var resp learninghttp.EnrollmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.CourseID != courseID || resp.Status != domain.StatusEnrollmentActive {
		t.Errorf("unexpected enrollment resp: %+v", resp)
	}

	// 2. Unauthorized 401
	req = httptest.NewRequest(http.MethodPost, "/courses/"+courseID.String()+"/enroll", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	// 3. Invalid UUID 400
	req = httptest.NewRequest(http.MethodPost, "/courses/invalid-id/enroll", nil)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400 or 422, got %d", rec.Code)
	}

	// 4. Duplicate conflict 409
	svcConflict := &fakeLearningService{
		enrollErr: domain.ErrAlreadyEnrolled,
	}
	routerConflict, _ := setupTestRouter(svcConflict)
	req = httptest.NewRequest(http.MethodPost, "/courses/"+courseID.String()+"/enroll", nil)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	routerConflict.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict, got %d", rec.Code)
	}
}

func TestHandler_Sessions(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeLearningService{
		startSessDTO: &domain.LearningSession{
			ID:                  sessID,
			UserID:              userID,
			StartedAt:           now,
			ActivitiesCompleted: 0,
			Minutes:             0,
		},
		compSessDTO: &domain.LearningSession{
			ID:                  sessID,
			UserID:              userID,
			StartedAt:           now,
			EndedAt:             &now,
			ActivitiesCompleted: 3,
			Minutes:             15,
		},
	}
	router, _ := setupTestRouter(svc)

	// 1. POST /me/sessions 201
	req := httptest.NewRequest(http.MethodPost, "/me/sessions", bytes.NewReader([]byte(`{"metadata":{"app":"web"}}`)))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var startResp learninghttp.LearningSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &startResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if startResp.ID != sessID {
		t.Errorf("got session ID %s, want %s", startResp.ID, sessID)
	}

	// 2. POST /me/sessions/{id}/complete 200
	req = httptest.NewRequest(
		http.MethodPost, "/me/sessions/"+sessID.String()+"/complete",
		bytes.NewReader([]byte(`{"activities_completed":3}`)),
	)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var compResp learninghttp.LearningSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &compResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if compResp.Minutes != 15 || compResp.ActivitiesCompleted != 3 {
		t.Errorf("unexpected complete resp: %+v", compResp)
	}

	// 3. Complete unowned / not found 404
	svc404 := &fakeLearningService{
		compSessErr: domain.ErrSessionNotFound,
	}
	router404, _ := setupTestRouter(svc404)
	req = httptest.NewRequest(http.MethodPost, "/me/sessions/"+sessID.String()+"/complete", nil)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router404.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandler_SessionBadRequests(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()

	// A body that is not JSON is a 4xx from the handler, before the service runs.
	svc := &fakeLearningService{}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/me/sessions", bytes.NewReader([]byte(`{`)))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("malformed start body: got %d, want a 4xx", rec.Code)
	}

	req = httptest.NewRequest(
		http.MethodPost, "/me/sessions/"+sessID.String()+"/complete", bytes.NewReader([]byte(`{`)),
	)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("malformed complete body: got %d, want a 4xx", rec.Code)
	}

	// An id that is not a UUID never reaches the service either.
	req = httptest.NewRequest(http.MethodPost, "/me/sessions/not-a-uuid/complete", nil)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Errorf("malformed session id: got %d, want a 4xx", rec.Code)
	}

	// A negative activity count is the service's validation error, rendered as 422.
	svcInvalid := &fakeLearningService{compSessErr: domain.ErrInvalidActivityCount}
	routerInvalid, _ := setupTestRouter(svcInvalid)
	req = httptest.NewRequest(
		http.MethodPost, "/me/sessions/"+sessID.String()+"/complete",
		bytes.NewReader([]byte(`{"activities_completed":-1}`)),
	)
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	routerInvalid.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Errorf("negative activity count: got %d, want 422 or 400", rec.Code)
	}

	// Starting a session is 401 without an actor.
	req = httptest.NewRequest(http.MethodPost, "/me/sessions", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous start: got %d, want 401", rec.Code)
	}
}

func TestHandler_GetDashboard_NotStarted(t *testing.T) {
	userID := uuid.New()
	svc := &fakeLearningService{
		dashDTO: &domain.DashboardData{
			State:           domain.DashboardStateNotStarted,
			NextActivity:    nil,
			DueReviewsCount: 0,
			SkillMastery:    []domain.SkillMastery{},
		},
	}
	router, err := setupTestRouter(svc)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/me/dashboard", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	if raw["state"] != "not_started" {
		t.Errorf("state = %v, want not_started", raw["state"])
	}
	if _, ok := raw["next_activity"]; ok {
		t.Errorf("next_activity must be omitted in not_started state: body=%s", body)
	}
	if due, ok := raw["due_reviews_count"].(float64); !ok || due != 0 {
		t.Errorf("due_reviews_count = %v, want 0", raw["due_reviews_count"])
	}
	sm, ok := raw["skill_mastery"].([]any)
	if !ok || sm == nil {
		t.Errorf("skill_mastery must be empty array [], not null: body=%s", body)
	}
}

func TestHandler_GetDashboard_InProgress(t *testing.T) {
	userID := uuid.New()
	actID := uuid.New()
	lessonID := uuid.New()
	unitID := uuid.New()
	courseID := uuid.New()
	estMin := 15

	svc := &fakeLearningService{
		dashDTO: &domain.DashboardData{
			State: domain.DashboardStateInProgress,
			NextActivity: &domain.NextActivity{
				ActivityID:       actID,
				LessonID:         lessonID,
				UnitID:           unitID,
				CourseID:         courseID,
				Title:            "Present Tense Quiz",
				Kind:             "quiz",
				Skill:            testSkillGrammar,
				EstimatedMinutes: &estMin,
			},
			DueReviewsCount: 0,
			SkillMastery: []domain.SkillMastery{
				{
					Skill:      testSkillGrammar,
					Level:      "B1",
					Confidence: 0.65,
					UpdatedAt:  time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/dashboard", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp learninghttp.DashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	if resp.State != testStateInProgress {
		t.Errorf("state = %s, want in_progress", resp.State)
	}
	if resp.NextActivity == nil {
		t.Fatalf("expected next_activity to be populated")
	}
	if resp.NextActivity.ActivityID != actID || resp.NextActivity.Title != "Present Tense Quiz" {
		t.Errorf("unexpected next_activity: %+v", resp.NextActivity)
	}
	if len(resp.SkillMastery) != 1 || resp.SkillMastery[0].Skill != testSkillGrammar {
		t.Errorf("unexpected skill_mastery: %+v", resp.SkillMastery)
	}
}

func TestHandler_GetDashboard_Completed(t *testing.T) {
	userID := uuid.New()
	svc := &fakeLearningService{
		dashDTO: &domain.DashboardData{
			State:           domain.DashboardStateCompleted,
			NextActivity:    nil,
			DueReviewsCount: 0,
			SkillMastery:    []domain.SkillMastery{},
		},
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/dashboard", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if raw["state"] != testStateCompleted {
		t.Errorf("state = %v, want completed", raw["state"])
	}
	if _, ok := raw["next_activity"]; ok {
		t.Errorf("next_activity must be omitted in completed state")
	}
}

func TestHandler_GetDashboard_Unauthorized(t *testing.T) {
	svc := &fakeLearningService{}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/dashboard", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_GetDashboard_ServiceError(t *testing.T) {
	userID := uuid.New()
	svc := &fakeLearningService{
		dashErr: errors.New("db error"),
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/dashboard", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandler_GetProgress_Success(t *testing.T) {
	userID := uuid.New()
	courseID1 := uuid.New()
	courseID2 := uuid.New()
	completedAt := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
	score := 95

	svc := &fakeLearningService{
		progDTO: &domain.ProgressData{
			Courses: []domain.CourseProgressData{
				{
					CourseID:            courseID1,
					Status:              testStateInProgress,
					CompletedActivities: 5,
					TotalActivities:     10,
					Percentage:          50,
					Score:               nil,
					CompletedAt:         nil,
				},
				{
					CourseID:            courseID2,
					Status:              testStateCompleted,
					CompletedActivities: 20,
					TotalActivities:     20,
					Percentage:          100,
					Score:               &score,
					CompletedAt:         &completedAt,
				},
			},
			Skills: []domain.SkillMastery{
				{
					Skill:      "vocabulary",
					Level:      "A2",
					Confidence: 0.50,
					UpdatedAt:  time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/progress", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp learninghttp.ProgressResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	if len(resp.Courses) != 2 {
		t.Fatalf("got %d courses, want 2", len(resp.Courses))
	}
	if resp.Courses[0].Percentage != 50 || resp.Courses[0].Status != testStateInProgress {
		t.Errorf("course 0 mismatch: %+v", resp.Courses[0])
	}
	if resp.Courses[1].Percentage != 100 || resp.Courses[1].Status != testStateCompleted {
		t.Errorf("course 1 mismatch: %+v", resp.Courses[1])
	}
	if resp.Courses[1].Score == nil || *resp.Courses[1].Score != 95 {
		t.Errorf("course 1 score mismatch: %+v", resp.Courses[1].Score)
	}
	if len(resp.Skills) != 1 || resp.Skills[0].Skill != "vocabulary" {
		t.Errorf("skills mismatch: %+v", resp.Skills)
	}
}

func TestHandler_GetProgress_Empty(t *testing.T) {
	userID := uuid.New()
	svc := &fakeLearningService{
		progDTO: &domain.ProgressData{
			Courses: []domain.CourseProgressData{},
			Skills:  []domain.SkillMastery{},
		},
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/progress", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	courses, ok := raw["courses"].([]any)
	if !ok || courses == nil {
		t.Errorf("courses must be empty array [], not null: body=%s", body)
	}
	skills, ok := raw["skills"].([]any)
	if !ok || skills == nil {
		t.Errorf("skills must be empty array [], not null: body=%s", body)
	}
}

func TestHandler_GetProgress_Unauthorized(t *testing.T) {
	svc := &fakeLearningService{}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/progress", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_GetProgress_ServiceError(t *testing.T) {
	userID := uuid.New()
	svc := &fakeLearningService{
		progErr: errors.New("database failure"),
	}
	router, _ := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/progress", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
