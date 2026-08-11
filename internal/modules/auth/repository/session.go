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

// ListLiveSessions returns the account's sessions that have not been revoked,
// most recently active first.
func (r *Repository) ListLiveSessions(ctx context.Context, userID uuid.UUID) ([]domain.Session, error) {
	rows, err := r.queries.ListLiveSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list live sessions: %w", err)
	}
	sessions := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, toSession(row))
	}
	return sessions, nil
}

// GetOwnedSession reads a session that belongs to userID.
//
// A session that exists but belongs to somebody else is reported as not found,
// identically to one that never existed, because the SQL puts both the id and
// the owner in the WHERE clause. That is what makes the 404 structural rather
// than a comparison a later edit could drop.
func (r *Repository) GetOwnedSession(ctx context.Context, sessionID, userID uuid.UUID) (
	domain.Session, bool, error,
) {
	row, err := r.queries.GetOwnedSession(ctx, sqlcauth.GetOwnedSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, false, nil
		}
		return domain.Session{}, false, fmt.Errorf("get owned session: %w", err)
	}
	return toSession(row), true, nil
}

// RevokeAllSessionsForUser ends every live session an account has, and returns
// how many it ended.
func (r *Repository) RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	affected, err := r.queries.RevokeAllSessionsForUser(ctx, sqlcauth.RevokeAllSessionsForUserParams{
		UserID: userID,
		Now:    now,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions: %w", err)
	}
	return int(affected), nil
}

// RevokeRefreshTokensBySession takes away everything a session could still
// renew with. It is the half of revocation that acts immediately — the access
// token the session issued survives to its own expiry (ADR-0007).
func (r *Repository) RevokeRefreshTokensBySession(
	ctx context.Context, sessionID uuid.UUID, now time.Time,
) (int, error) {
	affected, err := r.queries.RevokeRefreshTokensBySession(ctx, sqlcauth.RevokeRefreshTokensBySessionParams{
		SessionID: sessionID,
		Now:       now,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke refresh tokens by session: %w", err)
	}
	return int(affected), nil
}

// RevokeRefreshTokensForUser is the same, across every session an account has.
func (r *Repository) RevokeRefreshTokensForUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	affected, err := r.queries.RevokeRefreshTokensForUser(ctx, sqlcauth.RevokeRefreshTokensForUserParams{
		UserID: userID,
		Now:    now,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke refresh tokens for user: %w", err)
	}
	return int(affected), nil
}

func toSession(row sqlcauth.CoreSession) domain.Session {
	return domain.Session{
		ID:            row.ID,
		UserID:        row.UserID,
		DeviceLabel:   row.DeviceLabel,
		IPHash:        row.IpHash,
		UserAgentHash: row.UserAgentHash,
		CreatedAt:     row.CreatedAt,
		LastSeenAt:    row.LastSeenAt,
		RevokedAt:     row.RevokedAt,
	}
}
