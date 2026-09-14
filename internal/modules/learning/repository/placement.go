package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// overdueSweepBatch bounds how many overdue sessions one sweep reads.
const overdueSweepBatch int32 = 100

const constraintOneOpenPlacement = "uq_placement_sessions_one_in_progress"

// CreatePlacementSession stores a new session. A second session in progress for
// the same learner is ErrPlacementInProgress, from the partial unique index.
func (r *Repository) CreatePlacementSession(
	ctx context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	estimate, items, err := encodeSessionState(session)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreatePlacementSession(ctx, sqlc.CreatePlacementSessionParams{
		ID:         session.ID,
		UserID:     session.UserID,
		Stage:      session.Stage,
		StartedAt:  session.StartedAt,
		DeadlineAt: session.DeadlineAt,
		Estimate:   estimate,
		Items:      items,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == constraintOneOpenPlacement {
			return nil, domain.ErrPlacementInProgress
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row)
}

// GetPlacementSession returns a session by id, or nil.
func (r *Repository) GetPlacementSession(ctx context.Context, id uuid.UUID) (*domain.PlacementSession, error) {
	return optionalSession(r.queries.GetPlacementSession(ctx, id))
}

// GetOpenPlacementSession returns the learner's session in progress, or nil.
func (r *Repository) GetOpenPlacementSession(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementSession, error) {
	return optionalSession(r.queries.GetOpenPlacementSession(ctx, userID))
}

// GetLatestCompletedPlacementSession returns the learner's last completed session, or nil.
func (r *Repository) GetLatestCompletedPlacementSession(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementSession, error) {
	return optionalSession(r.queries.GetLatestCompletedPlacementSession(ctx, userID))
}

// FindPlacementSessionByAttempt returns the session that served an attempt, or nil.
func (r *Repository) FindPlacementSessionByAttempt(
	ctx context.Context, userID, attemptID uuid.UUID,
) (*domain.PlacementSession, error) {
	return optionalSession(r.queries.FindPlacementSessionByAttempt(ctx, sqlc.FindPlacementSessionByAttemptParams{
		UserID:    userID,
		AttemptID: attemptID.String(),
	}))
}

// SavePlacementProgress writes the estimate, items and stage of a session still
// in progress. It returns ErrPlacementConflict when the version moved.
func (r *Repository) SavePlacementProgress(
	ctx context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	estimate, items, err := encodeSessionState(session)
	if err != nil {
		return nil, err
	}
	return versionedSession(r.queries.SavePlacementProgress(ctx, sqlc.SavePlacementProgressParams{
		Stage:    session.Stage,
		Estimate: estimate,
		Items:    items,
		ID:       session.ID,
		Version:  clampInt32(session.Version),
	}))
}

// FinishPlacementSession closes a session in progress as completed or expired.
func (r *Repository) FinishPlacementSession(
	ctx context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	estimate, items, err := encodeSessionState(session)
	if err != nil {
		return nil, err
	}
	return versionedSession(r.queries.FinishPlacementSession(ctx, sqlc.FinishPlacementSessionParams{
		Status:      session.Status,
		Estimate:    estimate,
		Items:       items,
		ResultID:    session.ResultID,
		CompletedAt: session.CompletedAt,
		ID:          session.ID,
		Version:     clampInt32(session.Version),
	}))
}

// SavePlacementProductive writes the writing and speaking part's state.
func (r *Repository) SavePlacementProductive(
	ctx context.Context, session *domain.PlacementSession,
) (*domain.PlacementSession, error) {
	_, items, err := encodeSessionState(session)
	if err != nil {
		return nil, err
	}
	return versionedSession(r.queries.SavePlacementProductive(ctx, sqlc.SavePlacementProductiveParams{
		Items:                items,
		ProductiveStatus:     session.ProductiveStatus,
		ProductiveDeadlineAt: session.ProductiveDeadlineAt,
		ID:                   session.ID,
		Version:              clampInt32(session.Version),
	}))
}

// ListOverduePlacementSessions lists sessions in progress whose deadline is before cutoff.
func (r *Repository) ListOverduePlacementSessions(ctx context.Context, cutoff time.Time) ([]uuid.UUID, error) {
	ids, err := r.queries.ListOverduePlacementSessions(ctx, sqlc.ListOverduePlacementSessionsParams{
		Cutoff:  cutoff,
		MaxRows: overdueSweepBatch,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return ids, nil
}

// CreatePlacementResult stores a placement result for a session.
func (r *Repository) CreatePlacementResult(
	ctx context.Context, result *domain.PlacementResult,
) (*domain.PlacementResult, error) {
	perSkill, err := json.Marshal(result.PerSkill)
	if err != nil {
		return nil, fmt.Errorf("encode per_skill: %w", err)
	}
	row, err := r.queries.CreateSessionPlacementResult(ctx, sqlc.CreateSessionPlacementResultParams{
		UserID:         result.UserID,
		EstimatedLevel: result.Level,
		PerSkill:       perSkill,
		SessionID:      result.SessionID,
		TakenAt:        result.TakenAt,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainPlacementResult(row)
}

// GetPlacementResult returns a result by id, or nil.
func (r *Repository) GetPlacementResult(ctx context.Context, id uuid.UUID) (*domain.PlacementResult, error) {
	return optionalResult(r.queries.GetPlacementResult(ctx, id))
}

// GetCurrentPlacementResult returns the learner's most recent result, or nil.
func (r *Repository) GetCurrentPlacementResult(
	ctx context.Context, userID uuid.UUID,
) (*domain.PlacementResult, error) {
	return optionalResult(r.queries.GetCurrentPlacementResult(ctx, userID))
}

// UpdatePlacementResultPerSkill replaces a result's per-skill bands.
func (r *Repository) UpdatePlacementResultPerSkill(
	ctx context.Context, id uuid.UUID, perSkill map[string]domain.SkillEstimate,
) (*domain.PlacementResult, error) {
	encoded, err := json.Marshal(perSkill)
	if err != nil {
		return nil, fmt.Errorf("encode per_skill: %w", err)
	}
	row, err := r.queries.UpdatePlacementResultPerSkill(ctx, sqlc.UpdatePlacementResultPerSkillParams{
		PerSkill: encoded,
		ID:       id,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainPlacementResult(row)
}

// GetWeeklyPlan returns the learner's plan for a week, or nil.
func (r *Repository) GetWeeklyPlan(
	ctx context.Context, userID uuid.UUID, weekStart time.Time,
) (*domain.WeeklyPlan, error) {
	row, err := r.queries.GetWeeklyPlan(ctx, sqlc.GetWeeklyPlanParams{
		UserID:    userID,
		WeekStart: pgtype.Date{Time: weekStart, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainWeeklyPlan(row)
}

// CreateWeeklyPlan stores a week's plan. It returns nil when another request
// stored that week's plan first.
func (r *Repository) CreateWeeklyPlan(ctx context.Context, plan *domain.WeeklyPlan) (*domain.WeeklyPlan, error) {
	planItems := plan.Items
	if planItems == nil {
		// A nil slice encodes as null, which ck_weekly_plans_items_array refuses.
		planItems = []domain.WeeklyPlanItem{}
	}
	items, err := json.Marshal(planItems)
	if err != nil {
		return nil, fmt.Errorf("encode weekly plan items: %w", err)
	}
	row, err := r.queries.CreateWeeklyPlan(ctx, sqlc.CreateWeeklyPlanParams{
		UserID:      plan.UserID,
		WeekStart:   pgtype.Date{Time: plan.WeekStart, Valid: true},
		MinutesGoal: clampInt32(plan.MinutesGoal),
		Items:       items,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainWeeklyPlan(row)
}

// SumLearningMinutesBetween totals the learner's session minutes started in [from, to).
func (r *Repository) SumLearningMinutesBetween(
	ctx context.Context, userID uuid.UUID, from, to time.Time,
) (int, error) {
	minutes, err := r.queries.SumLearningMinutesBetween(ctx, sqlc.SumLearningMinutesBetweenParams{
		UserID: userID, FromTime: from, ToTime: to,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return int(minutes), nil
}

// CountPracticedDailySetsBetween counts the daily sets in [fromDate, toDate) with a graded attempt.
func (r *Repository) CountPracticedDailySetsBetween(
	ctx context.Context, userID uuid.UUID, fromDate, toDate, fromTime time.Time,
) (int, error) {
	count, err := r.queries.CountPracticedDailySetsBetween(ctx, sqlc.CountPracticedDailySetsBetweenParams{
		UserID:   userID,
		FromDate: pgtype.Date{Time: fromDate, Valid: true},
		ToDate:   pgtype.Date{Time: toDate, Valid: true},
		FromTime: fromTime,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return int(count), nil
}

// CountAttemptsByGradersBetween counts graded or grading attempts by grader in [from, to).
func (r *Repository) CountAttemptsByGradersBetween(
	ctx context.Context, userID uuid.UUID, graders []string, from, to time.Time,
) (int, error) {
	count, err := r.queries.CountAttemptsByGradersBetween(ctx, sqlc.CountAttemptsByGradersBetweenParams{
		UserID: userID, Graders: graders, FromTime: from, ToTime: to,
	})
	if err != nil {
		return 0, mapPgError(err)
	}
	return int(count), nil
}

func clampInt32(n int) int32 {
	return int32(max(math.MinInt32, min(math.MaxInt32, n))) //nolint:gosec // bounded on the line itself
}

func encodeSessionState(session *domain.PlacementSession) (estimate, items []byte, err error) {
	estimate, err = json.Marshal(session.Estimate)
	if err != nil {
		return nil, nil, fmt.Errorf("encode placement estimate: %w", err)
	}
	if session.Items == nil {
		session.Items = []domain.PlacementItem{}
	}
	items, err = json.Marshal(session.Items)
	if err != nil {
		return nil, nil, fmt.Errorf("encode placement items: %w", err)
	}
	return estimate, items, nil
}

func optionalSession(row sqlc.LearnPlacementSession, err error) (*domain.PlacementSession, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row)
}

// versionedSession maps a guarded update: no row means another request moved
// the session first.
func versionedSession(row sqlc.LearnPlacementSession, err error) (*domain.PlacementSession, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPlacementConflict
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementSession(row)
}

func optionalResult(row sqlc.LearnPlacementResult, err error) (*domain.PlacementResult, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toDomainPlacementResult(row)
}

func toDomainPlacementSession(row sqlc.LearnPlacementSession) (*domain.PlacementSession, error) {
	session := &domain.PlacementSession{
		ID:                   row.ID,
		UserID:               row.UserID,
		Status:               row.Status,
		Stage:                row.Stage,
		StartedAt:            row.StartedAt,
		DeadlineAt:           row.DeadlineAt,
		Version:              int(row.Version),
		ProductiveStatus:     row.ProductiveStatus,
		ProductiveDeadlineAt: row.ProductiveDeadlineAt,
		ResultID:             row.ResultID,
		CompletedAt:          row.CompletedAt,
	}
	if err := json.Unmarshal(row.Estimate, &session.Estimate); err != nil {
		return nil, fmt.Errorf("decode placement estimate of %s: %w", row.ID, err)
	}
	if err := json.Unmarshal(row.Items, &session.Items); err != nil {
		return nil, fmt.Errorf("decode placement items of %s: %w", row.ID, err)
	}
	return session, nil
}

func toDomainPlacementResult(row sqlc.LearnPlacementResult) (*domain.PlacementResult, error) {
	result := &domain.PlacementResult{
		ID:        row.ID,
		UserID:    row.UserID,
		SessionID: row.SessionID,
		Level:     row.EstimatedLevel,
		TakenAt:   row.TakenAt,
		PerSkill:  map[string]domain.SkillEstimate{},
	}
	if len(row.PerSkill) > 0 {
		if err := json.Unmarshal(row.PerSkill, &result.PerSkill); err != nil {
			return nil, fmt.Errorf("decode per_skill of %s: %w", row.ID, err)
		}
	}
	return result, nil
}

func toDomainWeeklyPlan(row sqlc.LearnWeeklyPlan) (*domain.WeeklyPlan, error) {
	plan := &domain.WeeklyPlan{
		UserID:      row.UserID,
		WeekStart:   row.WeekStart.Time,
		MinutesGoal: int(row.MinutesGoal),
		CreatedAt:   row.CreatedAt,
	}
	if err := json.Unmarshal(row.Items, &plan.Items); err != nil {
		return nil, fmt.Errorf("decode weekly plan items: %w", err)
	}
	return plan, nil
}
