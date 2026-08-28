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
