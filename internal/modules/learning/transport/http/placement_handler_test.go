package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/service"
)

func placementRequest(
	t *testing.T, svc *fakeLearningService, method, path, body string, header http.Header,
) *httptest.ResponseRecorder {
	t.Helper()
	router, err := setupTestRouter(svc)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	for key, values := range header {
		req.Header[key] = values
	}
	req = withActor(req, uuid.New())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPlacementRoutes_ReadAndStart(t *testing.T) {
	sessionID := uuid.New()
	svc := &fakeLearningService{
		overviewDTO:  &service.PlacementOverviewDTO{InviteAvailable: true},
		placementDTO: &service.PlacementSessionDTO{ID: sessionID, Status: "in_progress"},
		pathDTO:      &service.StartingPathDTO{Level: "B1"},
		planDTO:      &service.WeeklyPlanDTO{MinutesGoal: 90},
	}

	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/me/placement", http.StatusOK},
		{http.MethodPost, "/me/placement", http.StatusCreated},
		{http.MethodGet, "/me/placement/sessions/" + sessionID.String(), http.StatusOK},
		{http.MethodGet, "/me/path", http.StatusOK},
		{http.MethodGet, "/me/weekly-plan", http.StatusOK},
	}
	for _, tc := range cases {
		rec := placementRequest(t, svc, tc.method, tc.path, "", nil)
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d: %s", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestPlacementRoutes_AnAnswerNeedsAnIdempotencyKey(t *testing.T) {
	svc := &fakeLearningService{placementDTO: &service.PlacementSessionDTO{}}
	path := "/me/placement/sessions/" + uuid.NewString() + "/answers"
	body := `{"activity_id": "` + uuid.NewString() + `", "response": {"selected_option_id": "A"}}`

	rec := placementRequest(t, svc, http.MethodPost, path, body, nil)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("an answer without a key = %d, want a 4xx", rec.Code)
	}
	if svc.seenActivity != uuid.Nil {
		t.Fatal("the service was called without a key")
	}

	key := uuid.New()
	rec = placementRequest(t, svc, http.MethodPost, path, body, http.Header{"Idempotency-Key": {key.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("an answer with a key = %d: %s", rec.Code, rec.Body.String())
	}
	if svc.seenKey != key || svc.seenActivity == uuid.Nil || len(svc.seenResponse) == 0 {
		t.Fatalf("the handler did not pass the key, activity and response through")
	}
}

func TestPlacementRoutes_AnAnswerNeedsAnActivityAndAResponse(t *testing.T) {
	svc := &fakeLearningService{placementDTO: &service.PlacementSessionDTO{}}
	path := "/me/placement/sessions/" + uuid.NewString() + "/answers"
	rec := placementRequest(t, svc, http.MethodPost, path, `{"response": {}}`,
		http.Header{"Idempotency-Key": {uuid.NewString()}})
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("an answer without activity_id = %d, want a 4xx", rec.Code)
	}
}

func TestPlacementRoutes_ProductivePassesTheSkip(t *testing.T) {
	svc := &fakeLearningService{placementDTO: &service.PlacementSessionDTO{}}
	path := "/me/placement/sessions/" + uuid.NewString() + "/productive"
	rec := placementRequest(t, svc, http.MethodPost, path, `{"skip": true}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("productive = %d: %s", rec.Code, rec.Body.String())
	}
	if svc.seenSkip == nil || !*svc.seenSkip {
		t.Fatal("skip was not passed through")
	}
}

func TestPlacementRoutes_UnspecifiedAliasesAreNotMounted(t *testing.T) {
	svc := &fakeLearningService{overviewDTO: &service.PlacementOverviewDTO{}}
	for _, path := range []string{"/placement/invitation", "/placement/sessions"} {
		rec := placementRequest(t, svc, http.MethodGet, path, "", nil)
		if rec.Code == http.StatusOK {
			t.Errorf("GET %s is served, but it is in no spec", path)
		}
	}
}
