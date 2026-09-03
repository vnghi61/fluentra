// Package repository implements database persistence for vocabulary.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
)

// Repository encapsulates database operations for vocabulary.
type Repository interface {
	InsertWord(ctx context.Context, arg sqlc.InsertWordParams) (sqlc.SkillWord, error)
	GetWordByLemmaAndPOS(ctx context.Context, lemma, pos string) (sqlc.SkillWord, error)
	GetWordByID(ctx context.Context, id uuid.UUID) (sqlc.SkillWord, error)
	ListWordsByLemma(ctx context.Context, lemma string) ([]sqlc.SkillWord, error)
	SearchWords(ctx context.Context, lemma string, limit, offset int32) ([]sqlc.SkillWord, error)
	CountSearchWords(ctx context.Context, lemma string) (int64, error)
	InsertWordSense(ctx context.Context, arg sqlc.InsertWordSenseParams) (sqlc.SkillWordSense, error)
	ListSensesByWordID(ctx context.Context, wordID uuid.UUID) ([]sqlc.SkillWordSense, error)
	GetSenseByID(ctx context.Context, id uuid.UUID) (sqlc.GetSenseByIDRow, error)
	GetSenseContentVersionByLemma(ctx context.Context, lemma string) (*uuid.UUID, error)
	ListSensesByIDs(ctx context.Context, ids []uuid.UUID) ([]sqlc.ListSensesByIDsRow, error)
	InsertWordRelation(ctx context.Context, arg sqlc.InsertWordRelationParams) (sqlc.SkillWordRelation, error)
	ListRelationsByWordID(ctx context.Context, wordID uuid.UUID) ([]sqlc.ListRelationsByWordIDRow, error)
	UpsertUserWordState(ctx context.Context, arg sqlc.UpsertUserWordStateParams) (sqlc.SkillUserWordState, error)
	GetUserWordState(ctx context.Context, userID, wordSenseID uuid.UUID) (sqlc.SkillUserWordState, error)
	InsertDeck(ctx context.Context, arg sqlc.InsertDeckParams) (sqlc.SkillDeck, error)
	GetDeckByID(ctx context.Context, id uuid.UUID) (sqlc.SkillDeck, error)
	ListDecksByUser(ctx context.Context, userID *uuid.UUID) ([]sqlc.ListDecksByUserRow, error)
	InsertDeckItem(ctx context.Context, arg sqlc.InsertDeckItemParams) (sqlc.SkillDeckItem, error)
	DeleteDeckItem(ctx context.Context, deckID, wordSenseID uuid.UUID) error
	ListDeckWords(ctx context.Context, deckID uuid.UUID, limit, offset int32) ([]sqlc.ListDeckWordsRow, error)
	// ListSensesForGeneration feeds the practice generator. Bounded by `limit`
	// because it runs in a scheduled job, and an unbounded scan is how a
	// background job becomes an outage as the dictionary grows.
	ListSensesForGeneration(ctx context.Context, limit int32) ([]sqlc.ListSensesForGenerationRow, error)

	// Learner uploads.
	InsertUpload(ctx context.Context, arg sqlc.InsertUploadParams) (sqlc.SkillVocabUpload, error)
	InsertUploadItem(ctx context.Context, arg sqlc.InsertUploadItemParams) (sqlc.SkillVocabUploadItem, error)
	GetUpload(ctx context.Context, id, userID uuid.UUID) (sqlc.SkillVocabUpload, error)
	ListUploadsByUser(ctx context.Context, userID uuid.UUID, limit int32) ([]sqlc.ListUploadsByUserRow, error)
	ListUploadItems(ctx context.Context, uploadID, userID uuid.UUID) ([]sqlc.SkillVocabUploadItem, error)
	ClaimPendingUploadItems(ctx context.Context, maxAttempts, limit int32) ([]sqlc.SkillVocabUploadItem, error)
	MarkUploadItemVerified(
		ctx context.Context, id uuid.UUID, senseID *uuid.UUID, model, reason string,
	) (sqlc.SkillVocabUploadItem, error)
	MarkUploadItemRejected(ctx context.Context, id uuid.UUID, reason string) (sqlc.SkillVocabUploadItem, error)
	RecordUploadItemAttempt(ctx context.Context, id uuid.UUID, reason string) error
	SetUploadDeck(ctx context.Context, uploadID, deckID uuid.UUID) error
	UpdateWordSenseEnrichment(
		ctx context.Context, arg sqlc.UpdateWordSenseEnrichmentParams,
	) (sqlc.SkillWordSense, error)
	MarkUploadItemQueued(
		ctx context.Context, id uuid.UUID, senseID *uuid.UUID, reason string,
	) (sqlc.SkillVocabUploadItem, error)
	ClaimQueuedUploadItems(
		ctx context.Context, maxAttempts, limit int32,
	) ([]sqlc.SkillVocabUploadItem, error)
	MarkQueuedUploadItemVerified(
		ctx context.Context, id uuid.UUID, model, reason string,
	) (sqlc.SkillVocabUploadItem, error)
	MarkQueuedUploadItemRejected(
		ctx context.Context, id uuid.UUID, reason string,
	) (sqlc.SkillVocabUploadItem, error)
	MarkQueuedUploadItemFailed(
		ctx context.Context, id uuid.UUID, reason string,
	) (sqlc.SkillVocabUploadItem, error)
	CompleteFinishedUploads(ctx context.Context) ([]sqlc.SkillVocabUpload, error)

	WithTx(tx pgx.Tx) Repository
}

type pgxRepository struct {
	q *sqlc.Queries
}

// New creates a new PostgreSQL repository for vocabulary.
func New(db sqlc.DBTX) Repository {
	if db == nil {
		return &pgxRepository{q: nil}
	}
	return &pgxRepository{
		q: sqlc.New(db),
	}
}

func (r *pgxRepository) WithTx(tx pgx.Tx) Repository {
	if r.q == nil {
		return r
	}
	return &pgxRepository{
		q: r.q.WithTx(tx),
	}
}

func (r *pgxRepository) InsertWord(ctx context.Context, arg sqlc.InsertWordParams) (sqlc.SkillWord, error) {
	return r.q.InsertWord(ctx, arg)
}

func (r *pgxRepository) GetWordByLemmaAndPOS(ctx context.Context, lemma, pos string) (sqlc.SkillWord, error) {
	return r.q.GetWordByLemmaAndPOS(ctx, sqlc.GetWordByLemmaAndPOSParams{
		Lemma: lemma,
		Pos:   pos,
	})
}

func (r *pgxRepository) GetWordByID(ctx context.Context, id uuid.UUID) (sqlc.SkillWord, error) {
	return r.q.GetWordByID(ctx, id)
}

func (r *pgxRepository) ListWordsByLemma(ctx context.Context, lemma string) ([]sqlc.SkillWord, error) {
	return r.q.ListWordsByLemma(ctx, lemma)
}

func (r *pgxRepository) SearchWords(ctx context.Context, lemma string, limit, offset int32) ([]sqlc.SkillWord, error) {
	return r.q.SearchWords(ctx, sqlc.SearchWordsParams{
		Column1: &lemma,
		Limit:   limit,
		Offset:  offset,
	})
}

func (r *pgxRepository) CountSearchWords(ctx context.Context, lemma string) (int64, error) {
	return r.q.CountSearchWords(ctx, &lemma)
}

func (r *pgxRepository) InsertWordSense(
	ctx context.Context, arg sqlc.InsertWordSenseParams) (sqlc.SkillWordSense, error,
) {
	return r.q.InsertWordSense(ctx, arg)
}

func (r *pgxRepository) ListSensesByWordID(ctx context.Context, wordID uuid.UUID) ([]sqlc.SkillWordSense, error) {
	return r.q.ListSensesByWordID(ctx, wordID)
}

func (r *pgxRepository) GetSenseByID(ctx context.Context, id uuid.UUID) (sqlc.GetSenseByIDRow, error) {
	return r.q.GetSenseByID(ctx, id)
}

func (r *pgxRepository) GetSenseContentVersionByLemma(
	ctx context.Context, lemma string,
) (*uuid.UUID, error) {
	return r.q.GetSenseContentVersionByLemma(ctx, lemma)
}

func (r *pgxRepository) ListSensesByIDs(ctx context.Context, ids []uuid.UUID) ([]sqlc.ListSensesByIDsRow, error) {
	return r.q.ListSensesByIDs(ctx, ids)
}

func (r *pgxRepository) InsertWordRelation(
	ctx context.Context, arg sqlc.InsertWordRelationParams) (sqlc.SkillWordRelation, error,
) {
	return r.q.InsertWordRelation(ctx, arg)
}

func (r *pgxRepository) ListRelationsByWordID(
	ctx context.Context, wordID uuid.UUID) ([]sqlc.ListRelationsByWordIDRow, error,
) {
	return r.q.ListRelationsByWordID(ctx, wordID)
}

func (r *pgxRepository) UpsertUserWordState(
	ctx context.Context, arg sqlc.UpsertUserWordStateParams) (sqlc.SkillUserWordState, error,
) {
	return r.q.UpsertUserWordState(ctx, arg)
}

func (r *pgxRepository) GetUserWordState(
	ctx context.Context, userID, wordSenseID uuid.UUID) (sqlc.SkillUserWordState, error,
) {
	return r.q.GetUserWordState(ctx, sqlc.GetUserWordStateParams{
		UserID:      userID,
		WordSenseID: wordSenseID,
	})
}

func (r *pgxRepository) InsertDeck(ctx context.Context, arg sqlc.InsertDeckParams) (sqlc.SkillDeck, error) {
	return r.q.InsertDeck(ctx, arg)
}

func (r *pgxRepository) GetDeckByID(ctx context.Context, id uuid.UUID) (sqlc.SkillDeck, error) {
	return r.q.GetDeckByID(ctx, id)
}

func (r *pgxRepository) ListDecksByUser(ctx context.Context, userID *uuid.UUID) ([]sqlc.ListDecksByUserRow, error) {
	return r.q.ListDecksByUser(ctx, userID)
}

func (r *pgxRepository) InsertDeckItem(ctx context.Context, arg sqlc.InsertDeckItemParams) (sqlc.SkillDeckItem, error) {
	return r.q.InsertDeckItem(ctx, arg)
}

func (r *pgxRepository) DeleteDeckItem(ctx context.Context, deckID, wordSenseID uuid.UUID) error {
	return r.q.DeleteDeckItem(ctx, sqlc.DeleteDeckItemParams{
		DeckID:      deckID,
		WordSenseID: wordSenseID,
	})
}

func (r *pgxRepository) ListDeckWords(
	ctx context.Context, deckID uuid.UUID, limit, offset int32,
) ([]sqlc.ListDeckWordsRow, error) {
	return r.q.ListDeckWords(ctx, sqlc.ListDeckWordsParams{
		DeckID: deckID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *pgxRepository) ListSensesForGeneration(
	ctx context.Context, limit int32,
) ([]sqlc.ListSensesForGenerationRow, error) {
	return r.q.ListSensesForGeneration(ctx, limit)
}

func (r *pgxRepository) InsertUpload(
	ctx context.Context, arg sqlc.InsertUploadParams,
) (sqlc.SkillVocabUpload, error) {
	return r.q.InsertUpload(ctx, arg)
}

func (r *pgxRepository) InsertUploadItem(
	ctx context.Context, arg sqlc.InsertUploadItemParams,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.InsertUploadItem(ctx, arg)
}

func (r *pgxRepository) GetUpload(
	ctx context.Context, id, userID uuid.UUID,
) (sqlc.SkillVocabUpload, error) {
	return r.q.GetUpload(ctx, sqlc.GetUploadParams{ID: id, UserID: userID})
}

func (r *pgxRepository) ListUploadsByUser(
	ctx context.Context, userID uuid.UUID, limit int32,
) ([]sqlc.ListUploadsByUserRow, error) {
	return r.q.ListUploadsByUser(ctx, sqlc.ListUploadsByUserParams{UserID: userID, Limit: limit})
}

func (r *pgxRepository) ListUploadItems(
	ctx context.Context, uploadID, userID uuid.UUID,
) ([]sqlc.SkillVocabUploadItem, error) {
	return r.q.ListUploadItems(ctx, sqlc.ListUploadItemsParams{UploadID: uploadID, UserID: userID})
}

func (r *pgxRepository) ClaimPendingUploadItems(
	ctx context.Context, maxAttempts, limit int32,
) ([]sqlc.SkillVocabUploadItem, error) {
	return r.q.ClaimPendingUploadItems(ctx, sqlc.ClaimPendingUploadItemsParams{
		Attempts: maxAttempts, Limit: limit,
	})
}

func (r *pgxRepository) MarkUploadItemVerified(
	ctx context.Context, id uuid.UUID, senseID *uuid.UUID, model, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.MarkUploadItemVerified(ctx, sqlc.MarkUploadItemVerifiedParams{
		ID: id, WordSenseID: senseID, VerifiedByModel: model, Reason: reason,
	})
}

func (r *pgxRepository) MarkUploadItemRejected(
	ctx context.Context, id uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.MarkUploadItemRejected(ctx, sqlc.MarkUploadItemRejectedParams{ID: id, Reason: reason})
}

func (r *pgxRepository) RecordUploadItemAttempt(
	ctx context.Context, id uuid.UUID, reason string,
) error {
	return r.q.RecordUploadItemAttempt(ctx, sqlc.RecordUploadItemAttemptParams{ID: id, Reason: reason})
}

func (r *pgxRepository) SetUploadDeck(ctx context.Context, uploadID, deckID uuid.UUID) error {
	return r.q.SetUploadDeck(ctx, sqlc.SetUploadDeckParams{ID: uploadID, DeckID: &deckID})
}

func (r *pgxRepository) CompleteFinishedUploads(
	ctx context.Context,
) ([]sqlc.SkillVocabUpload, error) {
	return r.q.CompleteFinishedUploads(ctx)
}

func (r *pgxRepository) UpdateWordSenseEnrichment(
	ctx context.Context, arg sqlc.UpdateWordSenseEnrichmentParams,
) (sqlc.SkillWordSense, error) {
	return r.q.UpdateWordSenseEnrichment(ctx, arg)
}

func (r *pgxRepository) MarkUploadItemQueued(
	ctx context.Context, id uuid.UUID, senseID *uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.MarkUploadItemQueued(ctx, sqlc.MarkUploadItemQueuedParams{
		ID:          id,
		WordSenseID: senseID,
		Reason:      reason,
	})
}

func (r *pgxRepository) ClaimQueuedUploadItems(
	ctx context.Context, maxAttempts, limit int32,
) ([]sqlc.SkillVocabUploadItem, error) {
	return r.q.ClaimQueuedUploadItems(ctx, sqlc.ClaimQueuedUploadItemsParams{
		Attempts: maxAttempts,
		Limit:    limit,
	})
}

func (r *pgxRepository) MarkQueuedUploadItemVerified(
	ctx context.Context, id uuid.UUID, model, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.MarkQueuedUploadItemVerified(ctx, sqlc.MarkQueuedUploadItemVerifiedParams{
		ID:              id,
		VerifiedByModel: model,
		Reason:          reason,
	})
}

func (r *pgxRepository) MarkQueuedUploadItemRejected(
	ctx context.Context, id uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.MarkQueuedUploadItemRejected(ctx, sqlc.MarkQueuedUploadItemRejectedParams{
		ID:     id,
		Reason: reason,
	})
}

func (r *pgxRepository) MarkQueuedUploadItemFailed(
	ctx context.Context, id uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	return r.q.MarkQueuedUploadItemFailed(ctx, sqlc.MarkQueuedUploadItemFailedParams{
		ID:     id,
		Reason: reason,
	})
}
