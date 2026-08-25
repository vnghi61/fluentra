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

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
)

// The contract tests answer one question the unit tests cannot: does what this
// handler actually writes match what api/openapi/openapi.yaml promises?
//
// The DTOs in this package are hand-written rather than taken from the generated
// models, because a business module importing api/openapi would couple it to
// every other module's spec. That choice is only safe if something checks the
// two agree — this is that something. Run with `make test-contract`.

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

// responseSchema pulls the schema for one operation's success response.
func responseSchema(t *testing.T, spec *openapi3.T, path, method string, status int) *openapi3.Schema {
	t.Helper()

	item := spec.Paths.Find(path)
	if item == nil {
		t.Fatalf("the spec has no path %q", path)
	}
	operation := item.GetOperation(method)
	if operation == nil {
		t.Fatalf("the spec has no %s %s", method, path)
	}
	response := operation.Responses.Status(status)
	if response == nil || response.Value == nil {
		t.Fatalf("%s %s declares no %d response", method, path, status)
	}
	media := response.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s declares no application/json %d body", method, path, status)
	}
	return media.Schema.Value
}

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

const (
	contractUserID = "0199a1c2-3d4e-7f80-9abc-def012345679"
	contractWordID = "0199a1c2-3d4e-7f80-9abc-def01234567b"
	contractSense  = "0199a1c2-3d4e-7f80-9abc-def01234567c"
	contractDeckID = "0199a1c2-3d4e-7f80-9abc-def01234567d"

	pathDecks     = "/vocabulary/decks"
	pathDeckWords = "/vocabulary/decks/{id}/words"
)

func ptr[T any](v T) *T { return &v }

func contractWord() domain.Word {
	return domain.Word{
		ID:            uuid.MustParse(contractWordID),
		Lemma:         "meticulous",
		POS:           domain.POSAdjective,
		CEFRLevel:     domain.CEFRB2,
		FrequencyRank: ptr(4821),
		IPA:           ptr("/məˈtɪkjələs/"),
		Senses: []domain.WordSense{{
			ID:         uuid.MustParse(contractSense),
			WordID:     uuid.MustParse(contractWordID),
			Definition: "Showing great attention to detail.",
			Examples: []domain.ExampleSentence{{
				Sentence:   "She kept meticulous records of every transaction.",
				SentenceVi: ptr("Cô ấy lưu giữ hồ sơ tỉ mỉ về mọi giao dịch."),
			}},
			CreatedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
		}},
	}
}

func contractDeck() domain.Deck {
	return domain.Deck{
		ID:        uuid.MustParse(contractDeckID),
		OwnerID:   ptr(uuid.MustParse(contractUserID)),
		Slug:      "ielts-academic-vocab",
		Name:      "IELTS Academic Vocabulary",
		IsPublic:  false,
		ItemCount: 50,
		CreatedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
	}
}

// call drives one request through the real router with an authenticated actor.
func call(
	t *testing.T, svc *fakeVocabService, method, path, body string, wantStatus int,
) *httptest.ResponseRecorder {
	t.Helper()

	router := setupTestRouter(svc)

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	request = withActor(request, uuid.MustParse(contractUserID))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d (body %s)", method, path, recorder.Code, wantStatus, recorder.Body)
	}
	return recorder
}

func TestContract_LookupWordMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeVocabService{
		lookupWordFn: func(_ context.Context, _ string) ([]domain.Word, error) {
			return []domain.Word{contractWord()}, nil
		},
	}

	recorder := call(t, svc, http.MethodGet, "/vocabulary/words/meticulous", "", http.StatusOK)
	assertMatchesSchema(t,
		responseSchema(t, spec, "/vocabulary/words/{lemma}", http.MethodGet, http.StatusOK),
		recorder.Body.Bytes())
}

func TestContract_SearchMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeVocabService{
		searchWordsFn: func(_ context.Context, _ string, _, _ int32) ([]domain.Word, int, error) {
			return []domain.Word{contractWord()}, 1, nil
		},
	}

	recorder := call(t, svc, http.MethodGet, "/vocabulary/search?q=meti", "", http.StatusOK)
	assertMatchesSchema(t,
		responseSchema(t, spec, "/vocabulary/search", http.MethodGet, http.StatusOK),
		recorder.Body.Bytes())
}

func TestContract_DeckEndpointsMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeVocabService{
		listDecksFn: func(_ context.Context, _ *uuid.UUID) ([]domain.Deck, error) {
			return []domain.Deck{contractDeck()}, nil
		},
		createDeckFn: func(
			_ context.Context, ownerID *uuid.UUID, slug, name string, _ *string, isPublic bool,
		) (domain.Deck, error) {
			deck := contractDeck()
			deck.OwnerID, deck.Slug, deck.Name, deck.IsPublic = ownerID, slug, name, isPublic
			return deck, nil
		},
	}

	list := call(t, svc, http.MethodGet, "/vocabulary/decks", "", http.StatusOK)
	assertMatchesSchema(t,
		responseSchema(t, spec, pathDecks, http.MethodGet, http.StatusOK), list.Body.Bytes())

	created := call(t, svc, http.MethodPost, pathDecks,
		`{"slug":"ielts-academic-vocab","name":"IELTS Academic Vocabulary","is_public":false}`,
		http.StatusCreated)
	assertMatchesSchema(t,
		responseSchema(t, spec, pathDecks, http.MethodPost, http.StatusCreated), created.Body.Bytes())
}

// TestContract_ListDeckWordsMatchesTheSpec covers the operation the handler
// served before the spec published it. P10.4 renders exactly these fields, so
// the schema and the DTO agreeing is what lets a frontend agent build the
// flashcard from the generated types alone.
func TestContract_ListDeckWordsMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	word := contractWord()
	sense := word.Senses[0]
	svc := &fakeVocabService{
		listDeckWordsFn: func(_ context.Context, deckID uuid.UUID, _, _ int32) ([]domain.DeckItem, error) {
			return []domain.DeckItem{{
				ID:          uuid.MustParse(contractSense),
				DeckID:      deckID,
				WordSenseID: uuid.MustParse(contractSense),
				Word:        &word,
				Sense:       &sense,
				AddedAt:     time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
			}}, nil
		},
	}

	recorder := call(t, svc, http.MethodGet, "/vocabulary/decks/"+contractDeckID+"/words", "", http.StatusOK)
	assertMatchesSchema(t,
		responseSchema(t, spec, pathDeckWords, http.MethodGet, http.StatusOK),
		recorder.Body.Bytes())
}

func TestContract_WordStateMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &fakeVocabService{
		setWordStateFn: func(_ context.Context, _, _ uuid.UUID, _ domain.WordStatus) error { return nil },
		getWordStateFn: func(_ context.Context, userID, senseID uuid.UUID) (domain.UserWordState, error) {
			return domain.UserWordState{
				ID:          uuid.MustParse(contractSense),
				UserID:      userID,
				WordSenseID: senseID,
				Status:      domain.StatusKnown,
				FirstSeenAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	recorder := call(t, svc, http.MethodPost,
		"/vocabulary/words/"+contractSense+"/state", `{"status":"known"}`, http.StatusOK)
	assertMatchesSchema(t,
		responseSchema(t, spec, "/vocabulary/words/{sense_id}/state", http.MethodPost, http.StatusOK),
		recorder.Body.Bytes())
}

// TestContract_RequestBodiesUsedByTheTestsAreValid closes the other half of the
// loop: a handler could pass every test above while rejecting what a real
// client, generated from this spec, actually sends.
func TestContract_RequestBodiesUsedByTheTestsAreValid(t *testing.T) {
	spec := loadSpec(t)

	cases := []struct {
		path   string
		method string
		body   string
	}{
		{
			pathDecks, http.MethodPost,
			`{"slug":"ielts-academic-vocab","name":"IELTS Academic Vocabulary","is_public":false}`,
		},
		{"/vocabulary/words/{sense_id}/state", http.MethodPost, `{"status":"known"}`},
		{pathDeckWords, http.MethodPost, `{"word_sense_id":"` + contractSense + `"}`},
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
// endpoint. GET /vocabulary/decks/{id}/words was exactly that: a working handler
// on a path no OpenAPI document published, so it existed for nobody generating
// a client. This test is why that cannot happen again silently.
func TestContract_EveryRoutedPathIsInTheSpec(t *testing.T) {
	spec := loadSpec(t)

	routed := []struct {
		path   string
		method string
	}{
		{"/vocabulary/words/{lemma}", http.MethodGet},
		{"/vocabulary/search", http.MethodGet},
		{pathDecks, http.MethodGet},
		{pathDecks, http.MethodPost},
		{pathDeckWords, http.MethodGet},
		{pathDeckWords, http.MethodPost},
		{"/vocabulary/decks/{id}/words/{sense_id}", http.MethodDelete},
		{"/vocabulary/words/{sense_id}/state", http.MethodPost},
		{"/admin/vocabulary/words", http.MethodPost},
	}

	for _, route := range routed {
		item := spec.Paths.Find(route.path)
		if item == nil {
			t.Errorf("the handler mounts %s but the spec does not publish that path", route.path)
			continue
		}
		if item.GetOperation(route.method) == nil {
			t.Errorf("the handler mounts %s %s but the spec publishes no such operation",
				route.method, route.path)
		}
	}
}
