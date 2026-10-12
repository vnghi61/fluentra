// Package repository persists the admin module's tables — admin_actions,
// feature_flags and banners — and nothing else. Every cross-module read goes through a
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

// Repository manages persistence for admin tables (admin_actions, feature_flags, banners).
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

// bannerImageKeyConstraint keeps one banner per image (migration 952).
const bannerImageKeyConstraint = "uq_banners_image_key"

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

// ListBanners returns every banner for the admin screen, capped by the query.
func (r *Repository) ListBanners(ctx context.Context) ([]domain.Banner, error) {
	rows, err := r.queries.ListBanners(ctx)
	if err != nil {
		return nil, fmt.Errorf("list banners: %w", err)
	}
	return toDomainBanners(rows), nil
}

// ListActiveBanners returns at most limit banners showing at instant at, in
// display order.
func (r *Repository) ListActiveBanners(ctx context.Context, at time.Time, limit int32) ([]domain.Banner, error) {
	rows, err := r.queries.ListActiveBanners(ctx, sqlcadmin.ListActiveBannersParams{At: at, MaxRows: limit})
	if err != nil {
		return nil, fmt.Errorf("list active banners: %w", err)
	}
	return toDomainBanners(rows), nil
}

// GetBanner returns one banner. A missing id is domain.ErrBannerNotFound.
func (r *Repository) GetBanner(ctx context.Context, id uuid.UUID) (domain.Banner, error) {
	row, err := r.queries.GetBannerByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Banner{}, domain.ErrBannerNotFound
		}
		return domain.Banner{}, fmt.Errorf("get banner: %w", err)
	}
	return toDomainBanner(row), nil
}

// CreateBanner inserts a validated banner with its resolved image, if any.
func (r *Repository) CreateBanner(
	ctx context.Context, in domain.BannerInput, image *domain.BannerImage, createdBy uuid.UUID,
) (domain.Banner, error) {
	key, alt, width, height := imageColumns(image)
	row, err := r.queries.CreateBanner(ctx, sqlcadmin.CreateBannerParams{
		Title:       in.Title,
		Body:        in.Body,
		CtaLabel:    in.CTALabel,
		CtaUrl:      in.CTAURL,
		Tone:        string(in.Tone),
		IsActive:    in.IsActive,
		Priority:    int32(in.Priority), //nolint:gosec // validated to 0..1000 by the domain
		Dismissible: in.Dismissible,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		CreatedBy:   createdBy,
		ImageKey:    key,
		ImageAlt:    alt,
		ImageWidth:  width,
		ImageHeight: height,
		Kind:        string(in.Kind),
	})
	if err != nil {
		if isUniqueViolation(err, bannerImageKeyConstraint) {
			return domain.Banner{}, domain.ErrBannerImageInUse
		}
		return domain.Banner{}, fmt.Errorf("create banner: %w", err)
	}
	return toDomainBanner(row), nil
}

// UpdateBanner replaces a banner's editable fields and image. A missing id is
// domain.ErrBannerNotFound.
func (r *Repository) UpdateBanner(
	ctx context.Context, id uuid.UUID, in domain.BannerInput, image *domain.BannerImage,
) (domain.Banner, error) {
	key, alt, width, height := imageColumns(image)
	row, err := r.queries.UpdateBanner(ctx, sqlcadmin.UpdateBannerParams{
		ID:          id,
		Title:       in.Title,
		Body:        in.Body,
		CtaLabel:    in.CTALabel,
		CtaUrl:      in.CTAURL,
		Tone:        string(in.Tone),
		IsActive:    in.IsActive,
		Priority:    int32(in.Priority), //nolint:gosec // validated to 0..1000 by the domain
		Dismissible: in.Dismissible,
		StartsAt:    in.StartsAt,
		EndsAt:      in.EndsAt,
		ImageKey:    key,
		ImageAlt:    alt,
		ImageWidth:  width,
		ImageHeight: height,
		Kind:        string(in.Kind),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Banner{}, domain.ErrBannerNotFound
		}
		if isUniqueViolation(err, bannerImageKeyConstraint) {
			return domain.Banner{}, domain.ErrBannerImageInUse
		}
		return domain.Banner{}, fmt.Errorf("update banner: %w", err)
	}
	return toDomainBanner(row), nil
}

// DeleteBanner removes a banner. A missing id is domain.ErrBannerNotFound.
func (r *Repository) DeleteBanner(ctx context.Context, id uuid.UUID) error {
	affected, err := r.queries.DeleteBanner(ctx, id)
	if err != nil {
		return fmt.Errorf("delete banner: %w", err)
	}
	if affected == 0 {
		return domain.ErrBannerNotFound
	}
	return nil
}

// imageColumns spreads an optional image over its four columns; the CHECK on
// core.banners requires them all set or all null (alt is ” without one).
func imageColumns(image *domain.BannerImage) (key *string, alt string, width, height *int32) {
	if image == nil {
		return nil, "", nil, nil
	}
	k := image.Key
	w := int32(image.Width)  //nolint:gosec // bounded to 1600 by processing and the CHECK
	h := int32(image.Height) //nolint:gosec // bounded to 900 by processing and the CHECK
	return &k, image.Alt, &w, &h
}

func toDomainBanners(rows []sqlcadmin.CoreBanner) []domain.Banner {
	banners := make([]domain.Banner, 0, len(rows))
	for _, row := range rows {
		banners = append(banners, toDomainBanner(row))
	}
	return banners
}

func toDomainBanner(row sqlcadmin.CoreBanner) domain.Banner {
	var image *domain.BannerImage
	if row.ImageKey != nil && row.ImageWidth != nil && row.ImageHeight != nil {
		image = &domain.BannerImage{
			Key:    *row.ImageKey,
			Alt:    row.ImageAlt,
			Width:  int(*row.ImageWidth),
			Height: int(*row.ImageHeight),
		}
	}
	return domain.Banner{
		ID:          row.ID,
		Kind:        domain.BannerKind(row.Kind),
		Title:       row.Title,
		Body:        row.Body,
		CTALabel:    row.CtaLabel,
		CTAURL:      row.CtaUrl,
		Tone:        domain.BannerTone(row.Tone),
		IsActive:    row.IsActive,
		Priority:    int(row.Priority),
		Dismissible: row.Dismissible,
		StartsAt:    row.StartsAt,
		EndsAt:      row.EndsAt,
		Image:       image,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
