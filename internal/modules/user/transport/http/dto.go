package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
)

// dateLayout is the `date` format from the OpenAPI schema. Dates cross this
// boundary as strings rather than time.Time so that a value with a time
// component is a validation failure rather than something silently truncated.
const dateLayout = "2006-01-02"

// meResponse mirrors the `Me` schema in api/openapi/components/user.yaml. It is
// hand-written rather than taken from the generated models because a business
// module importing api/openapi would couple it to every other module's spec;
// the contract test in this package is what keeps the two in step.
type meResponse struct {
	ID              uuid.UUID       `json:"id"`
	Email           string          `json:"email"`
	Status          string          `json:"status"`
	EmailVerifiedAt *time.Time      `json:"email_verified_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Profile         profileResponse `json:"profile"`
}

type profileResponse struct {
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	Country     *string `json:"country"`
	Timezone    string  `json:"timezone"`
	DateOfBirth *string `json:"date_of_birth"`
}

func toMeResponse(account service.Account) meResponse {
	return meResponse{
		ID:              account.User.ID,
		Email:           account.User.Email,
		Status:          string(account.User.Status),
		EmailVerifiedAt: account.User.EmailVerifiedAt,
		CreatedAt:       account.User.CreatedAt,
		UpdatedAt:       account.User.UpdatedAt,
		Profile: profileResponse{
			DisplayName: account.Profile.DisplayName,
			// Still null until P3.1 turns the stored asset id into a URL
			// through the storage facade.
			AvatarURL:   nil,
			Country:     account.Profile.Country,
			Timezone:    account.Profile.Timezone,
			DateOfBirth: formatDate(account.Profile.DateOfBirth),
		},
	}
}

func formatDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(dateLayout)
	return &formatted
}

// preferencesResponse mirrors the `Preferences` schema.
type preferencesResponse struct {
	Locale               string             `json:"locale"`
	Theme                string             `json:"theme"`
	DailyGoalMinutes     int                `json:"daily_goal_minutes"`
	NotificationChannels []string           `json:"notification_channels"`
	QuietHours           *quietHoursPayload `json:"quiet_hours"`
	AIProcessingOptOut   bool               `json:"ai_processing_opt_out"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

type quietHoursPayload struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func toPreferencesResponse(preferences domain.Preferences) preferencesResponse {
	channels := make([]string, 0, len(preferences.NotificationChannels))
	for _, channel := range preferences.NotificationChannels {
		channels = append(channels, string(channel))
	}

	response := preferencesResponse{
		Locale:               preferences.Locale,
		Theme:                string(preferences.Theme),
		DailyGoalMinutes:     preferences.DailyGoalMinutes,
		NotificationChannels: channels,
		AIProcessingOptOut:   preferences.AIProcessingOptOut,
		UpdatedAt:            preferences.UpdatedAt,
	}
	if preferences.QuietHours != nil {
		response.QuietHours = &quietHoursPayload{
			Start: preferences.QuietHours.Start.String(),
			End:   preferences.QuietHours.End.String(),
		}
	}
	return response
}
