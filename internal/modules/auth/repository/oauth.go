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

// CreateOAuthState records an in-flight authorization request.
func (r *Repository) CreateOAuthState(
	ctx context.Context, id uuid.UUID, state, provider, nonce string,
	verifierHash []byte, redirectTo *string, now, expiresAt time.Time,
) (domain.OAuthState, error) {
	row, err := r.queries.CreateOAuthState(ctx, sqlcauth.CreateOAuthStateParams{
		ID:               id,
		State:            state,
		Provider:         provider,
		Nonce:            nonce,
		PkceVerifierHash: verifierHash,
		RedirectTo:       redirectTo,
		Now:              now,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		return domain.OAuthState{}, fmt.Errorf("create oauth state: %w", err)
	}
	return toOAuthState(row), nil
}

// ConsumeOAuthState spends a state, or reports that nothing was spendable.
//
// The false return covers missing, unknown, expired and already-consumed, and
// deliberately does not say which. Those are the four shapes a CSRF attempt
// takes, and telling them apart in a response would help whoever is probing.
func (r *Repository) ConsumeOAuthState(ctx context.Context, state, provider string, now time.Time) (
	domain.OAuthState, bool, error,
) {
	row, err := r.queries.ConsumeOAuthState(ctx, sqlcauth.ConsumeOAuthStateParams{
		State:    state,
		Provider: provider,
		Now:      now,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OAuthState{}, false, nil
		}
		return domain.OAuthState{}, false, fmt.Errorf("consume oauth state: %w", err)
	}
	return toOAuthState(row), true, nil
}

// DeleteExpiredOAuthStates sweeps flows nobody came back for. Most rows here are
// abandoned consent screens, so this is the only thing that keeps the table from
// growing without bound.
func (r *Repository) DeleteExpiredOAuthStates(ctx context.Context, cutoff time.Time) (int, error) {
	affected, err := r.queries.DeleteExpiredOAuthStates(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete expired oauth states: %w", err)
	}
	return int(affected), nil
}

// CreateOAuthIdentity attaches a provider account to a local one.
func (r *Repository) CreateOAuthIdentity(
	ctx context.Context, id, userID uuid.UUID, provider, subject string, emailHash []byte, now time.Time,
) (domain.OAuthIdentity, error) {
	row, err := r.queries.CreateOAuthIdentity(ctx, sqlcauth.CreateOAuthIdentityParams{
		ID:        id,
		UserID:    userID,
		Provider:  provider,
		Subject:   subject,
		EmailHash: emailHash,
		Now:       now,
	})
	if err != nil {
		if isUniqueViolation(err, "uq_oauth_identities_subject") {
			return domain.OAuthIdentity{}, domain.ErrOAuthAlreadyLinked
		}
		if isUniqueViolation(err, "uq_oauth_identities_user_provider") {
			return domain.OAuthIdentity{}, domain.ErrOAuthAlreadyLinked
		}
		return domain.OAuthIdentity{}, fmt.Errorf("create oauth identity: %w", err)
	}
	return toOAuthIdentity(row), nil
}

// FindOAuthIdentityBySubject looks an identity up the way the callback does:
// by the provider's stable subject, never by the email.
//
// An address can be reassigned by a provider and a subject cannot, so matching
// on the address would mean whoever inherits a corporate mailbox inherits the
// Fluentra account with it.
func (r *Repository) FindOAuthIdentityBySubject(ctx context.Context, provider, subject string) (
	domain.OAuthIdentity, bool, error,
) {
	row, err := r.queries.GetOAuthIdentityBySubject(ctx, sqlcauth.GetOAuthIdentityBySubjectParams{
		Provider: provider, Subject: subject,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OAuthIdentity{}, false, nil
		}
		return domain.OAuthIdentity{}, false, fmt.Errorf("get oauth identity by subject: %w", err)
	}
	return toOAuthIdentity(row), true, nil
}

// FindOAuthIdentityByUser is the read behind the unlink and the account view.
func (r *Repository) FindOAuthIdentityByUser(ctx context.Context, userID uuid.UUID, provider string) (
	domain.OAuthIdentity, bool, error,
) {
	row, err := r.queries.GetOAuthIdentityByUser(ctx, sqlcauth.GetOAuthIdentityByUserParams{
		UserID: userID, Provider: provider,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OAuthIdentity{}, false, nil
		}
		return domain.OAuthIdentity{}, false, fmt.Errorf("get oauth identity by user: %w", err)
	}
	return toOAuthIdentity(row), true, nil
}

// DeleteOAuthIdentity removes the link, and reports whether there was one.
func (r *Repository) DeleteOAuthIdentity(ctx context.Context, userID uuid.UUID, provider string) (bool, error) {
	affected, err := r.queries.DeleteOAuthIdentity(ctx, sqlcauth.DeleteOAuthIdentityParams{
		UserID: userID, Provider: provider,
	})
	if err != nil {
		return false, fmt.Errorf("delete oauth identity: %w", err)
	}
	return affected > 0, nil
}

// CountSignInMethods counts the ways an account can be signed into: a password,
// plus each linked identity. It is the guard behind BR-AUTH-20.
func (r *Repository) CountSignInMethods(ctx context.Context, userID uuid.UUID) (int, error) {
	methods, err := r.queries.CountSignInMethods(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count sign-in methods: %w", err)
	}
	return int(methods), nil
}

func toOAuthState(row sqlcauth.CoreOauthState) domain.OAuthState {
	return domain.OAuthState{
		ID:               row.ID,
		State:            row.State,
		Provider:         row.Provider,
		Nonce:            row.Nonce,
		PKCEVerifierHash: row.PkceVerifierHash,
		RedirectTo:       row.RedirectTo,
		CreatedAt:        row.CreatedAt,
		ExpiresAt:        row.ExpiresAt,
		ConsumedAt:       row.ConsumedAt,
	}
}

func toOAuthIdentity(row sqlcauth.CoreOauthIdentity) domain.OAuthIdentity {
	return domain.OAuthIdentity{
		ID:        row.ID,
		UserID:    row.UserID,
		Provider:  row.Provider,
		Subject:   row.Subject,
		EmailHash: row.EmailHash,
		LinkedAt:  row.LinkedAt,
	}
}
