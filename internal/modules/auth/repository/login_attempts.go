package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	sqlcauth "github.com/fluentra/fluentra/internal/generated/auth/sqlc"
)

// RecordLoginAttempt stores a login attempt record for audit forensics.
func (r *Repository) RecordLoginAttempt(
	ctx context.Context, id uuid.UUID, userID *uuid.UUID, emailHash, ipHash []byte, success bool, failureReason *string, createdAt time.Time,
) error {
	_, err := r.queries.RecordLoginAttempt(ctx, sqlcauth.RecordLoginAttemptParams{
		ID:            id,
		UserID:        userID,
		EmailHash:     emailHash,
		IpHash:        ipHash,
		Success:       success,
		FailureReason: failureReason,
		CreatedAt:     createdAt,
	})
	if err != nil {
		return fmt.Errorf("record login attempt: %w", err)
	}
	return nil
}

// CountRecentFailedAttemptsByAccount counts failed login attempts for an email hash since a given time.
func (r *Repository) CountRecentFailedAttemptsByAccount(
	ctx context.Context, emailHash []byte, since time.Time,
) (int64, error) {
	count, err := r.queries.CountRecentFailedAttemptsByAccount(ctx, sqlcauth.CountRecentFailedAttemptsByAccountParams{
		EmailHash: emailHash,
		CreatedAt: since,
	})
	if err != nil {
		return 0, fmt.Errorf("count failed attempts by account: %w", err)
	}
	return count, nil
}

// CountRecentFailedAttemptsByIP counts failed login attempts for an IP hash since a given time.
func (r *Repository) CountRecentFailedAttemptsByIP(
	ctx context.Context, ipHash []byte, since time.Time,
) (int64, error) {
	count, err := r.queries.CountRecentFailedAttemptsByIP(ctx, sqlcauth.CountRecentFailedAttemptsByIPParams{
		IpHash:    ipHash,
		CreatedAt: since,
	})
	if err != nil {
		return 0, fmt.Errorf("count failed attempts by IP: %w", err)
	}
	return count, nil
}
