// Package repository handles database access for the learning module.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// CreateAttemptParams contains fields to create an in-progress attempt.
type CreateAttemptParams struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	UserID         uuid.UUID
	ActivityID     uuid.UUID
	IdempotencyKey *uuid.UUID
	Response       json.RawMessage
	Score          *int32
	MaxScore       int32
	Grader         *string
	DurationMs     int32
	Status         string
}

// ClaimAttemptParams holds inputs for atomically claiming an attempt for grading.
type ClaimAttemptParams struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	IdempotencyKey *uuid.UUID
	Response       json.RawMessage
}

// UpdateAttemptStatusParams holds updates to apply when grading is complete.
type UpdateAttemptStatusParams struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	Status     string
	Score      *int32
	Grader     *string
	DurationMs int32
}

// ProgressDTO models progress state for a scope.
type ProgressDTO struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Scope       string     `json:"scope"`
	ScopeID     uuid.UUID  `json:"scope_id"`
	Status      string     `json:"status"`
	Score       *float64   `json:"score,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// UpsertProgressParams holds fields to create or update progress.
type UpsertProgressParams struct {
	UserID      uuid.UUID
	Scope       string
	ScopeID     uuid.UUID
	Status      string
	Score       *int32
	CompletedAt *time.Time
}

// Repository handles database operations for learning.
type Repository struct {
	queries *sqlc.Queries
}

// New creates a new learning Repository over db.
func New(db dbx.Querier) *Repository {
	if db == nil {
		return &Repository{queries: nil}
	}
	return &Repository{
		queries: sqlc.New(db),
	}
}

// WithTx returns a Repository operating within tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		queries: r.queries.WithTx(tx),
	}
}

// CreateAttempt inserts a new attempt row.
func (r *Repository) CreateAttempt(ctx context.Context, params CreateAttemptParams) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, nil
	}
	var respBytes []byte
	if len(params.Response) > 0 {
		respBytes = []byte(params.Response)
	}

	row, err := r.queries.CreateAttempt(ctx, sqlc.CreateAttemptParams{
		ID:             params.ID,
		CreatedAt:      params.CreatedAt,
		UserID:         params.UserID,
		ActivityID:     params.ActivityID,
		IdempotencyKey: params.IdempotencyKey,
		Response:       respBytes,
		Score:          params.Score,
		MaxScore:       params.MaxScore,
		Grader:         params.Grader,
		DurationMs:     params.DurationMs,
		Status:         params.Status,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toDomainAttempt(row), nil
}

// GetAttemptByID retrieves an attempt by its ID.
func (r *Repository) GetAttemptByID(ctx context.Context, id uuid.UUID) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, domain.ErrAttemptNotFound
	}
	row, err := r.queries.GetAttemptByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAttemptNotFound
		}
		return nil, mapPgError(err)
	}
	return toDomainAttempt(row), nil
}

// ClaimAttemptForGrading atomically flips an in-progress attempt to grading.
// Returns pgx.ErrNoRows if another concurrent caller claimed it first (Trap 4).
func (r *Repository) ClaimAttemptForGrading(ctx context.Context, params ClaimAttemptParams) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, domain.ErrAttemptNotFound
	}
	var respBytes []byte
	if len(params.Response) > 0 {
		respBytes = []byte(params.Response)
	}

	row, err := r.queries.ClaimAttemptForGrading(ctx, sqlc.ClaimAttemptForGradingParams{
		ID:             params.ID,
		CreatedAt:      params.CreatedAt,
		IdempotencyKey: params.IdempotencyKey,
		Response:       respBytes,
	})
	if err != nil {
		return nil, err // Preserve pgx.ErrNoRows so service can distinguish race condition
	}
	return toDomainAttempt(row), nil
}

// UpdateAttemptStatus updates the attempt's status, score, grader, and duration upon completion.
func (r *Repository) UpdateAttemptStatus(
	ctx context.Context, params UpdateAttemptStatusParams,
) (*domain.Attempt, error) {
	if r.queries == nil {
		return nil, domain.ErrAttemptNotFound
	}
	row, err := r.queries.UpdateAttemptStatus(ctx, sqlc.UpdateAttemptStatusParams{
		ID:         params.ID,
		CreatedAt:  params.CreatedAt,
		Status:     params.Status,
		Score:      params.Score,
		Grader:     params.Grader,
		DurationMs: params.DurationMs,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAttemptNotFound
		}
		return nil, mapPgError(err)
	}
	return toDomainAttempt(row), nil
}

// GetProgressByUserScope retrieves progress for a specific scope.
func (r *Repository) GetProgressByUserScope(
	ctx context.Context, userID uuid.UUID, scope string, scopeID uuid.UUID,
) (*ProgressDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.GetProgressByUserScope(ctx, sqlc.GetProgressByUserScopeParams{
		UserID:  userID,
		Scope:   scope,
		ScopeID: scopeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapPgError(err)
	}
	return toProgressDTO(row), nil
}

// UpsertProgress creates or updates progress for a user and scope.
func (r *Repository) UpsertProgress(ctx context.Context, params UpsertProgressParams) (*ProgressDTO, error) {
	if r.queries == nil {
		return nil, nil
	}
	row, err := r.queries.UpsertProgress(ctx, sqlc.UpsertProgressParams{
		UserID:      params.UserID,
		Scope:       params.Scope,
		ScopeID:     params.ScopeID,
		Status:      params.Status,
		Score:       params.Score,
		CompletedAt: params.CompletedAt,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	return toProgressDTO(row), nil
}

// ListProgressByUserScopeAndIDs retrieves progress rows for a batch of scope IDs.
func (r *Repository) ListProgressByUserScopeAndIDs(
	ctx context.Context, userID uuid.UUID, scope string, scopeIDs []uuid.UUID,
) ([]ProgressDTO, error) {
	if r.queries == nil || len(scopeIDs) == 0 {
		return nil, nil
	}
	rows, err := r.queries.ListProgressByUserScopeAndIDs(ctx, sqlc.ListProgressByUserScopeAndIDsParams{
		UserID:   userID,
		Scope:    scope,
		ScopeIds: scopeIDs,
	})
	if err != nil {
		return nil, mapPgError(err)
	}
	out := make([]ProgressDTO, len(rows))
	for i, row := range rows {
		p := toProgressDTO(row)
		if p != nil {
			out[i] = *p
		}
	}
	return out, nil
}

func toDomainAttempt(row sqlc.LearnAttempt) *domain.Attempt {
	var keyStr *string
	if row.IdempotencyKey != nil {
		s := row.IdempotencyKey.String()
		keyStr = &s
	}
	// Integers all the way through: attempts.score and max_score are integer
	// columns, and contract.GradeResult declares them int. A float here would be
	// a third representation of the same number.
	var score *int
	if row.Score != nil {
		n := int(*row.Score)
		score = &n
	}
	var dur *int64
	if row.DurationMs > 0 {
		d := int64(row.DurationMs)
		dur = &d
	}
	return &domain.Attempt{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		UserID:         row.UserID,
		ActivityID:     row.ActivityID,
		IdempotencyKey: keyStr,
		Response:       json.RawMessage(row.Response),
		Score:          score,
		MaxScore:       int(row.MaxScore),
		Grader:         row.Grader,
		DurationMs:     dur,
		Status:         row.Status,
	}
}

func toProgressDTO(row sqlc.LearnProgress) *ProgressDTO {
	var score *float64
	if row.Score != nil {
		f := float64(*row.Score)
		score = &f
	}
	return &ProgressDTO{
		ID:          row.ID,
		UserID:      row.UserID,
		Scope:       row.Scope,
		ScopeID:     row.ScopeID,
		Status:      row.Status,
		Score:       score,
		CompletedAt: row.CompletedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func mapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return domain.ErrIdempotencyConflict
		case "23514": // check_violation
			switch pgErr.ConstraintName {
			case "ck_attempts_status":
				return domain.ErrInvalidStatus
			}
		}
	}
	return err
}
