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

// CreateExportRequest inserts a new export record.
func (r *Repository) CreateExportRequest(ctx context.Context, id, userID uuid.UUID) (domain.ExportRequest, error) {
	row, err := r.queries.CreateExportRequest(ctx, sqlcuser.CreateExportRequestParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return domain.ExportRequest{}, fmt.Errorf("create export request: %w", err)
	}
	return toDomainExport(row), nil
}

// GetPendingExportForUser finds an active export request in pending or processing state.
func (r *Repository) GetPendingExportForUser(
	ctx context.Context, userID uuid.UUID,
) (domain.ExportRequest, bool, error) {
	row, err := r.queries.GetPendingExportForUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ExportRequest{}, false, nil
		}
		return domain.ExportRequest{}, false, fmt.Errorf("get pending export for user: %w", err)
	}
	return toDomainExport(row), true, nil
}

// GetExportByID retrieves an export request by ID.
func (r *Repository) GetExportByID(ctx context.Context, id uuid.UUID) (domain.ExportRequest, error) {
	row, err := r.queries.GetExportByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ExportRequest{}, domain.ErrUserNotFound
		}
		return domain.ExportRequest{}, fmt.Errorf("get export by id: %w", err)
	}
	return toDomainExport(row), nil
}

// UpdateExportStatus updates the state and timestamps of an export request.
func (r *Repository) UpdateExportStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.ExportStatus,
	startedAt, completedAt, expiresAt *time.Time,
	objectKey, errorMessage *string,
) error {
	err := r.queries.UpdateExportStatus(ctx, sqlcuser.UpdateExportStatusParams{
		ID:           id,
		Status:       string(status),
		StartedAt:    startedAt,
		CompletedAt:  completedAt,
		ExpiresAt:    expiresAt,
		ObjectKey:    objectKey,
		ErrorMessage: errorMessage,
	})
	if err != nil {
		return fmt.Errorf("update export status: %w", err)
	}
	return nil
}

// GetExpiredExports lists completed exports whose retention has expired.
func (r *Repository) GetExpiredExports(ctx context.Context, limit int32) ([]domain.ExportRequest, error) {
	rows, err := r.queries.GetExpiredExports(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("get expired exports: %w", err)
	}
	items := make([]domain.ExportRequest, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainExport(row))
	}
	return items, nil
}

// DeleteExport removes an export record.
func (r *Repository) DeleteExport(ctx context.Context, id uuid.UUID) error {
	if err := r.queries.DeleteExport(ctx, id); err != nil {
		return fmt.Errorf("delete export: %w", err)
	}
	return nil
}

// CreateDeletionRequest inserts a new deletion record.
func (r *Repository) CreateDeletionRequest(
	ctx context.Context, id, userID uuid.UUID, executeAt time.Time,
) (domain.DeletionRequest, error) {
	row, err := r.queries.CreateDeletionRequest(ctx, sqlcuser.CreateDeletionRequestParams{
		ID:          id,
		UserID:      userID,
		Status:      string(domain.DeletionStatusPending),
		RequestedAt: time.Now().UTC(),
		ExecuteAt:   executeAt,
	})
	if err != nil {
		return domain.DeletionRequest{}, fmt.Errorf("create deletion request: %w", err)
	}
	return toDomainDeletion(row), nil
}

// GetPendingDeletionForUser finds an active deletion request in pending or processing state.
func (r *Repository) GetPendingDeletionForUser(
	ctx context.Context, userID uuid.UUID,
) (domain.DeletionRequest, bool, error) {
	row, err := r.queries.GetPendingDeletionByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DeletionRequest{}, false, nil
		}
		return domain.DeletionRequest{}, false, fmt.Errorf("get pending deletion for user: %w", err)
	}
	return toDomainDeletion(row), true, nil
}

// GetDeletionByID reads a deletion record by ID.
func (r *Repository) GetDeletionByID(ctx context.Context, id uuid.UUID) (domain.DeletionRequest, error) {
	row, err := r.queries.GetDeletionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DeletionRequest{}, domain.ErrDeletionNotFound
		}
		return domain.DeletionRequest{}, fmt.Errorf("get deletion by id: %w", err)
	}
	return toDomainDeletion(row), nil
}

// CancelDeletion marks a pending deletion request as cancelled.
func (r *Repository) CancelDeletion(ctx context.Context, id uuid.UUID, cancelledAt time.Time) error {
	_, err := r.queries.CancelDeletion(ctx, sqlcuser.CancelDeletionParams{
		ID:          id,
		CancelledAt: &cancelledAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrDeletionNotCancellable
		}
		return fmt.Errorf("cancel deletion: %w", err)
	}
	return nil
}

// UpdateDeletionStatus updates lifecycle timestamps and error state on a deletion request.
func (r *Repository) UpdateDeletionStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.DeletionStatus,
	startedAt, completedAt *time.Time,
	errorMessage *string,
) error {
	err := r.queries.UpdateDeletionStatus(ctx, sqlcuser.UpdateDeletionStatusParams{
		ID:           id,
		Status:       string(status),
		StartedAt:    startedAt,
		CompletedAt:  completedAt,
		ErrorMessage: errorMessage,
	})
	if err != nil {
		return fmt.Errorf("update deletion status: %w", err)
	}
	return nil
}

// GetDueDeletions returns pending deletions whose execute_at timestamp is at or before cutoff.
func (r *Repository) GetDueDeletions(
	ctx context.Context, cutoff time.Time, limit int32,
) ([]domain.DeletionRequest, error) {
	rows, err := r.queries.GetDueDeletions(ctx, sqlcuser.GetDueDeletionsParams{
		ExecuteAt: cutoff,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get due deletions: %w", err)
	}
	items := make([]domain.DeletionRequest, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainDeletion(row))
	}
	return items, nil
}

// AnonymiseUser rewrites the email and sets status to deleted.
func (r *Repository) AnonymiseUser(ctx context.Context, userID uuid.UUID, anonymisedEmail string) error {
	err := r.queries.AnonymiseUser(ctx, sqlcuser.AnonymiseUserParams{
		ID:    userID,
		Email: anonymisedEmail,
	})
	if err != nil {
		return fmt.Errorf("anonymise user: %w", err)
	}
	return nil
}

// AnonymiseProfile clears personal profile details.
func (r *Repository) AnonymiseProfile(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.AnonymiseProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("anonymise profile: %w", err)
	}
	return nil
}

// DeletePreferences removes the user_preferences row.
func (r *Repository) DeletePreferences(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.DeleteUserPreferences(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete user preferences: %w", err)
	}
	return nil
}

// DeleteLearningProfile removes the learning_profiles row.
func (r *Repository) DeleteLearningProfile(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.DeleteLearningProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete learning profile: %w", err)
	}
	return nil
}

// UpdateUserStatus changes the user's status.
func (r *Repository) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status domain.Status) error {
	_, err := r.queries.UpdateUserStatus(ctx, sqlcuser.UpdateUserStatusParams{
		ID:     userID,
		Status: sqlcuser.CoreUserStatus(status),
	})
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	return nil
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

// SearchAdminRow is the raw search row returned for admin search queries.
type SearchAdminRow struct {
	ID          uuid.UUID
	Email       string
	Status      domain.Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DisplayName string
	Locale      string
	Timezone    string
}

// UserFilterParams carries filter fields for user search at repository layer.
type UserFilterParams struct {
	EmailPrefix   *string
	DisplayName   *string
	Status        *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

// SearchUsersAdmin performs cursor-paginated user searches for admin screens.
func (r *Repository) SearchUsersAdmin(
	ctx context.Context,
	filter UserFilterParams,
	cursorID *uuid.UUID,
	cursorTime *time.Time,
	limit int,
) ([]SearchAdminRow, error) {
	var emailPrefix, displayName, status string
	if filter.EmailPrefix != nil {
		emailPrefix = *filter.EmailPrefix
	}
	if filter.DisplayName != nil {
		displayName = *filter.DisplayName
	}
	if filter.Status != nil {
		status = *filter.Status
	}
	var createdAfter, createdBefore time.Time
	if filter.CreatedAfter != nil {
		createdAfter = *filter.CreatedAfter
	}
	if filter.CreatedBefore != nil {
		createdBefore = *filter.CreatedBefore
	}
	var cID uuid.UUID
	if cursorID != nil {
		cID = *cursorID
	}
	var cTime time.Time
	if cursorTime != nil {
		cTime = *cursorTime
	}

	rows, err := r.queries.SearchUsersAdmin(ctx, sqlcuser.SearchUsersAdminParams{
		EmailPrefix:     emailPrefix,
		DisplayName:     displayName,
		Status:          status,
		CreatedAfter:    createdAfter,
		CreatedBefore:   createdBefore,
		CursorID:        cID,
		CursorCreatedAt: cTime,
		ResultLimit:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search users admin: %w", err)
	}

	result := make([]SearchAdminRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, SearchAdminRow{
			ID:          row.ID,
			Email:       row.Email,
			Status:      domain.Status(row.Status),
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
			DisplayName: row.DisplayName,
			Locale:      row.Locale,
			Timezone:    row.Timezone,
		})
	}
	return result, nil
}

