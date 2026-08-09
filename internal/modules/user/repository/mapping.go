package repository

import (
	"fmt"

	"github.com/google/uuid"

	sqlcuser "github.com/fluentra/fluentra/internal/generated/user/sqlc"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

// The generated row types are structurally identical across the queries that
// return the same columns, but they are distinct Go types, so each conversion
// takes the fields rather than the struct. That is verbose and it is also the
// thing that fails to compile when a query's column list changes — which is
// exactly when a silent mapping bug would otherwise appear.

func toDomainUser(row sqlcuser.CoreUser) (domain.User, error) {
	status, err := domain.ParseStatus(string(row.Status))
	if err != nil {
		return domain.User{}, fmt.Errorf("user %s has status %q: %w", row.ID, row.Status, err)
	}
	return domain.User{
		ID:              row.ID,
		Email:           row.Email,
		Status:          status,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func toDomainProfile(row sqlcuser.CoreProfile) domain.Profile {
	return domain.Profile{
		ID:            row.ID,
		UserID:        row.UserID,
		DisplayName:   row.DisplayName,
		AvatarAssetID: row.AvatarAssetID,
		Country:       row.Country,
		Timezone:      row.Timezone,
		DateOfBirth:   fromPgDate(row.DateOfBirth),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func toDomainPreferences(row sqlcuser.CoreUserPreference) (domain.Preferences, error) {
	channels := make([]domain.Channel, 0, len(row.NotificationChannels))
	for _, value := range row.NotificationChannels {
		channels = append(channels, domain.Channel(value))
	}

	preferences := domain.Preferences{
		ID:                   row.ID,
		UserID:               row.UserID,
		Locale:               row.Locale,
		Theme:                domain.Theme(row.Theme),
		DailyGoalMinutes:     int(row.DailyGoalMinutes),
		NotificationChannels: channels,
		QuietHours:           quietHoursFrom(row),
		AIProcessingOptOut:   row.AiProcessingOptOut,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
	return preferences, nil
}

// quietHoursFrom rebuilds the window. The two columns are constrained to be
// both null or both set, so a half-set pair here means the constraint was
// dropped; treating it as "no window" is the safe reading, because the
// alternative is delivering notifications into a window somebody configured.
func quietHoursFrom(row sqlcuser.CoreUserPreference) *domain.QuietHours {
	start := fromPgTime(row.QuietHoursStart)
	end := fromPgTime(row.QuietHoursEnd)
	if start == nil || end == nil {
		return nil
	}
	return &domain.QuietHours{Start: *start, End: *end}
}

// summaryFrom builds the cross-module rendering shape. The profile and
// preference columns are nullable because the query left-joins them, so each
// falls back to the same default the column would have had.
func summaryFrom(
	id uuid.UUID,
	status sqlcuser.CoreUserStatus,
	displayName *string,
	avatarAssetID *uuid.UUID,
	timezone *string,
	locale *string,
) domain.Summary {
	summary := domain.Summary{
		ID:            id,
		Status:        domain.Status(status),
		Timezone:      domain.DefaultTimezone,
		Locale:        domain.DefaultLocale,
		AvatarAssetID: avatarAssetID,
	}
	if displayName != nil {
		summary.DisplayName = *displayName
	}
	if timezone != nil {
		summary.Timezone = *timezone
	}
	if locale != nil {
		summary.Locale = *locale
	}
	return summary
}
