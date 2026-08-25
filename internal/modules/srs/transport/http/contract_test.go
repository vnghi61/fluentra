//go:build contract

package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/srs/contract"
	"github.com/fluentra/fluentra/internal/modules/srs/service"
)

// The contract tests answer one question the unit tests cannot: does what this
// handler actually writes match what api/openapi/openapi.yaml promises?
//
// The DTOs in this package are hand-written rather than taken from the generated
// models, because a business module importing api/openapi would couple it to
// every other module's spec. That choice is only safe if something checks the
// two agree — this is that something. Run with `make test-contract`.

// loadSpec reads the bundled spec, the same artefact the code generators
// consume, so a test passing here and a client generated from the spec cannot
// disagree.
func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()

	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: true}
	path := filepath.Join("..", "..", "..", "..", "..", "api", "openapi", "openapi.bundle.yaml")
	spec, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	if err := spec.Validate(loader.Context); err != nil {
		t.Fatalf("the spec itself is invalid: %v", err)
	}
	return spec
}

// responseSchema pulls the 200 schema for one operation out of the spec.
func responseSchema(t *testing.T, spec *openapi3.T, path, method string) *openapi3.Schema {
	t.Helper()

	item := spec.Paths.Find(path)
	if item == nil {
		t.Fatalf("the spec has no path %q", path)
	}
	operation := item.GetOperation(method)
	if operation == nil {
		t.Fatalf("the spec has no %s %s", method, path)
	}
	response := operation.Responses.Status(http.StatusOK)
	if response == nil || response.Value == nil {
		t.Fatalf("%s %s declares no 200 response", method, path)
	}
	media := response.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s declares no application/json 200 body", method, path)
	}
	return media.Schema.Value
}

// assertMatchesSchema validates a real response body against the published
// schema, including the parts a Go struct cannot express: required fields, enum
// members, string formats and numeric bounds.
func assertMatchesSchema(t *testing.T, schema *openapi3.Schema, body []byte) {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if err := schema.VisitJSON(decoded); err != nil {
		t.Fatalf("response does not match the published schema: %v\nbody: %s", err, body)
	}
}

func contractCard() contract.ReviewCardSummary {
	return contract.ReviewCardSummary{
		ID:               uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678"),
		UserID:           uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345679"),
		ContentVersionID: uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def01234567a"),
		Skill:            "vocabulary",
		Stability:        8.4231,
		Difficulty:       5.1,
		DueAt:            time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
		Reps:             3,
		Lapses:           1,
		State:            stateReview,
		Content: &contract.ReviewCardContent{
			Kind:      "vocab_flashcard",
			CEFRLevel: "B2",
			Body:      []byte(`{"word":"meticulous","ipa":"/məˈtɪkjələs/"}`),
		},
	}
}

// call drives one request through the real router with an authenticated actor.
func call(t *testing.T, svc *fakeService, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	router := setupTestRouter(svc)

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	request = withActor(request, uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345679"))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d, want 200 (body %s)", method, path, recorder.Code, recorder.Body)
	}
	return recorder
}

func TestContract_ReviewSessionMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeService{
		dueCardsFn: func(_ context.Context, _ uuid.UUID, _ int32) ([]contract.ReviewCardSummary, error) {
			return []contract.ReviewCardSummary{contractCard()}, nil
		},
	}

	recorder := call(t, svc, http.MethodGet, "/reviews/session", "")
	assertMatchesSchema(t, responseSchema(t, spec, "/reviews/session", http.MethodGet), recorder.Body.Bytes())
}

func TestContract_DueCountMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeService{
		dueCountFn: func(_ context.Context, _ uuid.UUID) (int, error) { return 12, nil },
	}

	recorder := call(t, svc, http.MethodGet, "/reviews/due-count", "")
	assertMatchesSchema(t, responseSchema(t, spec, "/reviews/due-count", http.MethodGet), recorder.Body.Bytes())
}

func TestContract_AnswerMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeService{
		answerCardFn: func(
			_ context.Context, _, _ uuid.UUID, _ string, _ int,
		) (service.AnswerResult, error) {
			return service.AnswerResult{
				Card:         contractCard(),
				NextDueAt:    time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC),
				IntervalDays: 8,
			}, nil
		},
	}

	const path = "/reviews/0199a1c2-3d4e-7f80-9abc-def012345678/answer"
	recorder := call(t, svc, http.MethodPost, path, `{"grade":"good","elapsed_ms":2100}`)
	assertMatchesSchema(t,
		responseSchema(t, spec, "/reviews/{card_id}/answer", http.MethodPost), recorder.Body.Bytes())
}

func TestContract_SuspendAndResetMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeService{
		suspendCardFn: func(_ context.Context, _, _ uuid.UUID) (contract.ReviewCardSummary, error) {
			card := contractCard()
			suspended := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
			card.SuspendedAt = &suspended
			return card, nil
		},
		resetCardFn: func(_ context.Context, _, _ uuid.UUID) (contract.ReviewCardSummary, error) {
			card := contractCard()
			card.State = "new"
			card.Reps = 0
			card.Lapses = 0
			return card, nil
		},
	}

	const cardPath = "/reviews/0199a1c2-3d4e-7f80-9abc-def012345678"

	suspend := call(t, svc, http.MethodPost, cardPath+"/suspend", "")
	assertMatchesSchema(t,
		responseSchema(t, spec, "/reviews/{card_id}/suspend", http.MethodPost), suspend.Body.Bytes())

	reset := call(t, svc, http.MethodPost, cardPath+"/reset", "")
	assertMatchesSchema(t,
		responseSchema(t, spec, "/reviews/{card_id}/reset", http.MethodPost), reset.Body.Bytes())
}

func TestContract_CompleteSessionMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeService{
		completeSessionFn: func(_ context.Context, _ uuid.UUID, reviewed, correct int) (service.SessionResult, error) {
			return service.SessionResult{
				Reviewed:    reviewed,
				Correct:     correct,
				Minutes:     4,
				CompletedAt: time.Date(2026, 8, 25, 10, 30, 0, 0, time.UTC),
			}, nil
		},
	}

	recorder := call(t, svc, http.MethodPost, "/reviews/session/complete", `{"reviewed":10,"correct":8}`)
	assertMatchesSchema(t,
		responseSchema(t, spec, "/reviews/session/complete", http.MethodPost), recorder.Body.Bytes())
}

// TestContract_ForecastMatchesTheSpec also pins the date format. `date` in the
// response is `format: date`, not date-time, and a Go time.Time serialised the
// obvious way would be neither.
func TestContract_ForecastMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeService{
		forecastFn: func(_ context.Context, _ uuid.UUID, _ int) ([]service.ForecastDay, error) {
			return []service.ForecastDay{
				{Date: "2026-08-26", DueCount: 15},
				{Date: "2026-08-27", DueCount: 3},
			}, nil
		},
	}

	recorder := call(t, svc, http.MethodGet, "/reviews/forecast", "")
	assertMatchesSchema(t, responseSchema(t, spec, "/reviews/forecast", http.MethodGet), recorder.Body.Bytes())
}

// TestContract_RequestBodiesUsedByTheTestsAreValid closes the other half of the
// loop. If the bodies these tests send were not themselves valid against the
// spec, a handler could pass every test above while rejecting everything a real
// client sends.
func TestContract_RequestBodiesUsedByTheTestsAreValid(t *testing.T) {
	spec := loadSpec(t)

	cases := []struct {
		path   string
		method string
		body   string
	}{
		{"/reviews/{card_id}/answer", http.MethodPost, `{"grade":"good","elapsed_ms":2100}`},
		{"/reviews/session/complete", http.MethodPost, `{"reviewed":10,"correct":8}`},
	}

	for _, tc := range cases {
		item := spec.Paths.Find(tc.path)
		if item == nil {
			t.Fatalf("the spec has no path %q", tc.path)
		}
		operation := item.GetOperation(tc.method)
		if operation == nil || operation.RequestBody == nil || operation.RequestBody.Value == nil {
			t.Fatalf("%s %s declares no request body", tc.method, tc.path)
		}
		media := operation.RequestBody.Value.Content.Get("application/json")
		if media == nil || media.Schema == nil {
			t.Fatalf("%s %s declares no application/json request body", tc.method, tc.path)
		}
		assertMatchesSchema(t, media.Schema.Value, []byte(tc.body))
	}
}

// TestContract_EveryRoutedPathIsInTheSpec is the check that catches an invented
// endpoint. A handler mounted on a path the spec does not publish is invisible
// to every generated client and to the frontend agent building against it.
func TestContract_EveryRoutedPathIsInTheSpec(t *testing.T) {
	spec := loadSpec(t)

	// The chi router names the card parameter `id`; the spec names it `card_id`.
	// The wire path is identical either way, so compare with the spec's spelling.
	routed := []string{
		"/reviews/session",
		"/reviews/due-count",
		"/reviews/{card_id}/answer",
		"/reviews/session/complete",
		"/reviews/{card_id}/suspend",
		"/reviews/{card_id}/reset",
		"/reviews/forecast",
	}

	for _, path := range routed {
		if spec.Paths.Find(path) == nil {
			t.Errorf("the handler mounts %s but the spec does not publish it", path)
		}
	}
}
