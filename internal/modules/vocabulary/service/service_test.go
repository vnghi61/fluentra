package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type fakeRepo struct {
	words      map[uuid.UUID]sqlc.SkillWord
	senses     map[uuid.UUID]sqlc.SkillWordSense
	relations  map[uuid.UUID]sqlc.SkillWordRelation
	decks      map[uuid.UUID]sqlc.SkillDeck
	deckItems  map[string]sqlc.SkillDeckItem
	wordStates map[string]sqlc.SkillUserWordState
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		words:      make(map[uuid.UUID]sqlc.SkillWord),
		senses:     make(map[uuid.UUID]sqlc.SkillWordSense),
		relations:  make(map[uuid.UUID]sqlc.SkillWordRelation),
		decks:      make(map[uuid.UUID]sqlc.SkillDeck),
		deckItems:  make(map[string]sqlc.SkillDeckItem),
		wordStates: make(map[string]sqlc.SkillUserWordState),
	}
}

func (f *fakeRepo) WithTx(_ pgx.Tx) repository.Repository { return f }

func (f *fakeRepo) InsertWord(_ context.Context, arg sqlc.InsertWordParams) (sqlc.SkillWord, error) {
	for _, w := range f.words {
		if w.Lemma == arg.Lemma && w.Pos == arg.Pos {
			w.CefrLevel = arg.CefrLevel
			w.FrequencyRank = arg.FrequencyRank
			w.Ipa = arg.Ipa
			w.AudioAssetID = arg.AudioAssetID
			w.UpdatedAt = time.Now().UTC()
			f.words[w.ID] = w
			return w, nil
		}
	}
	id := uuid.New()
	word := sqlc.SkillWord{
		ID:            id,
		Lemma:         arg.Lemma,
		Pos:           arg.Pos,
		CefrLevel:     arg.CefrLevel,
		FrequencyRank: arg.FrequencyRank,
		Ipa:           arg.Ipa,
		AudioAssetID:  arg.AudioAssetID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	f.words[id] = word
	return word, nil
}

func (f *fakeRepo) GetWordByLemmaAndPOS(_ context.Context, lemma, pos string) (sqlc.SkillWord, error) {
	for _, w := range f.words {
		if w.Lemma == lemma && w.Pos == pos {
			return w, nil
		}
	}
	return sqlc.SkillWord{}, pgx.ErrNoRows
}

func (f *fakeRepo) GetWordByID(_ context.Context, id uuid.UUID) (sqlc.SkillWord, error) {
	w, ok := f.words[id]
	if !ok {
		return sqlc.SkillWord{}, pgx.ErrNoRows
	}
	return w, nil
}

func (f *fakeRepo) ListWordsByLemma(_ context.Context, lemma string) ([]sqlc.SkillWord, error) {
	var result []sqlc.SkillWord
	for _, w := range f.words {
		if w.Lemma == lemma {
			result = append(result, w)
		}
	}
	return result, nil
}

func (f *fakeRepo) SearchWords(_ context.Context, _ string, limit, _ int32) ([]sqlc.SkillWord, error) {
	var result []sqlc.SkillWord
	for _, w := range f.words {
		result = append(result, w)
		if len(result) >= int(limit) {
			break
		}
	}
	return result, nil
}

func (f *fakeRepo) CountSearchWords(_ context.Context, _ string) (int64, error) {
	return int64(len(f.words)), nil
}

func (f *fakeRepo) InsertWordSense(_ context.Context, arg sqlc.InsertWordSenseParams) (sqlc.SkillWordSense, error) {
	id := uuid.New()
	sense := sqlc.SkillWordSense{
		ID:               id,
		WordID:           arg.WordID,
		ContentVersionID: arg.ContentVersionID,
		Definition:       arg.Definition,
		DefinitionVi:     arg.DefinitionVi,
		Register:         arg.Register,
		Domain:           arg.Domain,
		Examples:         arg.Examples,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	f.senses[id] = sense
	return sense, nil
}

func (f *fakeRepo) ListSensesByWordID(_ context.Context, wordID uuid.UUID) ([]sqlc.SkillWordSense, error) {
	var result []sqlc.SkillWordSense
	for _, s := range f.senses {
		if s.WordID == wordID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (f *fakeRepo) GetSenseByID(_ context.Context, id uuid.UUID) (sqlc.GetSenseByIDRow, error) {
	s, ok := f.senses[id]
	if !ok {
		return sqlc.GetSenseByIDRow{}, pgx.ErrNoRows
	}
	w := f.words[s.WordID]
	return sqlc.GetSenseByIDRow{
		ID:               s.ID,
		WordID:           s.WordID,
		ContentVersionID: s.ContentVersionID,
		Definition:       s.Definition,
		DefinitionVi:     s.DefinitionVi,
		Register:         s.Register,
		Domain:           s.Domain,
		Examples:         s.Examples,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
		Lemma:            w.Lemma,
		Pos:              w.Pos,
		CefrLevel:        w.CefrLevel,
		Ipa:              w.Ipa,
		AudioAssetID:     w.AudioAssetID,
	}, nil
}

func (f *fakeRepo) ListSensesByIDs(_ context.Context, ids []uuid.UUID) ([]sqlc.ListSensesByIDsRow, error) {
	var result []sqlc.ListSensesByIDsRow
	for _, id := range ids {
		if s, ok := f.senses[id]; ok {
			w := f.words[s.WordID]
			result = append(result, sqlc.ListSensesByIDsRow{
				ID:               s.ID,
				WordID:           s.WordID,
				ContentVersionID: s.ContentVersionID,
				Definition:       s.Definition,
				DefinitionVi:     s.DefinitionVi,
				Register:         s.Register,
				Domain:           s.Domain,
				Examples:         s.Examples,
				CreatedAt:        s.CreatedAt,
				UpdatedAt:        s.UpdatedAt,
				Lemma:            w.Lemma,
				Pos:              w.Pos,
				CefrLevel:        w.CefrLevel,
				Ipa:              w.Ipa,
				AudioAssetID:     w.AudioAssetID,
			})
		}
	}
	return result, nil
}

func (f *fakeRepo) InsertWordRelation(
	_ context.Context, arg sqlc.InsertWordRelationParams) (sqlc.SkillWordRelation, error,
) {
	id := uuid.New()
	rel := sqlc.SkillWordRelation{
		ID:         id,
		FromWordID: arg.FromWordID,
		ToWordID:   arg.ToWordID,
		Relation:   arg.Relation,
		CreatedAt:  time.Now().UTC(),
	}
	f.relations[id] = rel
	return rel, nil
}

func (f *fakeRepo) ListRelationsByWordID(_ context.Context, wordID uuid.UUID) ([]sqlc.ListRelationsByWordIDRow, error) {
	var result []sqlc.ListRelationsByWordIDRow
	for _, r := range f.relations {
		if r.FromWordID == wordID {
			target := f.words[r.ToWordID]
			result = append(result, sqlc.ListRelationsByWordIDRow{
				ID:          r.ID,
				FromWordID:  r.FromWordID,
				ToWordID:    r.ToWordID,
				Relation:    r.Relation,
				CreatedAt:   r.CreatedAt,
				TargetLemma: target.Lemma,
				TargetPos:   target.Pos,
			})
		}
	}
	return result, nil
}

func (f *fakeRepo) UpsertUserWordState(
	_ context.Context, arg sqlc.UpsertUserWordStateParams) (sqlc.SkillUserWordState, error,
) {
	key := arg.UserID.String() + ":" + arg.WordSenseID.String()
	st, ok := f.wordStates[key]
	if !ok {
		st = sqlc.SkillUserWordState{
			ID:          uuid.New(),
			UserID:      arg.UserID,
			WordSenseID: arg.WordSenseID,
			Status:      arg.Status,
			FirstSeenAt: time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
	} else {
		st.Status = arg.Status
		st.UpdatedAt = time.Now().UTC()
	}
	f.wordStates[key] = st
	return st, nil
}

func (f *fakeRepo) GetUserWordState(_ context.Context, userID, wordSenseID uuid.UUID) (sqlc.SkillUserWordState, error) {
	key := userID.String() + ":" + wordSenseID.String()
	st, ok := f.wordStates[key]
	if !ok {
		return sqlc.SkillUserWordState{}, pgx.ErrNoRows
	}
	return st, nil
}

func (f *fakeRepo) InsertDeck(_ context.Context, arg sqlc.InsertDeckParams) (sqlc.SkillDeck, error) {
	id := uuid.New()
	deck := sqlc.SkillDeck{
		ID:          id,
		OwnerID:     arg.OwnerID,
		Slug:        arg.Slug,
		Name:        arg.Name,
		Description: arg.Description,
		IsPublic:    arg.IsPublic,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	f.decks[id] = deck
	return deck, nil
}

func (f *fakeRepo) GetDeckByID(_ context.Context, id uuid.UUID) (sqlc.SkillDeck, error) {
	d, ok := f.decks[id]
	if !ok {
		return sqlc.SkillDeck{}, pgx.ErrNoRows
	}
	return d, nil
}

func (f *fakeRepo) ListDecksByUser(_ context.Context, userID *uuid.UUID) ([]sqlc.ListDecksByUserRow, error) {
	var result []sqlc.ListDecksByUserRow
	for _, d := range f.decks {
		if (userID != nil && d.OwnerID != nil && *d.OwnerID == *userID) || d.IsPublic {
			result = append(result, sqlc.ListDecksByUserRow{
				ID:          d.ID,
				OwnerID:     d.OwnerID,
				Slug:        d.Slug,
				Name:        d.Name,
				Description: d.Description,
				IsPublic:    d.IsPublic,
				CreatedAt:   d.CreatedAt,
				UpdatedAt:   d.UpdatedAt,
				ItemCount:   0,
			})
		}
	}
	return result, nil
}

func (f *fakeRepo) InsertDeckItem(_ context.Context, arg sqlc.InsertDeckItemParams) (sqlc.SkillDeckItem, error) {
	key := arg.DeckID.String() + ":" + arg.WordSenseID.String()
	item := sqlc.SkillDeckItem{
		ID:          uuid.New(),
		DeckID:      arg.DeckID,
		WordSenseID: arg.WordSenseID,
		CreatedAt:   time.Now().UTC(),
	}
	f.deckItems[key] = item
	return item, nil
}

func (f *fakeRepo) DeleteDeckItem(_ context.Context, deckID, wordSenseID uuid.UUID) error {
	key := deckID.String() + ":" + wordSenseID.String()
	delete(f.deckItems, key)
	return nil
}

func (f *fakeRepo) ListDeckWords(_ context.Context, deckID uuid.UUID, _, _ int32) ([]sqlc.ListDeckWordsRow, error) {
	var result []sqlc.ListDeckWordsRow
	for _, item := range f.deckItems {
		if item.DeckID == deckID {
			s := f.senses[item.WordSenseID]
			w := f.words[s.WordID]
			result = append(result, sqlc.ListDeckWordsRow{
				AddedAt:          item.CreatedAt,
				SenseID:          s.ID,
				WordID:           w.ID,
				ContentVersionID: s.ContentVersionID,
				Definition:       s.Definition,
				DefinitionVi:     s.DefinitionVi,
				Register:         s.Register,
				Domain:           s.Domain,
				Examples:         s.Examples,
				Lemma:            w.Lemma,
				Pos:              w.Pos,
				CefrLevel:        w.CefrLevel,
				Ipa:              w.Ipa,
				AudioAssetID:     w.AudioAssetID,
			})
		}
	}
	return result, nil
}

func TestVocabulary_LookupWord_And_Senses(t *testing.T) {
	repo := newFakeRepo()
	svc := service.New(service.Deps{
		Repo:  repo,
		Clock: clock.Real{},
	})
	ctx := context.Background()

	audioID := uuid.New()
	ipa := "/bæŋk/"
	word, err := svc.CreateWord(ctx, domain.Word{
		Lemma:        "bank",
		POS:          domain.POSNoun,
		CEFRLevel:    domain.CEFRA1,
		IPA:          &ipa,
		AudioAssetID: &audioID,
	})
	require.NoError(t, err)

	examples := []domain.ExampleSentence{
		{Sentence: "I need to go to the bank to deposit money."},
	}
	sense1, err := svc.CreateSense(ctx, domain.WordSense{
		WordID:     word.ID,
		Definition: "A financial institution where you keep money.",
		Examples:   examples,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, sense1.ID)

	// Lookup word
	words, err := svc.LookupWord(ctx, "bank")
	require.NoError(t, err)
	require.Len(t, words, 1)
	assert.Equal(t, "bank", words[0].Lemma)
	// Audio stays null until content.Reader can resolve a media asset; see
	// resolveAudioURL. A URL pointing at an endpoint that does not exist would
	// pass this assertion and 404 in the flashcard.
	assert.Nil(t, words[0].AudioURL)
	require.Len(t, words[0].Senses, 1)
	assert.Equal(t, "A financial institution where you keep money.", words[0].Senses[0].Definition)
}

func TestVocabulary_DeckCRUD_And_Words(t *testing.T) {
	repo := newFakeRepo()
	svc := service.New(service.Deps{
		Repo:  repo,
		Clock: clock.Real{},
	})
	ctx := context.Background()

	userID := uuid.New()
	audioID := uuid.New()
	word, err := svc.CreateWord(ctx, domain.Word{
		Lemma:        "ubiquitous",
		POS:          domain.POSAdjective,
		CEFRLevel:    domain.CEFRC1,
		AudioAssetID: &audioID,
	})
	require.NoError(t, err)

	sense, err := svc.CreateSense(ctx, domain.WordSense{
		WordID:     word.ID,
		Definition: "Present, appearing, or found everywhere.",
	})
	require.NoError(t, err)

	// Create deck
	desc := "Advanced C1 vocabulary"
	deck, err := svc.CreateDeck(ctx, &userID, "c1-vocab", "C1 Vocab", &desc, false)
	require.NoError(t, err)
	assert.Equal(t, "C1 Vocab", deck.Name)

	// Add word to deck
	err = svc.AddWordToDeck(ctx, deck.ID, sense.ID)
	require.NoError(t, err)

	// List deck words
	items, err := svc.ListDeckWords(ctx, deck.ID, 20, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "ubiquitous", items[0].Word.Lemma)
	assert.Equal(t, "Present, appearing, or found everywhere.", items[0].Sense.Definition)
	assert.Nil(t, items[0].Word.AudioURL)

	// Remove word from deck
	err = svc.RemoveWordFromDeck(ctx, deck.ID, sense.ID)
	require.NoError(t, err)

	itemsAfter, err := svc.ListDeckWords(ctx, deck.ID, 20, 0)
	require.NoError(t, err)
	assert.Empty(t, itemsAfter)
}

func TestVocabulary_UserWordState(t *testing.T) {
	repo := newFakeRepo()
	svc := service.New(service.Deps{
		Repo:  repo,
		Clock: clock.Real{},
	})
	ctx := context.Background()

	userID := uuid.New()
	senseID := uuid.New()

	// Default state is new
	st, err := svc.GetWordState(ctx, userID, senseID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusNew, st.Status)

	// Mark known
	err = svc.SetWordState(ctx, userID, senseID, domain.StatusKnown)
	require.NoError(t, err)

	stKnown, err := svc.GetWordState(ctx, userID, senseID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusKnown, stKnown.Status)

	// Mark ignored (BR-VOCABULARY-08)
	err = svc.SetWordState(ctx, userID, senseID, domain.StatusIgnored)
	require.NoError(t, err)

	stIgnored, err := svc.GetWordState(ctx, userID, senseID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusIgnored, stIgnored.Status)
}

// spyReviewScheduler records what vocabulary asks srs to do with a learner's cards.
type spyReviewScheduler struct {
	userID     uuid.UUID
	versionIDs []uuid.UUID
	suspended  bool
	calls      int
}

func (s *spyReviewScheduler) SetCardsSuspended(
	_ context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool,
) error {
	s.userID = userID
	s.versionIDs = contentVersionIDs
	s.suspended = suspended
	s.calls++
	return nil
}

// TestVocabulary_MarkingAWordKnownStopsItsScheduling is half of the P9.4
// acceptance criterion. "Known" has to reach the due queue, not just a column in
// skill.user_word_state, and srs owns learn.review_cards — so the assertion is
// that vocabulary asks through the contract. The other half, that a suspended
// card leaves the queue, is asserted in the srs service tests.
func TestVocabulary_MarkingAWordKnownStopsItsScheduling(t *testing.T) {
	userID := uuid.New()
	wordID := uuid.New()
	senseID := uuid.New()
	versionID := uuid.New()

	repo := newFakeRepo()
	repo.words[wordID] = sqlc.SkillWord{ID: wordID, Lemma: wordMeticulous, Pos: "adjective", CefrLevel: "B2"}
	repo.senses[senseID] = sqlc.SkillWordSense{
		ID:               senseID,
		WordID:           wordID,
		ContentVersionID: &versionID,
		Definition:       "Showing great attention to detail.",
	}

	spy := &spyReviewScheduler{}
	svc := service.New(service.Deps{Repo: repo, Reviews: spy})
	ctx := context.Background()

	require.NoError(t, svc.SetWordState(ctx, userID, senseID, domain.StatusKnown))
	require.Equal(t, 1, spy.calls)
	assert.Equal(t, userID, spy.userID)
	assert.Equal(t, []uuid.UUID{versionID}, spy.versionIDs)
	assert.True(t, spy.suspended, "a known word must leave the review rotation")

	// And the door swings back: un-marking it resumes scheduling.
	require.NoError(t, svc.SetWordState(ctx, userID, senseID, domain.StatusLearning))
	require.Equal(t, 2, spy.calls)
	assert.False(t, spy.suspended, "a word back in learning must be scheduled again")
}

// TestVocabulary_IgnoredWordAlsoStopsScheduling: "ignored" is the same promise
// as "known" from the learner's point of view — stop asking me this.
func TestVocabulary_IgnoredWordAlsoStopsScheduling(t *testing.T) {
	userID := uuid.New()
	wordID := uuid.New()
	senseID := uuid.New()
	versionID := uuid.New()

	repo := newFakeRepo()
	repo.words[wordID] = sqlc.SkillWord{ID: wordID, Lemma: "ephemeral", Pos: "adjective", CefrLevel: "C1"}
	repo.senses[senseID] = sqlc.SkillWordSense{ID: senseID, WordID: wordID, ContentVersionID: &versionID}

	spy := &spyReviewScheduler{}
	svc := service.New(service.Deps{Repo: repo, Reviews: spy})

	require.NoError(t, svc.SetWordState(context.Background(), userID, senseID, domain.StatusIgnored))
	assert.True(t, spy.suspended)
}
