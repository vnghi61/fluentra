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

// CreateSession opens a sign-in. The id is supplied by the caller because it is
// already in the access token's `sid` claim by the time this runs.
func (r *Repository) CreateSession(
	ctx context.Context, id, userID uuid.UUID, deviceLabel *string, ipHash, userAgentHash []byte, now time.Time,
) error {
	_, err := r.queries.CreateSession(ctx, sqlcauth.CreateSessionParams{
		ID:            id,
		UserID:        userID,
		DeviceLabel:   deviceLabel,
		IpHash:        ipHash,
		UserAgentHash: userAgentHash,
		Now:           now,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// TouchSession moves last_seen_at forward. A revoked session is not touched and
// is not an error: the caller that rotates a token has already been refused by
// the claim, and a second failure here would say nothing new.
func (r *Repository) TouchSession(ctx context.Context, sessionID uuid.UUID, now time.Time) error {
	if _, err := r.queries.TouchSession(ctx, sqlcauth.TouchSessionParams{ID: sessionID, Now: now}); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// RevokeSession ends a sign-in. It reports whether a row changed, which is how
// the caller tells "revoked it" from "it was already revoked" without a second
// read — the difference decides whether a security event is raised twice.
func (r *Repository) RevokeSession(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	affected, err := r.queries.RevokeSession(ctx, sqlcauth.RevokeSessionParams{ID: sessionID, Now: now})
	if err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	return affected > 0, nil
}

// CreateRefreshToken writes the next token in a family.
func (r *Repository) CreateRefreshToken(
	ctx context.Context, id uuid.UUID, tokenHash []byte, familyID, sessionID uuid.UUID, now, expiresAt time.Time,
) (domain.RefreshToken, error) {
	row, err := r.queries.CreateRefreshToken(ctx, sqlcauth.CreateRefreshTokenParams{
		ID:        id,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		SessionID: sessionID,
		Now:       now,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("create refresh token: %w", err)
	}
	return toRefreshToken(row), nil
}

// ClaimRefreshToken spends a token and returns it, or reports that nothing was
// claimable.
//
// The false return covers four different situations — no such token, already
// spent, already revoked, expired — and deliberately does not say which. The
// statement is a single UPDATE precisely so that no read happens between the
// check and the write; adding a discriminating read inside it would put the gap
// back. FindRefreshToken is what the caller uses afterwards to find out why,
// once the race is already decided.
func (r *Repository) ClaimRefreshToken(ctx context.Context, tokenHash []byte, now time.Time) (
	domain.SessionToken, bool, error,
) {
	row, err := r.queries.ClaimRefreshToken(ctx, sqlcauth.ClaimRefreshTokenParams{
		TokenHash: tokenHash,
		Now:       now,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SessionToken{}, false, nil
		}
		return domain.SessionToken{}, false, fmt.Errorf("claim refresh token: %w", err)
	}
	return domain.SessionToken{
		RefreshToken: domain.RefreshToken{
			ID:        row.ID,
			TokenHash: row.TokenHash,
			FamilyID:  row.FamilyID,
			SessionID: row.SessionID,
			IssuedAt:  row.IssuedAt,
			ExpiresAt: row.ExpiresAt,
			UsedAt:    row.UsedAt,
			RevokedAt: row.RevokedAt,
		},
		UserID: row.UserID,
	}, true, nil
}

// FindRefreshToken reads a row without changing it.
func (r *Repository) FindRefreshToken(ctx context.Context, tokenHash []byte) (domain.SessionToken, bool, error) {
	row, err := r.queries.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SessionToken{}, false, nil
		}
		return domain.SessionToken{}, false, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return domain.SessionToken{
		RefreshToken: domain.RefreshToken{
			ID:        row.ID,
			TokenHash: row.TokenHash,
			FamilyID:  row.FamilyID,
			SessionID: row.SessionID,
			IssuedAt:  row.IssuedAt,
			ExpiresAt: row.ExpiresAt,
			UsedAt:    row.UsedAt,
			RevokedAt: row.RevokedAt,
		},
		UserID: row.UserID,
	}, true, nil
}

// RevokeRefreshFamily takes away every token descended from one sign-in, and
// returns how many it took. One statement, so a family cannot be half revoked
// by a caller that stopped iterating.
func (r *Repository) RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID, now time.Time) (int, error) {
	affected, err := r.queries.RevokeRefreshFamily(ctx, sqlcauth.RevokeRefreshFamilyParams{
		FamilyID: familyID,
		Now:      now,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke refresh family: %w", err)
	}
	return int(affected), nil
}

func toRefreshToken(row sqlcauth.CoreRefreshToken) domain.RefreshToken {
	return domain.RefreshToken{
		ID:        row.ID,
		TokenHash: row.TokenHash,
		FamilyID:  row.FamilyID,
		SessionID: row.SessionID,
		IssuedAt:  row.IssuedAt,
		ExpiresAt: row.ExpiresAt,
		UsedAt:    row.UsedAt,
		RevokedAt: row.RevokedAt,
	}
}
