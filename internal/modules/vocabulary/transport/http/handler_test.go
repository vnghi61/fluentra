package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
	vocabularyhttp "github.com/fluentra/fluentra/internal/modules/vocabulary/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type fakeGuard struct{}

func (fakeGuard) Require(_ context.Context, _ string) error { return nil }

type fakeVocabService struct {
	lookupWordFn  func(ctx context.Context, lemma string) ([]domain.Word, error)
	searchWordsFn func(ctx context.Context, query string, limit, offset int32) ([]domain.Word, int, error)
	createWordFn  func(ctx context.Context, word domain.Word) (domain.Word, error)
	createDeckFn  func(
		ctx context.Context, ownerID *uuid.UUID, slug, name string, description *string, isPublic bool) (domain.Deck, error,
	)
	getDeckFn            func(ctx context.Context, deckID uuid.UUID) (domain.Deck, error)
	listDecksFn          func(ctx context.Context, userID *uuid.UUID) ([]domain.Deck, error)
	addWordToDeckFn      func(ctx context.Context, deckID, wordSenseID uuid.UUID) error
	removeWordFromDeckFn func(ctx context.Context, deckID, wordSenseID uuid.UUID) error
	listDeckWordsFn      func(ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]domain.DeckItem, error)
	setWordStateFn       func(ctx context.Context, userID, wordSenseID uuid.UUID, status domain.WordStatus) error
	getWordStateFn       func(ctx context.Context, userID, wordSenseID uuid.UUID) (domain.UserWordState, error)
}

func (f *fakeVocabService) LookupWord(ctx context.Context, lemma string) ([]domain.Word, error) {
	if f.lookupWordFn != nil {
		return f.lookupWordFn(ctx, lemma)
	}
	return nil, nil
}

func (f *fakeVocabService) SearchWords(
	ctx context.Context, query string, limit, offset int32,
) ([]domain.Word, int, error) {
	if f.searchWordsFn != nil {
		return f.searchWordsFn(ctx, query, limit, offset)
	}
	return nil, 0, nil
}

func (f *fakeVocabService) CreateWord(ctx context.Context, word domain.Word) (domain.Word, error) {
	if f.createWordFn != nil {
		return f.createWordFn(ctx, word)
	}
	return word, nil
}

func (f *fakeVocabService) CreateDeck(
	ctx context.Context, ownerID *uuid.UUID, slug, name string, description *string, isPublic bool) (domain.Deck, error,
) {
	if f.createDeckFn != nil {
		return f.createDeckFn(ctx, ownerID, slug, name, description, isPublic)
	}
	return domain.Deck{
		ID: uuid.New(), OwnerID: ownerID, Slug: slug, Name: name,
		Description: description, IsPublic: isPublic,
	}, nil
}

func (f *fakeVocabService) GetDeck(ctx context.Context, deckID uuid.UUID) (domain.Deck, error) {
	if f.getDeckFn != nil {
		return f.getDeckFn(ctx, deckID)
	}
	return domain.Deck{ID: deckID}, nil
}

func (f *fakeVocabService) ListDecks(ctx context.Context, userID *uuid.UUID) ([]domain.Deck, error) {
	if f.listDecksFn != nil {
		return f.listDecksFn(ctx, userID)
	}
	return nil, nil
}

func (f *fakeVocabService) AddWordToDeck(ctx context.Context, deckID, wordSenseID uuid.UUID) error {
	if f.addWordToDeckFn != nil {
		return f.addWordToDeckFn(ctx, deckID, wordSenseID)
	}
	return nil
}

func (f *fakeVocabService) RemoveWordFromDeck(ctx context.Context, deckID, wordSenseID uuid.UUID) error {
	if f.removeWordFromDeckFn != nil {
		return f.removeWordFromDeckFn(ctx, deckID, wordSenseID)
	}
	return nil
}

func (f *fakeVocabService) ListDeckWords(
	ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]domain.DeckItem, error,
) {
	if f.listDeckWordsFn != nil {
		return f.listDeckWordsFn(ctx, deckID, limit, offset)
	}
	return nil, nil
}

func (f *fakeVocabService) SetWordState(
	ctx context.Context, userID, wordSenseID uuid.UUID, status domain.WordStatus,
) error {
	if f.setWordStateFn != nil {
		return f.setWordStateFn(ctx, userID, wordSenseID, status)
	}
	return nil
}

func (f *fakeVocabService) GetWordState(
	ctx context.Context, userID, wordSenseID uuid.UUID) (domain.UserWordState, error,
) {
	if f.getWordStateFn != nil {
		return f.getWordStateFn(ctx, userID, wordSenseID)
	}
	return domain.UserWordState{UserID: userID, WordSenseID: wordSenseID, Status: domain.StatusNew}, nil
}

func setupTestRouter(svc vocabularyhttp.VocabularyService) chi.Router {
	r := chi.NewRouter()
	h, err := vocabularyhttp.NewHandler(svc, fakeGuard{})
	if err != nil {
		panic(err)
	}
	h.Routes(r)
	h.AdminRoutes(r)
	return r
}

func withActor(req *http.Request, userID uuid.UUID) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   "user",
	}
	ctx := httpx.WithActor(req.Context(), actor)
	return req.WithContext(ctx)
}

func TestHTTP_LookupWord(t *testing.T) {
	audioURL := "/api/v1/content/assets/11111111-1111-1111-1111-111111111111"
	ipa := "/həˈloʊ/"
	svc := &fakeVocabService{
		lookupWordFn: func(_ context.Context, lemma string) ([]domain.Word, error) {
			assert.Equal(t, "hello", lemma)
			return []domain.Word{
				{
					ID:        uuid.New(),
					Lemma:     "hello",
					POS:       domain.POSInterjection,
					CEFRLevel: domain.CEFRA1,
					IPA:       &ipa,
					AudioURL:  &audioURL,
					Senses: []domain.WordSense{
						{
							ID:         uuid.New(),
							Definition: "Used as a greeting or to begin a phone conversation.",
						},
					},
				},
			}, nil
		},
	}

	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/vocabulary/words/hello", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp vocabularyhttp.DictionaryLookupResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Words, 1)
	assert.Equal(t, "hello", resp.Words[0].Lemma)
	assert.Equal(t, audioURL, *resp.Words[0].AudioURL)
}

func TestHTTP_SearchWords(t *testing.T) {
	svc := &fakeVocabService{
		searchWordsFn: func(_ context.Context, query string, _, _ int32) ([]domain.Word, int, error) {
			assert.Equal(t, "flo", query)
			return []domain.Word{
				{
					ID:        uuid.New(),
					Lemma:     "flower",
					POS:       domain.POSNoun,
					CEFRLevel: domain.CEFRA1,
				},
				{
					ID:        uuid.New(),
					Lemma:     "flow",
					POS:       domain.POSVerb,
					CEFRLevel: domain.CEFRB1,
				},
			}, 2, nil
		},
	}

	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/vocabulary/search?q=flo", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp vocabularyhttp.SearchWordsResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, 2, resp.Total, "total is counted, not the page length")
}

func TestHTTP_DeckLifecycle(t *testing.T) {
	userID := uuid.New()
	deckID := uuid.New()
	senseID := uuid.New()
	now := time.Now().UTC()

	svc := &fakeVocabService{
		createDeckFn: func(_ context.Context, ownerID *uuid.UUID, slug, name string, _ *string, _ bool) (domain.Deck, error) {
			assert.Equal(t, userID, *ownerID)
			assert.Equal(t, "ielts-8", slug)
			return domain.Deck{
				ID:        deckID,
				OwnerID:   ownerID,
				Slug:      slug,
				Name:      name,
				CreatedAt: now,
			}, nil
		},
		addWordToDeckFn: func(_ context.Context, dID, sID uuid.UUID) error {
			assert.Equal(t, deckID, dID)
			assert.Equal(t, senseID, sID)
			return nil
		},
		listDeckWordsFn: func(_ context.Context, dID uuid.UUID, _, _ int32) ([]domain.DeckItem, error) {
			assert.Equal(t, deckID, dID)
			return []domain.DeckItem{
				{
					ID:          senseID,
					DeckID:      dID,
					WordSenseID: senseID,
					Sense: &domain.WordSense{
						ID:         senseID,
						Definition: "Comprehensive description",
					},
					Word: &domain.Word{
						Lemma:     "resilient",
						POS:       domain.POSAdjective,
						CEFRLevel: domain.CEFRC1,
					},
					AddedAt: now,
				},
			}, nil
		},
	}

	router := setupTestRouter(svc)

	// 1. Create Deck
	createBody, _ := json.Marshal(vocabularyhttp.CreateDeckRequest{
		Slug: "ielts-8",
		Name: "IELTS Band 8 Vocab",
	})
	req := httptest.NewRequest(http.MethodPost, "/vocabulary/decks", bytes.NewReader(createBody))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// 2. Add word
	addBody, _ := json.Marshal(vocabularyhttp.AddWordToDeckRequest{
		WordSenseID: senseID,
	})
	req = httptest.NewRequest(http.MethodPost, "/vocabulary/decks/"+deckID.String()+"/words", bytes.NewReader(addBody))
	req = withActor(req, userID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// 3. List deck words
	req = httptest.NewRequest(http.MethodGet, "/vocabulary/decks/"+deckID.String()+"/words", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var listResp vocabularyhttp.ListDeckWordsResponse
	err := json.Unmarshal(rec.Body.Bytes(), &listResp)
	require.NoError(t, err)
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, "resilient", listResp.Items[0].Lemma)
}

func TestHTTP_SetWordState(t *testing.T) {
	userID := uuid.New()
	senseID := uuid.New()

	firstSeen := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	svc := &fakeVocabService{
		setWordStateFn: func(_ context.Context, uID, sID uuid.UUID, status domain.WordStatus) error {
			assert.Equal(t, userID, uID)
			assert.Equal(t, senseID, sID)
			assert.Equal(t, domain.StatusKnown, status)
			return nil
		},
		// The handler reads the stored row back, because first_seen_at is part of
		// the published response and only the row knows when the learner first
		// met this sense.
		getWordStateFn: func(_ context.Context, uID, sID uuid.UUID) (domain.UserWordState, error) {
			return domain.UserWordState{
				UserID:      uID,
				WordSenseID: sID,
				Status:      domain.StatusKnown,
				FirstSeenAt: firstSeen,
				UpdatedAt:   time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	router := setupTestRouter(svc)

	body, _ := json.Marshal(vocabularyhttp.SetWordStateRequest{
		Status: "known",
	})
	req := httptest.NewRequest(http.MethodPost, "/vocabulary/words/"+senseID.String()+"/state", bytes.NewReader(body))
	req = withActor(req, userID)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp vocabularyhttp.SetWordStateResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "known", resp.Status)
	assert.Equal(t, userID, resp.UserID)
	assert.Equal(t, senseID, resp.WordSenseID)
	assert.Equal(t, firstSeen, resp.FirstSeenAt, "first_seen_at comes from the stored row, not from now()")
}
