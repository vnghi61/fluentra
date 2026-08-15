// Package repository maps rows to domain values and back. It holds no business
// rules: everything here is either a call into the sqlc-generated queries or a
// conversion between the shapes those queries use and the ones the domain does.
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

	sqlcuser "github.com/fluentra/fluentra/internal/generated/user/sqlc"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/shared/dbx"
)

// Repository reads and writes the user module's tables.
type Repository struct {
	queries *sqlcuser.Queries
}

// New creates a repository over db.
//
// The parameter is dbx.Querier rather than an interface declared here. Both
// *pgxpool.Pool and pgx.Tx satisfy it, which is what lets the service hand the
// same repository a transaction without the repository knowing whether it is
// in one — and a type from the shared kernel crosses a component boundary
// without creating one, which an interface owned by this package would.
func New(db dbx.Querier) *Repository {
	return &Repository{queries: sqlcuser.New(db)}
}

// WithTx returns a repository that runs inside tx. The receiver is unchanged,
// so a service holding one repository can safely derive per-transaction copies
// from it concurrently.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{queries: sqlcuser.New(tx)}
}

// GetUser reads the identity record.
func (r *Repository) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return toDomainUser(row)
}

// GetUserByEmail reads the identity record by address.
//
// email is citext, so the match is case-insensitive in the database rather than
// by a lower() the caller could forget (BR-USER-01).
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return toDomainUser(row)
}

// MarkEmailVerified stamps the address as proved, keeping the first timestamp
// if there already is one.
func (r *Repository) MarkEmailVerified(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	row, err := r.queries.MarkUserEmailVerified(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("mark user email verified: %w", err)
	}
	return toDomainUser(row)
}

// PurgeUnverifiedBefore deletes accounts that never completed verification.
func (r *Repository) PurgeUnverifiedBefore(ctx context.Context, cutoff time.Time) (int, error) {
	removed, err := r.queries.PurgeUnverifiedUsersBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge unverified users: %w", err)
	}
	return int(removed), nil
}

// Exists reports whether the account exists.
func (r *Repository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.queries.UserExists(ctx, id)
	if err != nil {
		return false, fmt.Errorf("user exists: %w", err)
	}
	return exists, nil
}

// CreateUser inserts the identity record.
func (r *Repository) CreateUser(ctx context.Context, id uuid.UUID, email string, status domain.Status) (
	domain.User, error,
) {
	row, err := r.queries.CreateUser(ctx, sqlcuser.CreateUserParams{
		ID:     id,
		Email:  email,
		Status: sqlcuser.CoreUserStatus(status),
	})
	if err != nil {
		if isUniqueViolation(err, "uq_users_email") {
			return domain.User{}, domain.ErrEmailAlreadyRegistered
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return toDomainUser(row)
}

// GetProfile reads the descriptive record.
func (r *Repository) GetProfile(ctx context.Context, userID uuid.UUID) (domain.Profile, error) {
	row, err := r.queries.GetProfileByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Profile{}, domain.ErrProfileNotFound
		}
		return domain.Profile{}, fmt.Errorf("get profile by user id: %w", err)
	}
	return toDomainProfile(row), nil
}

// CreateProfile inserts the descriptive record.
func (r *Repository) CreateProfile(ctx context.Context, id, userID uuid.UUID, profile domain.Profile) (
	domain.Profile, error,
) {
	row, err := r.queries.CreateProfile(ctx, sqlcuser.CreateProfileParams{
		ID:          id,
		UserID:      userID,
		DisplayName: profile.DisplayName,
		Country:     profile.Country,
		Timezone:    profile.Timezone,
		DateOfBirth: toPgDate(profile.DateOfBirth),
	})
	if err != nil {
		return domain.Profile{}, fmt.Errorf("create profile: %w", err)
	}
	return toDomainProfile(row), nil
}

// UpdateProfile applies a partial change. A nil field in change is left alone
// by the query's COALESCE, which is why the parameters are pointers all the
// way from the HTTP body to the SQL.
func (r *Repository) UpdateProfile(ctx context.Context, userID uuid.UUID, change domain.ProfileChange) (
	domain.Profile, error,
) {
	row, err := r.queries.UpdateProfile(ctx, sqlcuser.UpdateProfileParams{
		UserID:      userID,
		DisplayName: change.DisplayName,
		Country:     change.Country,
		Timezone:    change.Timezone,
		DateOfBirth: toPgDate(change.DateOfBirth),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Profile{}, domain.ErrProfileNotFound
		}
		return domain.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return toDomainProfile(row), nil
}

// UpdateProfileAvatar sets or clears the avatar asset id.
func (r *Repository) UpdateProfileAvatar(ctx context.Context, userID uuid.UUID, avatarAssetID *uuid.UUID) (
	domain.Profile, error,
) {
	row, err := r.queries.UpdateProfileAvatar(ctx, sqlcuser.UpdateProfileAvatarParams{
		UserID:        userID,
		AvatarAssetID: avatarAssetID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Profile{}, domain.ErrProfileNotFound
		}
		return domain.Profile{}, fmt.Errorf("update profile avatar: %w", err)
	}
	return toDomainProfile(row), nil
}

// GetPreferences reads the settings record.
func (r *Repository) GetPreferences(ctx context.Context, userID uuid.UUID) (domain.Preferences, error) {
	row, err := r.queries.GetUserPreferences(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Preferences{}, domain.ErrPreferencesNotFound
		}
		return domain.Preferences{}, fmt.Errorf("get user preferences: %w", err)
	}
	return toDomainPreferences(row)
}

// CreatePreferences inserts the settings record with the column defaults.
func (r *Repository) CreatePreferences(ctx context.Context, id, userID uuid.UUID) (domain.Preferences, error) {
	row, err := r.queries.CreateUserPreferences(ctx, sqlcuser.CreateUserPreferencesParams{ID: id, UserID: userID})
	if err != nil {
		return domain.Preferences{}, fmt.Errorf("create user preferences: %w", err)
	}
	return toDomainPreferences(row)
}

// ReplacePreferences writes the whole settings record.
func (r *Repository) ReplacePreferences(ctx context.Context, preferences domain.Preferences) (
	domain.Preferences, error,
) {
	row, err := r.queries.ReplaceUserPreferences(ctx, sqlcuser.ReplaceUserPreferencesParams{
		UserID:               preferences.UserID,
		Locale:               preferences.Locale,
		Theme:                sqlcuser.CoreUiTheme(preferences.Theme),
		DailyGoalMinutes:     int32(preferences.DailyGoalMinutes), //nolint:gosec // bounded 5..480 by domain.Validate
		NotificationChannels: channelsToStrings(preferences.NotificationChannels),
		QuietHoursStart:      toPgTime(quietStart(preferences.QuietHours)),
		QuietHoursEnd:        toPgTime(quietEnd(preferences.QuietHours)),
		AiProcessingOptOut:   preferences.AIProcessingOptOut,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Preferences{}, domain.ErrPreferencesNotFound
		}
		return domain.Preferences{}, fmt.Errorf("replace user preferences: %w", err)
	}
	return toDomainPreferences(row)
}

// GetSummary reads one rendering summary.
func (r *Repository) GetSummary(ctx context.Context, id uuid.UUID) (domain.Summary, error) {
	row, err := r.queries.GetUserSummaryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Summary{}, domain.ErrUserNotFound
		}
		return domain.Summary{}, fmt.Errorf("get user summary: %w", err)
	}
	return summaryFrom(row.ID, row.Status, row.DisplayName, row.AvatarAssetID, row.Timezone, row.Locale), nil
}

// ListSummaries reads every summary that exists among ids, in one statement.
func (r *Repository) ListSummaries(ctx context.Context, ids []uuid.UUID) ([]domain.Summary, error) {
	rows, err := r.queries.ListUserSummariesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list user summaries: %w", err)
	}
	summaries := make([]domain.Summary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries,
			summaryFrom(row.ID, row.Status, row.DisplayName, row.AvatarAssetID, row.Timezone, row.Locale))
	}
	return summaries, nil
}

// isUniqueViolation reports whether err is a unique constraint failure on the
// named constraint. Matching the name and not just the code matters: a table
// with two unique constraints would otherwise report the wrong one, and the
// message the learner sees is chosen from it.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

func toPgDate(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *value, Valid: true}
}

func fromPgDate(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	day := value.Time
	return &day
}

func toPgTime(value *domain.TimeOfDay) pgtype.Time {
	if value == nil {
		return pgtype.Time{}
	}
	micros := int64(value.Hour)*int64(time.Hour/time.Microsecond) +
		int64(value.Minute)*int64(time.Minute/time.Microsecond)
	return pgtype.Time{Microseconds: micros, Valid: true}
}

func fromPgTime(value pgtype.Time) *domain.TimeOfDay {
	if !value.Valid {
		return nil
	}
	total := time.Duration(value.Microseconds) * time.Microsecond
	timeOfDay := domain.TimeOfDay{Hour: int(total / time.Hour), Minute: int((total % time.Hour) / time.Minute)}
	return &timeOfDay
}

func quietStart(window *domain.QuietHours) *domain.TimeOfDay {
	if window == nil {
		return nil
	}
	return &window.Start
}

func quietEnd(window *domain.QuietHours) *domain.TimeOfDay {
	if window == nil {
		return nil
	}
	return &window.End
}

func channelsToStrings(channels []domain.Channel) []string {
	values := make([]string, 0, len(channels))
	for _, channel := range channels {
		values = append(values, string(channel))
	}
	return values
}
