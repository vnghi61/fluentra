package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/service"
)

func postGrade(t *testing.T, router http.Handler, activityID uuid.UUID, sign bool) *httptest.ResponseRecorder {
	t.Helper()
	body := bytes.NewReader([]byte(`{"response":{"selected_option_id":"opt_habit"}}`))
	req := httptest.NewRequest(http.MethodPost, "/activities/"+activityID.String()+"/grade", body)
	if sign {
		req = withActor(req, uuid.New())
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestGradePreview_NeedsNoAccount is the endpoint's reason to exist: a visitor
// who has not signed up answers an activity and is told whether they were right.
//
// The request carries no actor at all — not an empty one, none — because that is
// the state ADR-0025 opened this route for.
func TestGradePreview_NeedsNoAccount(t *testing.T) {
	t.Parallel()

	answer := "opt_habit"
	svc := &fakeLearningService{
		previewDTO: &service.PreviewGradeResultDTO{
			Correct: true, Score: 100, MaxScore: 100,
			Feedback: "Correct! Well done.", CorrectAnswer: &answer,
		},
	}
	router, err := setupTestRouter(svc)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}

	activityID := uuid.New()
	rec := postGrade(t, router, activityID, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d — this route must not require an account", rec.Code, http.StatusOK)
	}
	if svc.seenPreviewActivity != activityID {
		t.Errorf("service saw activity %s, want %s", svc.seenPreviewActivity, activityID)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["correct"] != true {
		t.Errorf("correct = %v, want true", got["correct"])
	}
	// The answer is revealed here because the lesson body no longer carries it.
	if got["correct_answer"] != answer {
		t.Errorf("correct_answer = %v, want %q", got["correct_answer"], answer)
	}
	// Stated, not implied. A client reading this cannot mistake a preview for a
	// recorded attempt.
	if got["saved"] != false {
		t.Errorf("saved = %v, want false", got["saved"])
	}
	// There is no attempt, so there must be no attempt id to mistake for one.
	if _, present := got["attempt_id"]; present {
		t.Errorf("response carries an attempt_id: %v", got)
	}
}

// A signed-in caller is not refused — the route simply does not care. What it
// must never do is quietly become the path a learner's real work goes down;
// that is what the attempt flow is for, and the two are separate handlers.
func TestGradePreview_IgnoresAnActorRatherThanRefusingOne(t *testing.T) {
	t.Parallel()

	svc := &fakeLearningService{
		previewDTO: &service.PreviewGradeResultDTO{Correct: false, Score: 0, MaxScore: 100},
	}
	router, err := setupTestRouter(svc)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}

	if rec := postGrade(t, router, uuid.New(), true); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// No Idempotency-Key. Submitting an attempt requires one because a replay would
// otherwise double-write; a preview writes nothing, so demanding the header
// would be ceremony that fails honest callers.
func TestGradePreview_DoesNotRequireAnIdempotencyKey(t *testing.T) {
	t.Parallel()

	svc := &fakeLearningService{
		previewDTO: &service.PreviewGradeResultDTO{Correct: true, Score: 100, MaxScore: 100},
	}
	router, err := setupTestRouter(svc)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}

	rec := postGrade(t, router, uuid.New(), false)
	if rec.Code == http.StatusBadRequest || rec.Code == http.StatusUnprocessableEntity {
		t.Fatalf("a request with no Idempotency-Key was refused with %d: %s", rec.Code, rec.Body)
	}
}

func TestGradePreview_RejectsAMalformedActivityID(t *testing.T) {
	t.Parallel()

	router, err := setupTestRouter(&fakeLearningService{})
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/activities/not-a-uuid/grade",
		bytes.NewReader([]byte(`{"response":{}}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want a 4xx for a malformed id", rec.Code)
	}
}
