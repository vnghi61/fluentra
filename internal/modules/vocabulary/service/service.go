// Package service implements vocabulary use cases: dictionary lookup, senses, decks, and word state.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// clampInt32 narrows a caller-supplied count to the column width without
// wrapping. A frequency rank is a small positive number; saturating a bad one is
// safer than the silent overflow a bare conversion produces.
func clampInt32(v int) int32 {
	switch {
	case v < 0:
		return 0
	case v > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(v)
	}
}

// The service clamps paging independently of the transport, because it is a
// contract-level guarantee: any caller, not only the HTTP handler, gets a
// bounded page.
const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

// ContentReader defines content asset operations needed by vocabulary.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

// ReviewScheduler is the slice of srs.CardWriter this module needs: a learner
// who declares a word known must stop seeing it in their review queue, and
// learn.review_cards belongs to srs.
type ReviewScheduler interface {
	SetCardsSuspended(ctx context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool) error
}

// OutboxTx is the database transaction interface needed to write outbox events.
type OutboxTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// EventWriter writes domain events to the outbox.
type EventWriter interface {
	Write(ctx context.Context, tx OutboxTx, aggregate, event string, payload any) (uuid.UUID, error)
}

// Deps carries dependencies for constructing the vocabulary service.
type Deps struct {
	Repo    repository.Repository
	Content ContentReader
	Reviews ReviewScheduler
	Events  EventWriter
	Clock   clock.Clock
	NewID   func() uuid.UUID
}

// Service orchestrates dictionary and deck operations.
type Service struct {
	repo    repository.Repository
	content ContentReader
	reviews ReviewScheduler
	events  EventWriter
	clock   clock.Clock
	newID   func() uuid.UUID
}

// New constructs a vocabulary Service.
func New(deps Deps) *Service {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	newID := deps.NewID
	if newID == nil {
		newID = uuid.New
	}
	return &Service{
		repo:    deps.Repo,
		content: deps.Content,
		reviews: deps.Reviews,
		events:  deps.Events,
		clock:   clk,
		newID:   newID,
	}
}

// LookupWord searches words by exact lemma and returns all matching POS variants with senses.
func (s *Service) LookupWord(ctx context.Context, lemma string) ([]domain.Word, error) {
	wordRows, err := s.repo.ListWordsByLemma(ctx, lemma)
	if err != nil {
		return nil, fmt.Errorf("failed to list words by lemma: %w", err)
	}

	result := make([]domain.Word, 0, len(wordRows))
	for _, w := range wordRows {
		senses, err := s.repo.ListSensesByWordID(ctx, w.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list senses for word %s: %w", w.ID, err)
		}

		relations, err := s.repo.ListRelationsByWordID(ctx, w.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list relations for word %s: %w", w.ID, err)
		}

		domWord := mapDomainWord(w)
		domWord.Senses = make([]domain.WordSense, 0, len(senses))
		for _, sense := range senses {
			domWord.Senses = append(domWord.Senses, mapDomainSense(sense))
		}

		domWord.Relations = make([]domain.WordRelation, 0, len(relations))
		for _, rel := range relations {
			domWord.Relations = append(domWord.Relations, mapDomainRelation(rel))
		}

		result = append(result, domWord)
	}

	return result, nil
}

// SearchWords queries the dictionary prefix index with pagination.
func (s *Service) SearchWords(ctx context.Context, query string, limit, offset int32) ([]domain.Word, int, error) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.repo.SearchWords(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search words: %w", err)
	}

	// The total is counted, not inferred from len(rows): a page of twenty tells
	// the client nothing about whether there are twenty-one matches or two
	// hundred, and a client that guesses paginates wrongly.
	total, err := s.repo.CountSearchWords(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count word search results: %w", err)
	}

	words := make([]domain.Word, 0, len(rows))
	for _, row := range rows {
		words = append(words, mapDomainWord(row))
	}
	return words, int(total), nil
}

// GetWordByID retrieves a word by ID.
func (s *Service) GetWordByID(ctx context.Context, id uuid.UUID) (domain.Word, error) {
	row, err := s.repo.GetWordByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Word{}, apperr.New(apperr.NotFound, "WORD_NOT_FOUND", "Word not found.")
		}
		return domain.Word{}, fmt.Errorf("failed to get word: %w", err)
	}

	senses, err := s.repo.ListSensesByWordID(ctx, row.ID)
	if err != nil {
		return domain.Word{}, fmt.Errorf("failed to list senses for word %s: %w", row.ID, err)
	}

	relations, err := s.repo.ListRelationsByWordID(ctx, row.ID)
	if err != nil {
		return domain.Word{}, fmt.Errorf("failed to list relations for word %s: %w", row.ID, err)
	}

	domWord := mapDomainWord(row)
	domWord.Senses = make([]domain.WordSense, 0, len(senses))
	for _, sense := range senses {
		domWord.Senses = append(domWord.Senses, mapDomainSense(sense))
	}
	domWord.Relations = make([]domain.WordRelation, 0, len(relations))
	for _, rel := range relations {
		domWord.Relations = append(domWord.Relations, mapDomainRelation(rel))
	}

	return domWord, nil
}

// CreateWord writes a new dictionary word entry.
func (s *Service) CreateWord(ctx context.Context, word domain.Word) (domain.Word, error) {
	var freqRank *int32
	if word.FrequencyRank != nil {
		v := clampInt32(*word.FrequencyRank)
		freqRank = &v
	}

	arg := sqlc.InsertWordParams{
		Lemma:         word.Lemma,
		Pos:           string(word.POS),
		CefrLevel:     string(word.CEFRLevel),
		FrequencyRank: freqRank,
		Ipa:           word.IPA,
		AudioAssetID:  word.AudioAssetID,
	}

	row, err := s.repo.InsertWord(ctx, arg)
	if err != nil {
		return domain.Word{}, fmt.Errorf("failed to create word: %w", err)
	}
	return mapDomainWord(row), nil
}

// CreateSense writes a word sense definition.
func (s *Service) CreateSense(ctx context.Context, sense domain.WordSense) (domain.WordSense, error) {
	examplesJSON, err := json.Marshal(sense.Examples)
	if err != nil {
		examplesJSON = []byte("[]")
	}

	arg := sqlc.InsertWordSenseParams{
		WordID:           sense.WordID,
		ContentVersionID: sense.ContentVersionID,
		Definition:       sense.Definition,
		DefinitionVi:     sense.DefinitionVi,
		Register:         sense.Register,
		Domain:           sense.Domain,
		Examples:         examplesJSON,
	}

	row, err := s.repo.InsertWordSense(ctx, arg)
	if err != nil {
		return domain.WordSense{}, fmt.Errorf("failed to create word sense: %w", err)
	}
	return mapDomainSense(row), nil
}

// GetSense retrieves a sense and its associated word.
func (s *Service) GetSense(ctx context.Context, senseID uuid.UUID) (domain.WordSense, error) {
	row, err := s.repo.GetSenseByID(ctx, senseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WordSense{}, apperr.New(apperr.NotFound, "SENSE_NOT_FOUND", "Word sense not found.")
		}
		return domain.WordSense{}, fmt.Errorf("failed to get word sense: %w", err)
	}

	var examples []domain.ExampleSentence
	if len(row.Examples) > 0 {
		_ = json.Unmarshal(row.Examples, &examples)
	}

	return domain.WordSense{
		ID:               row.ID,
		WordID:           row.WordID,
		ContentVersionID: row.ContentVersionID,
		Definition:       row.Definition,
		DefinitionVi:     row.DefinitionVi,
		Register:         row.Register,
		Domain:           row.Domain,
		Examples:         examples,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

// GetSenses retrieves multiple word senses by their IDs (satisfies Reader contract).
func (s *Service) GetSenses(ctx context.Context, senseIDs []uuid.UUID) ([]domain.WordSense, error) {
	if len(senseIDs) == 0 {
		return []domain.WordSense{}, nil
	}

	rows, err := s.repo.ListSensesByIDs(ctx, senseIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to list senses by IDs: %w", err)
	}

	result := make([]domain.WordSense, 0, len(rows))
	for _, row := range rows {
		var examples []domain.ExampleSentence
		if len(row.Examples) > 0 {
			_ = json.Unmarshal(row.Examples, &examples)
		}

		result = append(result, domain.WordSense{
			ID:               row.ID,
			WordID:           row.WordID,
			ContentVersionID: row.ContentVersionID,
			Definition:       row.Definition,
			DefinitionVi:     row.DefinitionVi,
			Register:         row.Register,
			Domain:           row.Domain,
			Examples:         examples,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
	}
	return result, nil
}

// AddRelation connects two words semantically.
func (s *Service) AddRelation(ctx context.Context, fromWordID, toWordID uuid.UUID, relation string) error {
	arg := sqlc.InsertWordRelationParams{
		FromWordID: fromWordID,
		ToWordID:   toWordID,
		Relation:   relation,
	}
	_, err := s.repo.InsertWordRelation(ctx, arg)
	if err != nil {
		return fmt.Errorf("failed to insert word relation: %w", err)
	}
	return nil
}

// CreateDeck creates a new vocabulary deck.
func (s *Service) CreateDeck(
	ctx context.Context, ownerID *uuid.UUID, slug, name string, description *string, isPublic bool) (domain.Deck, error,
) {
	if slug == "" || name == "" {
		return domain.Deck{}, apperr.New(apperr.Validation, "INVALID_DECK", "Slug and name are required.")
	}

	arg := sqlc.InsertDeckParams{
		OwnerID:     ownerID,
		Slug:        slug,
		Name:        name,
		Description: description,
		IsPublic:    isPublic,
	}

	row, err := s.repo.InsertDeck(ctx, arg)
	if err != nil {
		return domain.Deck{}, fmt.Errorf("failed to create deck: %w", err)
	}

	return mapDomainDeck(row, 0), nil
}

// GetDeck retrieves a deck by ID.
func (s *Service) GetDeck(ctx context.Context, deckID uuid.UUID) (domain.Deck, error) {
	row, err := s.repo.GetDeckByID(ctx, deckID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Deck{}, apperr.New(apperr.NotFound, "DECK_NOT_FOUND", "Deck not found.")
		}
		return domain.Deck{}, fmt.Errorf("failed to get deck: %w", err)
	}
	return mapDomainDeck(row, 0), nil
}

// ListDecks returns all accessible decks (learner's personal decks + curated public decks).
func (s *Service) ListDecks(ctx context.Context, userID *uuid.UUID) ([]domain.Deck, error) {
	rows, err := s.repo.ListDecksByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list decks: %w", err)
	}

	result := make([]domain.Deck, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.Deck{
			ID:          row.ID,
			OwnerID:     row.OwnerID,
			Slug:        row.Slug,
			Name:        row.Name,
			Description: row.Description,
			IsPublic:    row.IsPublic,
			ItemCount:   int(row.ItemCount),
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return result, nil
}

// AddWordToDeck adds a word sense into a deck.
func (s *Service) AddWordToDeck(ctx context.Context, deckID, wordSenseID uuid.UUID) error {
	arg := sqlc.InsertDeckItemParams{
		DeckID:      deckID,
		WordSenseID: wordSenseID,
	}
	if _, err := s.repo.InsertDeckItem(ctx, arg); err != nil {
		return fmt.Errorf("failed to add word to deck: %w", err)
	}
	return nil
}

// RemoveWordFromDeck removes a word sense from a deck.
func (s *Service) RemoveWordFromDeck(ctx context.Context, deckID, wordSenseID uuid.UUID) error {
	if err := s.repo.DeleteDeckItem(ctx, deckID, wordSenseID); err != nil {
		return fmt.Errorf("failed to remove word from deck: %w", err)
	}
	return nil
}

// ListDeckWords returns paginated words within a deck.
func (s *Service) ListDeckWords(ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]domain.DeckItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.repo.ListDeckWords(ctx, deckID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list deck words: %w", err)
	}

	result := make([]domain.DeckItem, 0, len(rows))
	for _, row := range rows {
		var examples []domain.ExampleSentence
		if len(row.Examples) > 0 {
			_ = json.Unmarshal(row.Examples, &examples)
		}

		audioURL := resolveAudioURL(row.AudioAssetID)

		result = append(result, domain.DeckItem{
			ID:          row.SenseID,
			DeckID:      deckID,
			WordSenseID: row.SenseID,
			Sense: &domain.WordSense{
				ID:               row.SenseID,
				WordID:           row.WordID,
				ContentVersionID: row.ContentVersionID,
				Definition:       row.Definition,
				DefinitionVi:     row.DefinitionVi,
				Register:         row.Register,
				Domain:           row.Domain,
				Examples:         examples,
			},
			Word: &domain.Word{
				ID:           row.WordID,
				Lemma:        row.Lemma,
				POS:          domain.PartOfSpeech(row.Pos),
				CEFRLevel:    domain.CEFRLevel(row.CefrLevel),
				IPA:          row.Ipa,
				AudioAssetID: row.AudioAssetID,
				AudioURL:     audioURL,
			},
			AddedAt: row.AddedAt,
		})
	}
	return result, nil
}

// SetWordState marks learner status (known, ignored, learning).
func (s *Service) SetWordState(ctx context.Context, userID, wordSenseID uuid.UUID, status domain.WordStatus) error {
	arg := sqlc.UpsertUserWordStateParams{
		UserID:      userID,
		WordSenseID: wordSenseID,
		Status:      string(status),
	}
	if _, err := s.repo.UpsertUserWordState(ctx, arg); err != nil {
		return fmt.Errorf("failed to set user word state: %w", err)
	}
	return s.syncReviewScheduling(ctx, userID, wordSenseID, status)
}

// syncReviewScheduling makes the word status mean something to the due queue.
//
// "Known" and "ignored" are the learner telling us to stop asking, so the srs
// card for the sense's content version is suspended; moving back to new or
// learning puts it back. Without this, marking a word known changes a column and
// the learner keeps being asked the same question tomorrow.
//
// A failure here is logged rather than returned: the state the learner asked for
// is already stored, and failing their request because the queue lagged would be
// the worse outcome. The next status change reconciles it.
func (s *Service) syncReviewScheduling(
	ctx context.Context, userID, wordSenseID uuid.UUID, status domain.WordStatus,
) error {
	if s.reviews == nil {
		return nil
	}

	sense, err := s.GetSense(ctx, wordSenseID)
	if err != nil {
		slog.WarnContext(ctx, "could not read the sense behind a word state change",
			"word_sense_id", wordSenseID, "error", err)
		return nil
	}
	if sense.ContentVersionID == nil {
		// A sense with no content version has no review card to suspend.
		return nil
	}

	suspended := status == domain.StatusKnown || status == domain.StatusIgnored
	if err := s.reviews.SetCardsSuspended(
		ctx, userID, []uuid.UUID{*sense.ContentVersionID}, suspended,
	); err != nil {
		slog.WarnContext(ctx, "failed to sync review scheduling with word state",
			"user_id", userID, "word_sense_id", wordSenseID, "error", err)
	}
	return nil
}

// GetWordState reads user progress on a word sense.
func (s *Service) GetWordState(ctx context.Context, userID, wordSenseID uuid.UUID) (domain.UserWordState, error) {
	row, err := s.repo.GetUserWordState(ctx, userID, wordSenseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UserWordState{
				UserID:      userID,
				WordSenseID: wordSenseID,
				Status:      domain.StatusNew,
			}, nil
		}
		return domain.UserWordState{}, fmt.Errorf("failed to get user word state: %w", err)
	}

	return domain.UserWordState{
		ID:          row.ID,
		UserID:      row.UserID,
		WordSenseID: row.WordSenseID,
		Status:      domain.WordStatus(row.Status),
		FirstSeenAt: row.FirstSeenAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// resolveAudioURL is deliberately unimplemented rather than guessed.
//
// A word's pronunciation lives in content.media_assets, which stores an
// object_key, not a URL — turning one into something a browser can play needs a
// presigned link from the storage capability, and that has to be reached through
// c_content, because vocabulary may not read another module's table. content's
// Reader exposes no media lookup today, so there is nothing honest to return.
//
// The earlier version of this function formatted "/api/v1/content/assets/{id}",
// an endpoint that appears in no OpenAPI path and serves nothing: a flashcard
// following it gets a 404, which is worse than a null the client can render
// around. See vocabulary/TODO.md — the fix is a content.Reader method, and it
// belongs to the task that adds one.
func resolveAudioURL(assetID *uuid.UUID) *string {
	_ = assetID
	return nil
}

func mapDomainWord(row sqlc.SkillWord) domain.Word {
	var freqRank *int
	if row.FrequencyRank != nil {
		v := int(*row.FrequencyRank)
		freqRank = &v
	}

	audioURL := resolveAudioURL(row.AudioAssetID)

	return domain.Word{
		ID:            row.ID,
		Lemma:         row.Lemma,
		POS:           domain.PartOfSpeech(row.Pos),
		CEFRLevel:     domain.CEFRLevel(row.CefrLevel),
		FrequencyRank: freqRank,
		IPA:           row.Ipa,
		AudioAssetID:  row.AudioAssetID,
		AudioURL:      audioURL,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapDomainSense(row sqlc.SkillWordSense) domain.WordSense {
	var examples []domain.ExampleSentence
	if len(row.Examples) > 0 {
		_ = json.Unmarshal(row.Examples, &examples)
	}

	return domain.WordSense{
		ID:               row.ID,
		WordID:           row.WordID,
		ContentVersionID: row.ContentVersionID,
		Definition:       row.Definition,
		DefinitionVi:     row.DefinitionVi,
		Register:         row.Register,
		Domain:           row.Domain,
		Examples:         examples,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func mapDomainRelation(row sqlc.ListRelationsByWordIDRow) domain.WordRelation {
	return domain.WordRelation{
		ID:          row.ID,
		FromWordID:  row.FromWordID,
		ToWordID:    row.ToWordID,
		Relation:    row.Relation,
		TargetLemma: row.TargetLemma,
		TargetPOS:   domain.PartOfSpeech(row.TargetPos),
		CreatedAt:   row.CreatedAt,
	}
}

func mapDomainDeck(row sqlc.SkillDeck, count int) domain.Deck {
	return domain.Deck{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Slug:        row.Slug,
		Name:        row.Name,
		Description: row.Description,
		IsPublic:    row.IsPublic,
		ItemCount:   count,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
