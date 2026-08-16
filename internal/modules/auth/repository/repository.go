// Package repository is this module's adapter layer: the place where the ports
// the domain declares meet something concrete. Most of that is the sqlc-
// generated queries over core.credentials; one of it is the Have I Been Pwned
// range API, in breach.go, which is an external read like any other and belongs
// beside the reads it sits next to rather than in a facade of its own.
//
// There are no business rules here. Whether a password is acceptable is
// domain.Policy's decision; this package only fetches what that decision needs.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sqlcauth "github.com/fluentra/fluentra/internal/generated/auth/sqlc"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

// Repository reads and writes core.credentials.
type Repository struct {
	queries *sqlcauth.Queries
}

// New creates a repository over db.
//
// The parameter is dbx.Querier rather than an interface declared here, for the
// reason the user module gives: both *pgxpool.Pool and pgx.Tx satisfy it, so
// the service can hand the same repository a transaction without the repository
// knowing it is in one. Registration needs exactly that — the user row, the
// credential row and the outbox row are one transaction (rule L4).
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlcauth.New(db)}
}

// WithTx returns a repository that runs inside tx. The receiver is unchanged.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{queries: sqlcauth.New(tx)}
}

// Create stores the first password for an account.
func (r *Repository) Create(ctx context.Context, id, userID uuid.UUID, passwordHash string) (
	domain.Credential, error,
) {
	row, err := r.queries.CreateCredential(ctx, sqlcauth.CreateCredentialParams{
		ID:           id,
		UserID:       userID,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if isUniqueViolation(err, "uq_credentials_user") {
			return domain.Credential{}, domain.ErrCredentialAlreadyExists
		}
		return domain.Credential{}, fmt.Errorf("create credential: %w", err)
	}
	return toDomain(row), nil
}

// GetByUserID reads the credential for an account.
//
// An account with no row is domain.ErrCredentialNotFound rather than a zero
// value: a caller that forgot to check would otherwise verify a password
// against the empty string, and the empty string is not a hash the verifier
// rejects for the reason the caller would assume.
func (r *Repository) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Credential, error) {
	row, err := r.queries.GetCredentialByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Credential{}, domain.ErrCredentialNotFound
		}
		return domain.Credential{}, fmt.Errorf("get credential by user id: %w", err)
	}
	return toDomain(row), nil
}

// ReplaceHash overwrites the stored hash. It is the write behind both
// rehash-on-login and a password change; the row keeps no history that would
// let the two be told apart afterwards.
func (r *Repository) ReplaceHash(ctx context.Context, userID uuid.UUID, passwordHash string) (
	domain.Credential, error,
) {
	row, err := r.queries.ReplaceCredentialHash(ctx, sqlcauth.ReplaceCredentialHashParams{
		UserID:       userID,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Credential{}, domain.ErrCredentialNotFound
		}
		return domain.Credential{}, fmt.Errorf("replace credential hash: %w", err)
	}
	return toDomain(row), nil
}

// PurgeUser hard-deletes credentials, sessions, refresh tokens, devices, OAuth identities, and challenges for userID.
func (r *Repository) PurgeUser(ctx context.Context, userID uuid.UUID) error {
	if err := r.queries.DeleteRefreshTokensForUser(ctx, userID); err != nil {
		return fmt.Errorf("purge refresh tokens: %w", err)
	}
	if err := r.queries.DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("purge sessions: %w", err)
	}
	if err := r.queries.DeleteCredentialByUserID(ctx, userID); err != nil {
		return fmt.Errorf("purge credentials: %w", err)
	}
	if err := r.queries.DeleteTrustedDevicesForUser(ctx, userID); err != nil {
		return fmt.Errorf("purge trusted devices: %w", err)
	}
	if err := r.queries.DeleteOAuthIdentitiesForUser(ctx, userID); err != nil {
		return fmt.Errorf("purge oauth identities: %w", err)
	}
	if err := r.queries.DeleteChallengesForUser(ctx, &userID); err != nil {
		return fmt.Errorf("purge challenges: %w", err)
	}
	return nil
}

func toDomain(row sqlcauth.CoreCredential) domain.Credential {
	return domain.Credential{
		ID:           row.ID,
		UserID:       row.UserID,
		PasswordHash: secret.New(row.PasswordHash),
		AlgoParams:   row.AlgoParams,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

// isUniqueViolation reports whether err is a unique constraint failure on the
// named constraint. Matching the name and not only the code keeps the mapping
// honest if the table ever grows a second unique constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
