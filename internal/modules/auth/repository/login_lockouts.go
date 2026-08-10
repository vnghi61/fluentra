package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcauth "github.com/fluentra/fluentra/internal/generated/auth/sqlc"
)

// GetActiveLoginLockout returns the current expiry for one account or IP
// bucket. A missing row is an unlocked bucket, not an error.
func (r *Repository) GetActiveLoginLockout(
	ctx context.Context, scope string, subjectHash []byte, now time.Time,
) (time.Time, bool, error) {
	lockedUntil, err := r.queries.GetActiveLoginLockout(ctx, sqlcauth.GetActiveLoginLockoutParams{
		Scope:       scope,
		SubjectHash: subjectHash,
		LockedUntil: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("get active login lockout: %w", err)
	}
	return lockedUntil, true, nil
}

// AdvanceLoginLockout atomically starts the next exponential lockout period.
// A false result means another request won the race and installed an active
// lock first; callers must treat that as locked as well.
func (r *Repository) AdvanceLoginLockout(
	ctx context.Context, scope string, subjectHash []byte, now time.Time,
) (time.Time, bool, error) {
	lockedUntil, err := r.queries.AdvanceLoginLockout(ctx, sqlcauth.AdvanceLoginLockoutParams{
		Scope:       scope,
		SubjectHash: subjectHash,
		Now:         now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("advance login lockout: %w", err)
	}
	return lockedUntil, true, nil
}
