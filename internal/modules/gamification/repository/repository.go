// Package repository implements database persistence for the gamification module.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fluentra/fluentra/internal/generated/gamification/sqlc"
)

// Repository is the data access surface the service depends on.
//
// An interface rather than the concrete sqlc.Queries so the service can be unit
// tested without a database. It is deliberately a thin pass-through: the rules
// live in domain/, and a repository that starts deciding things is a repository
// that cannot be reasoned about from the SQL.
type Repository interface {
	// XP.
	AwardXP(ctx context.Context, arg sqlc.AwardXPParams) (sqlc.LearnXpEvent, error)
	GetActivityHighWater(ctx context.Context, userID uuid.UUID, activityID string) (sqlc.GetActivityHighWaterRow, error)
	UpsertActivityHighWater(
		ctx context.Context, arg sqlc.UpsertActivityHighWaterParams,
	) (sqlc.LearnXpActivityHighWater, error)
	TotalXP(ctx context.Context, userID uuid.UUID) (int64, error)
	XPSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
	XPFromSourceSince(ctx context.Context, userID uuid.UUID, source string, since time.Time) (int64, error)
	CountAwardsFromSourceSince(
		ctx context.Context, userID uuid.UUID, source string, since time.Time,
	) (int64, error)
	WeeklyXPStandings(ctx context.Context, from, until time.Time, limit int32) ([]sqlc.WeeklyXPStandingsRow, error)

	// Streaks.
	GetStreak(ctx context.Context, userID uuid.UUID) (sqlc.LearnStreak, error)
	EnsureStreak(ctx context.Context, userID uuid.UUID) (sqlc.LearnStreak, error)
	ExtendStreak(ctx context.Context, userID uuid.UUID, newLength int32, day time.Time) (sqlc.LearnStreak, error)
	BreakStreak(ctx context.Context, userID uuid.UUID) (sqlc.LearnStreak, error)
	ConsumeFreeze(ctx context.Context, userID uuid.UUID, day time.Time) (sqlc.LearnStreak, error)
	SetDailyGoal(ctx context.Context, userID uuid.UUID, goal int32) (sqlc.LearnStreak, error)
	SetLeaderboardOptIn(ctx context.Context, userID uuid.UUID, optIn bool) (sqlc.LearnStreak, error)
	ListStreaksAtRisk(ctx context.Context, before time.Time, limit int32) ([]sqlc.LearnStreak, error)

	// Badges.
	ListUnearnedBadges(ctx context.Context, userID uuid.UUID) ([]sqlc.LearnBadge, error)
	ListEarnedBadges(ctx context.Context, userID uuid.UUID) ([]sqlc.ListEarnedBadgesRow, error)
	AwardBadge(ctx context.Context, userID, badgeID uuid.UUID) (sqlc.LearnBadgesEarned, error)
	UpsertBadge(ctx context.Context, arg sqlc.UpsertBadgeParams) (sqlc.LearnBadge, error)

	// Quests.
	ListActiveQuests(ctx context.Context) ([]sqlc.LearnQuest, error)
	UpsertQuest(ctx context.Context, arg sqlc.UpsertQuestParams) (sqlc.LearnQuest, error)
	StartUserQuest(ctx context.Context, userID, questID uuid.UUID, start, expires time.Time) (sqlc.LearnUserQuest, error)
	ListOpenUserQuests(ctx context.Context, userID uuid.UUID, on time.Time) ([]sqlc.ListOpenUserQuestsRow, error)
	UpdateQuestProgress(ctx context.Context, id, userID uuid.UUID, progress []byte) (sqlc.LearnUserQuest, error)
	CompleteUserQuest(ctx context.Context, id, userID uuid.UUID) (sqlc.LearnUserQuest, error)

	// Leaderboard.
	UpsertLeaderboardEntry(
		ctx context.Context, arg sqlc.UpsertLeaderboardEntryParams,
	) (sqlc.LearnLeaderboardSnapshot, error)
	ListLeaderboard(
		ctx context.Context, league string, weekStart time.Time, limit int32,
	) ([]sqlc.LearnLeaderboardSnapshot, error)
	GetLeaderboardEntry(ctx context.Context, userID uuid.UUID, weekStart time.Time) (sqlc.LearnLeaderboardSnapshot, error)
	DeleteLeaderboardBefore(ctx context.Context, weekStart time.Time) error
}

type pgxRepository struct{ q *sqlc.Queries }

// New builds a repository over a pool or a transaction. A nil db yields a
// repository whose every call returns ErrNoPool, which is what cmd/worker gets
// when it constructs the module without a database — never a nil dereference.
func New(db sqlc.DBTX) Repository {
	if db == nil {
		return &pgxRepository{q: nil}
	}
	return &pgxRepository{q: sqlc.New(db)}
}

// Date converts a day to the pgtype.Date the generated code takes. Exported
// because the service builds query arguments in a couple of places where going
// through the repository would mean a method per shape.
func Date(day time.Time) pgtype.Date {
	return pgtype.Date{Time: day, Valid: true}
}

// DayOf reads a nullable date column back, returning the zero time when NULL.
func DayOf(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

func (r *pgxRepository) AwardXP(ctx context.Context, arg sqlc.AwardXPParams) (sqlc.LearnXpEvent, error) {
	if r.q == nil {
		return sqlc.LearnXpEvent{}, ErrNoPool
	}
	return r.q.AwardXP(ctx, arg)
}

func (r *pgxRepository) GetActivityHighWater(
	ctx context.Context, userID uuid.UUID, activityID string,
) (sqlc.GetActivityHighWaterRow, error) {
	if r.q == nil {
		return sqlc.GetActivityHighWaterRow{}, ErrNoPool
	}
	return r.q.GetActivityHighWater(ctx, sqlc.GetActivityHighWaterParams{
		UserID:     userID,
		ActivityID: activityID,
	})
}

func (r *pgxRepository) UpsertActivityHighWater(
	ctx context.Context, arg sqlc.UpsertActivityHighWaterParams,
) (sqlc.LearnXpActivityHighWater, error) {
	if r.q == nil {
		return sqlc.LearnXpActivityHighWater{}, ErrNoPool
	}
	return r.q.UpsertActivityHighWater(ctx, arg)
}

func (r *pgxRepository) TotalXP(ctx context.Context, userID uuid.UUID) (int64, error) {
	if r.q == nil {
		return 0, ErrNoPool
	}
	return r.q.TotalXP(ctx, userID)
}

func (r *pgxRepository) XPSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	if r.q == nil {
		return 0, ErrNoPool
	}
	return r.q.XPSince(ctx, sqlc.XPSinceParams{UserID: userID, AwardedAt: since})
}

func (r *pgxRepository) XPFromSourceSince(
	ctx context.Context, userID uuid.UUID, source string, since time.Time,
) (int64, error) {
	if r.q == nil {
		return 0, ErrNoPool
	}
	return r.q.XPFromSourceSince(ctx, sqlc.XPFromSourceSinceParams{
		UserID: userID, Source: source, AwardedAt: since,
	})
}

func (r *pgxRepository) CountAwardsFromSourceSince(
	ctx context.Context, userID uuid.UUID, source string, since time.Time,
) (int64, error) {
	if r.q == nil {
		return 0, ErrNoPool
	}
	return r.q.CountAwardsFromSourceSince(ctx, sqlc.CountAwardsFromSourceSinceParams{
		UserID: userID, Source: source, AwardedAt: since,
	})
}

func (r *pgxRepository) WeeklyXPStandings(
	ctx context.Context, from, until time.Time, limit int32,
) ([]sqlc.WeeklyXPStandingsRow, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.WeeklyXPStandings(ctx, sqlc.WeeklyXPStandingsParams{
		AwardedAt: from, AwardedAt_2: until, Limit: limit,
	})
}

func (r *pgxRepository) GetStreak(ctx context.Context, userID uuid.UUID) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.GetStreak(ctx, userID)
}

func (r *pgxRepository) EnsureStreak(ctx context.Context, userID uuid.UUID) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.EnsureStreak(ctx, userID)
}

func (r *pgxRepository) ExtendStreak(
	ctx context.Context, userID uuid.UUID, newLength int32, day time.Time,
) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.ExtendStreak(ctx, sqlc.ExtendStreakParams{
		UserID: userID, CurrentLength: newLength, LastActiveOn: Date(day),
	})
}

func (r *pgxRepository) BreakStreak(ctx context.Context, userID uuid.UUID) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.BreakStreak(ctx, userID)
}

func (r *pgxRepository) ConsumeFreeze(
	ctx context.Context, userID uuid.UUID, day time.Time,
) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.ConsumeFreeze(ctx, sqlc.ConsumeFreezeParams{UserID: userID, FreezeUsedOn: Date(day)})
}

func (r *pgxRepository) SetDailyGoal(
	ctx context.Context, userID uuid.UUID, goal int32,
) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.SetDailyGoal(ctx, sqlc.SetDailyGoalParams{UserID: userID, DailyGoalXp: goal})
}

func (r *pgxRepository) SetLeaderboardOptIn(
	ctx context.Context, userID uuid.UUID, optIn bool,
) (sqlc.LearnStreak, error) {
	if r.q == nil {
		return sqlc.LearnStreak{}, ErrNoPool
	}
	return r.q.SetLeaderboardOptIn(ctx, sqlc.SetLeaderboardOptInParams{
		UserID: userID, LeaderboardOptIn: optIn,
	})
}

func (r *pgxRepository) ListStreaksAtRisk(
	ctx context.Context, before time.Time, limit int32,
) ([]sqlc.LearnStreak, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.ListStreaksAtRisk(ctx, sqlc.ListStreaksAtRiskParams{
		LastActiveOn: Date(before), Limit: limit,
	})
}

func (r *pgxRepository) ListUnearnedBadges(
	ctx context.Context, userID uuid.UUID,
) ([]sqlc.LearnBadge, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.ListUnearnedBadges(ctx, userID)
}

func (r *pgxRepository) ListEarnedBadges(
	ctx context.Context, userID uuid.UUID,
) ([]sqlc.ListEarnedBadgesRow, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.ListEarnedBadges(ctx, userID)
}

func (r *pgxRepository) AwardBadge(
	ctx context.Context, userID, badgeID uuid.UUID,
) (sqlc.LearnBadgesEarned, error) {
	if r.q == nil {
		return sqlc.LearnBadgesEarned{}, ErrNoPool
	}
	return r.q.AwardBadge(ctx, sqlc.AwardBadgeParams{UserID: userID, BadgeID: badgeID})
}

func (r *pgxRepository) UpsertBadge(
	ctx context.Context, arg sqlc.UpsertBadgeParams,
) (sqlc.LearnBadge, error) {
	if r.q == nil {
		return sqlc.LearnBadge{}, ErrNoPool
	}
	return r.q.UpsertBadge(ctx, arg)
}

func (r *pgxRepository) ListActiveQuests(ctx context.Context) ([]sqlc.LearnQuest, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.ListActiveQuests(ctx)
}

func (r *pgxRepository) UpsertQuest(
	ctx context.Context, arg sqlc.UpsertQuestParams,
) (sqlc.LearnQuest, error) {
	if r.q == nil {
		return sqlc.LearnQuest{}, ErrNoPool
	}
	return r.q.UpsertQuest(ctx, arg)
}

func (r *pgxRepository) StartUserQuest(
	ctx context.Context, userID, questID uuid.UUID, start, expires time.Time,
) (sqlc.LearnUserQuest, error) {
	if r.q == nil {
		return sqlc.LearnUserQuest{}, ErrNoPool
	}
	return r.q.StartUserQuest(ctx, sqlc.StartUserQuestParams{
		UserID: userID, QuestID: questID, StartedOn: Date(start), ExpiresOn: Date(expires),
	})
}

func (r *pgxRepository) ListOpenUserQuests(
	ctx context.Context, userID uuid.UUID, on time.Time,
) ([]sqlc.ListOpenUserQuestsRow, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.ListOpenUserQuests(ctx, sqlc.ListOpenUserQuestsParams{
		UserID: userID, ExpiresOn: Date(on),
	})
}

func (r *pgxRepository) UpdateQuestProgress(
	ctx context.Context, id, userID uuid.UUID, progress []byte,
) (sqlc.LearnUserQuest, error) {
	if r.q == nil {
		return sqlc.LearnUserQuest{}, ErrNoPool
	}
	return r.q.UpdateQuestProgress(ctx, sqlc.UpdateQuestProgressParams{
		ID: id, UserID: userID, Progress: progress,
	})
}

func (r *pgxRepository) CompleteUserQuest(
	ctx context.Context, id, userID uuid.UUID,
) (sqlc.LearnUserQuest, error) {
	if r.q == nil {
		return sqlc.LearnUserQuest{}, ErrNoPool
	}
	return r.q.CompleteUserQuest(ctx, sqlc.CompleteUserQuestParams{ID: id, UserID: userID})
}

func (r *pgxRepository) UpsertLeaderboardEntry(
	ctx context.Context, arg sqlc.UpsertLeaderboardEntryParams,
) (sqlc.LearnLeaderboardSnapshot, error) {
	if r.q == nil {
		return sqlc.LearnLeaderboardSnapshot{}, ErrNoPool
	}
	return r.q.UpsertLeaderboardEntry(ctx, arg)
}

func (r *pgxRepository) ListLeaderboard(
	ctx context.Context, league string, weekStart time.Time, limit int32,
) ([]sqlc.LearnLeaderboardSnapshot, error) {
	if r.q == nil {
		return nil, ErrNoPool
	}
	return r.q.ListLeaderboard(ctx, sqlc.ListLeaderboardParams{
		League: league, WeekStart: Date(weekStart), Limit: limit,
	})
}

func (r *pgxRepository) GetLeaderboardEntry(
	ctx context.Context, userID uuid.UUID, weekStart time.Time,
) (sqlc.LearnLeaderboardSnapshot, error) {
	if r.q == nil {
		return sqlc.LearnLeaderboardSnapshot{}, ErrNoPool
	}
	return r.q.GetLeaderboardEntry(ctx, sqlc.GetLeaderboardEntryParams{
		UserID: userID, WeekStart: Date(weekStart),
	})
}

func (r *pgxRepository) DeleteLeaderboardBefore(ctx context.Context, weekStart time.Time) error {
	if r.q == nil {
		return ErrNoPool
	}
	return r.q.DeleteLeaderboardBefore(ctx, Date(weekStart))
}
