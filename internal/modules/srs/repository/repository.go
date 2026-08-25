// Package repository implements database persistence for the srs module.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/generated/srs/sqlc"
)

// Repository encapsulates database access for srs.
type Repository interface {
	UpsertReviewCard(ctx context.Context, arg sqlc.UpsertReviewCardParams) (sqlc.LearnReviewCard, error)
	GetReviewCardByID(ctx context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error)
	GetReviewCardByUserAndContent(
		ctx context.Context, userID, contentVersionID uuid.UUID,
	) (sqlc.LearnReviewCard, error)
	ListDueCards(
		ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int32,
	) ([]sqlc.LearnReviewCard, error)
	CountDueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int64, error)
	ForecastDueCards(
		ctx context.Context, userID uuid.UUID, timezone string, until time.Time,
	) ([]sqlc.ForecastDueCardsRow, error)
	UpdateReviewCardSchedule(ctx context.Context, arg sqlc.UpdateReviewCardScheduleParams) (sqlc.LearnReviewCard, error)
	SuspendReviewCard(ctx context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error)
	ResetReviewCard(ctx context.Context, arg sqlc.ResetReviewCardParams) (sqlc.LearnReviewCard, error)
	SetReviewCardsSuspended(
		ctx context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool,
	) (int64, error)
	InsertReviewLog(ctx context.Context, arg sqlc.InsertReviewLogParams) (sqlc.LearnReviewLog, error)
	ListReviewLogsByCard(
		ctx context.Context, cardID, userID uuid.UUID, limit int32,
	) ([]sqlc.LearnReviewLog, error)
	SumRecentReviewElapsedMs(ctx context.Context, userID uuid.UUID, since time.Time, limit int32) (int64, error)
	UpsertReviewDailyStats(
		ctx context.Context, arg sqlc.UpsertReviewDailyStatsParams,
	) (sqlc.LearnReviewDailyStat, error)
	WithTx(tx pgx.Tx) Repository
}

type pgxRepository struct {
	q *sqlc.Queries
}

// New constructs a new PostgreSQL repository for srs.
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

func (r *pgxRepository) UpsertReviewCard(
	ctx context.Context, arg sqlc.UpsertReviewCardParams,
) (sqlc.LearnReviewCard, error) {
	return r.q.UpsertReviewCard(ctx, arg)
}

func (r *pgxRepository) GetReviewCardByID(ctx context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error) {
	return r.q.GetReviewCardByID(ctx, sqlc.GetReviewCardByIDParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *pgxRepository) GetReviewCardByUserAndContent(
	ctx context.Context, userID, contentVersionID uuid.UUID,
) (sqlc.LearnReviewCard, error) {
	return r.q.GetReviewCardByUserAndContent(ctx, sqlc.GetReviewCardByUserAndContentParams{
		UserID:           userID,
		ContentVersionID: contentVersionID,
	})
}

func (r *pgxRepository) ListDueCards(
	ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int32,
) ([]sqlc.LearnReviewCard, error) {
	return r.q.ListDueCards(ctx, sqlc.ListDueCardsParams{
		UserID: userID,
		DueAt:  dueBefore,
		Limit:  limit,
	})
}

func (r *pgxRepository) CountDueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int64, error) {
	return r.q.CountDueCards(ctx, sqlc.CountDueCardsParams{
		UserID: userID,
		DueAt:  dueBefore,
	})
}

func (r *pgxRepository) ForecastDueCards(
	ctx context.Context, userID uuid.UUID, timezone string, until time.Time,
) ([]sqlc.ForecastDueCardsRow, error) {
	return r.q.ForecastDueCards(ctx, sqlc.ForecastDueCardsParams{
		Timezone: timezone,
		UserID:   userID,
		Until:    until,
	})
}

func (r *pgxRepository) UpdateReviewCardSchedule(
	ctx context.Context, arg sqlc.UpdateReviewCardScheduleParams,
) (sqlc.LearnReviewCard, error) {
	return r.q.UpdateReviewCardSchedule(ctx, arg)
}

func (r *pgxRepository) SuspendReviewCard(ctx context.Context, id, userID uuid.UUID) (sqlc.LearnReviewCard, error) {
	return r.q.SuspendReviewCard(ctx, sqlc.SuspendReviewCardParams{
		ID:     id,
		UserID: userID,
	})
}

func (r *pgxRepository) SetReviewCardsSuspended(
	ctx context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool,
) (int64, error) {
	return r.q.SetReviewCardsSuspended(ctx, sqlc.SetReviewCardsSuspendedParams{
		Suspended:         suspended,
		UserID:            userID,
		ContentVersionIds: contentVersionIDs,
	})
}

func (r *pgxRepository) ResetReviewCard(
	ctx context.Context, arg sqlc.ResetReviewCardParams) (sqlc.LearnReviewCard, error,
) {
	return r.q.ResetReviewCard(ctx, arg)
}

func (r *pgxRepository) InsertReviewLog(
	ctx context.Context, arg sqlc.InsertReviewLogParams,
) (sqlc.LearnReviewLog, error) {
	return r.q.InsertReviewLog(ctx, arg)
}

func (r *pgxRepository) ListReviewLogsByCard(
	ctx context.Context, cardID, userID uuid.UUID, limit int32,
) ([]sqlc.LearnReviewLog, error) {
	return r.q.ListReviewLogsByCard(ctx, sqlc.ListReviewLogsByCardParams{
		CardID: cardID,
		UserID: userID,
		Limit:  limit,
	})
}

func (r *pgxRepository) SumRecentReviewElapsedMs(
	ctx context.Context, userID uuid.UUID, since time.Time, limit int32,
) (int64, error) {
	return r.q.SumRecentReviewElapsedMs(ctx, sqlc.SumRecentReviewElapsedMsParams{
		UserID:     userID,
		ReviewedAt: since,
		Limit:      limit,
	})
}

func (r *pgxRepository) UpsertReviewDailyStats(
	ctx context.Context, arg sqlc.UpsertReviewDailyStatsParams,
) (sqlc.LearnReviewDailyStat, error) {
	return r.q.UpsertReviewDailyStats(ctx, arg)
}
