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
	startDTO  *service.StartAttemptDTO
	startErr  error
	submitDTO *service.SubmitAttemptResultDTO
	submitErr error
	getDTO    *service.AttemptDetailDTO
	getErr    error
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
			Status:     "in_progress",
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
