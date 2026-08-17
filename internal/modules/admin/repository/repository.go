// Package repository persists the admin module's tables — admin_actions and
// feature_flags — and nothing else. Every cross-module read goes through a
// contract, so this package never imports another module's repository.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcadmin "github.com/fluentra/fluentra/internal/generated/admin/sqlc"
	"github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository manages persistence for admin tables (admin_actions, feature_flags).
type Repository struct {
	queries *sqlcadmin.Queries
}

// New creates a new admin repository over db.
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlcadmin.New(db)}
}

// WithTx returns a repository tied to tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{queries: sqlcadmin.New(tx)}
}

// LogAdminAction records an admin operation log entry.
func (r *Repository) LogAdminAction(
	ctx context.Context,
	actorID, targetID uuid.UUID,
	action, reason string,
) (sqlcadmin.CoreAdminAction, error) {
	entry, err := r.queries.LogAdminAction(ctx, sqlcadmin.LogAdminActionParams{
		ActorID:  actorID,
		TargetID: targetID,
		Action:   action,
		Reason:   reason,
	})
	if err != nil {
		return sqlcadmin.CoreAdminAction{}, fmt.Errorf("log admin action: %w", err)
	}
	return entry, nil
}

// ListFeatureFlags retrieves all feature flags from the database.
func (r *Repository) ListFeatureFlags(ctx context.Context) ([]sqlcadmin.CoreFeatureFlag, error) {
	flags, err := r.queries.ListFeatureFlags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list feature flags: %w", err)
	}
	return flags, nil
}

// GetFeatureFlagByKey retrieves a feature flag by key.
func (r *Repository) GetFeatureFlagByKey(ctx context.Context, key string) (sqlcadmin.CoreFeatureFlag, error) {
	flag, err := r.queries.GetFeatureFlagByKey(ctx, key)
	if err != nil {
		return sqlcadmin.CoreFeatureFlag{}, fmt.Errorf("get feature flag by key: %w", err)
	}
	return flag, nil
}

// CreateFeatureFlag inserts a new feature flag into the database.
func (r *Repository) CreateFeatureFlag(
	ctx context.Context,
	key string,
	enabled bool,
	rolloutPercent int32,
	owner string,
	expiresOn time.Time,
	description string,
) (sqlcadmin.CoreFeatureFlag, error) {
	flag, err := r.queries.CreateFeatureFlag(ctx, sqlcadmin.CreateFeatureFlagParams{
		Key:            key,
		Enabled:        enabled,
		RolloutPercent: rolloutPercent,
		Owner:          owner,
		ExpiresOn:      pgtype.Date{Time: expiresOn, Valid: true},
		Description:    description,
	})
	if err != nil {
		if isUniqueViolation(err, featureFlagKeyConstraint) {
			return sqlcadmin.CoreFeatureFlag{}, domain.ErrFlagAlreadyExists
		}
		return sqlcadmin.CoreFeatureFlag{}, fmt.Errorf("create feature flag: %w", err)
	}
	return flag, nil
}

// featureFlagKeyConstraint is the primary key on core.feature_flags. Reusing a
// retired flag's key is the ordinary way to hit it.
const featureFlagKeyConstraint = "feature_flags_pkey"

// isUniqueViolation reports whether err is a unique constraint failure on the
// named constraint, so a re-used key is answered as a 409 rather than a 500.
// Named rather than blanket, for the reason auth's copy gives: any unique
// violation reported as "already exists" hides the ones that are genuinely
// bugs.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// UpdateFeatureFlag updates an existing feature flag.
func (r *Repository) UpdateFeatureFlag(
	ctx context.Context,
	key string,
	enabled bool,
	rolloutPercent int32,
	expiresOn time.Time,
	description string,
) (sqlcadmin.CoreFeatureFlag, error) {
	flag, err := r.queries.UpdateFeatureFlag(ctx, sqlcadmin.UpdateFeatureFlagParams{
		Key:            key,
		Enabled:        enabled,
		RolloutPercent: rolloutPercent,
		ExpiresOn:      pgtype.Date{Time: expiresOn, Valid: true},
		Description:    description,
	})
	if err != nil {
		return sqlcadmin.CoreFeatureFlag{}, fmt.Errorf("update feature flag: %w", err)
	}
	return flag, nil
}

// DeleteFeatureFlag removes a feature flag by key.
func (r *Repository) DeleteFeatureFlag(ctx context.Context, key string) error {
	if err := r.queries.DeleteFeatureFlag(ctx, key); err != nil {
		return fmt.Errorf("delete feature flag: %w", err)
	}
	return nil
}

// GetFlagsExpiringWithin returns feature flags expiring before or on cutoff.
func (r *Repository) GetFlagsExpiringWithin(
	ctx context.Context, cutoff time.Time,
) ([]sqlcadmin.CoreFeatureFlag, error) {
	flags, err := r.queries.GetFlagsExpiringWithin(ctx, pgtype.Date{Time: cutoff, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get flags expiring within: %w", err)
	}
	return flags, nil
}
