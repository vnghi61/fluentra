package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlcauth "github.com/fluentra/fluentra/internal/generated/auth/sqlc"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// CreateChallenge writes a new challenge.
func (r *Repository) CreateChallenge(ctx context.Context, challenge domain.NewChallengeInput) (
	domain.Challenge, error,
) {
	row, err := r.queries.CreateChallenge(ctx, sqlcauth.CreateChallengeParams{
		ID:          challenge.ID,
		Purpose:     sqlcauth.CoreChallengePurpose(challenge.Purpose),
		SubjectHash: challenge.SubjectHash,
		CodeHash:    challenge.CodeHash,
		MaxAttempts: int32(challenge.MaxAttempts), //nolint:gosec // bounded 1..10 by ck_auth_challenges_max_attempts
		ExpiresAt:   challenge.ExpiresAt,
		Now:         challenge.Now,
	})
	if err != nil {
		return domain.Challenge{}, fmt.Errorf("create challenge: %w", err)
	}
	return toDomainChallenge(row), nil
}

// GetChallenge reads a challenge by id.
func (r *Repository) GetChallenge(ctx context.Context, id uuid.UUID) (domain.Challenge, error) {
	row, err := r.queries.GetChallengeByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Challenge{}, domain.ErrChallengeNotFound
		}
		return domain.Challenge{}, fmt.Errorf("get challenge by id: %w", err)
	}
	return toDomainChallenge(row), nil
}

// ConsumeChallenge marks the challenge used, but only if it is still usable and
// still holds codeHash.
//
// The second return value is false when no row matched. That is not an error:
// it is the ordinary outcome of two requests racing, or of a resend landing
// between the caller's read and this write, and the caller decides what to say
// about it. Reporting it as an error here would lose the distinction between
// "lost the race" and "the database is broken".
func (r *Repository) ConsumeChallenge(ctx context.Context, id uuid.UUID, codeHash []byte, now time.Time) (
	domain.Challenge, bool, error,
) {
	row, err := r.queries.ConsumeChallenge(ctx, sqlcauth.ConsumeChallengeParams{
		ID: id, CodeHash: codeHash, Now: now,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Challenge{}, false, nil
		}
		return domain.Challenge{}, false, fmt.Errorf("consume challenge: %w", err)
	}
	return toDomainChallenge(row), true, nil
}

// RecordFailedAttempt charges one attempt against the challenge.
//
// False means the statement matched nothing — the challenge was already burned,
// consumed or expired by the time the write arrived. The caller reads that as
// "no attempt was available to charge" rather than as a failure.
func (r *Repository) RecordFailedAttempt(ctx context.Context, id uuid.UUID, now time.Time) (
	domain.Challenge, bool, error,
) {
	row, err := r.queries.RecordFailedAttempt(ctx, sqlcauth.RecordFailedAttemptParams{ID: id, Now: now})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Challenge{}, false, nil
		}
		return domain.Challenge{}, false, fmt.Errorf("record failed attempt: %w", err)
	}
	return toDomainChallenge(row), true, nil
}

// ResendChallenge replaces the code and clears the attempts, leaving expires_at
// alone. resendAllowedFrom is the cooldown boundary: a row whose last_sent_at is
// later than it does not match, and false comes back.
func (r *Repository) ResendChallenge(
	ctx context.Context, id uuid.UUID, codeHash []byte, resendAllowedFrom, now time.Time,
) (domain.Challenge, bool, error) {
	row, err := r.queries.ResendChallenge(ctx, sqlcauth.ResendChallengeParams{
		ID: id, CodeHash: codeHash, ResendAllowedFrom: resendAllowedFrom, Now: now,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Challenge{}, false, nil
		}
		return domain.Challenge{}, false, fmt.Errorf("resend challenge: %w", err)
	}
	return toDomainChallenge(row), true, nil
}

func toDomainChallenge(row sqlcauth.CoreAuthChallenge) domain.Challenge {
	return domain.Challenge{
		ID:          row.ID,
		Purpose:     domain.Purpose(row.Purpose),
		SubjectHash: row.SubjectHash,
		CodeHash:    row.CodeHash,
		Attempts:    int(row.Attempts),
		MaxAttempts: int(row.MaxAttempts),
		ExpiresAt:   row.ExpiresAt,
		ConsumedAt:  row.ConsumedAt,
		LastSentAt:  row.LastSentAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
