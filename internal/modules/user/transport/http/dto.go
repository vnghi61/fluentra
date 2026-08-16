package http

import (
	"fmt"
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
	var avatarURL *string
	if account.Profile.AvatarAssetID != nil {
		url := fmt.Sprintf("/api/v1/storage/avatars/%s", account.Profile.AvatarAssetID.String())
		avatarURL = &url
	}

	return meResponse{
		ID:              account.User.ID,
		Email:           account.User.Email,
		Status:          string(account.User.Status),
		EmailVerifiedAt: account.User.EmailVerifiedAt,
		CreatedAt:       account.User.CreatedAt,
		UpdatedAt:       account.User.UpdatedAt,
		Profile: profileResponse{
			DisplayName: account.Profile.DisplayName,
			AvatarURL:   avatarURL,
			Country:     account.Profile.Country,
			Timezone:    account.Profile.Timezone,
			DateOfBirth: formatDate(account.Profile.DateOfBirth),
		},
	}
}

type avatarUploadIntentResponse struct {
	UploadURL   string            `json:"upload_url"`
	Method      string            `json:"method"`
	FormData    map[string]string `json:"form_data,omitempty"`
	FileField   string            `json:"file_field,omitempty"`
	ObjectKey   string            `json:"object_key"`
	ExpiresAt   time.Time         `json:"expires_at"`
	MaxBytes    int64             `json:"max_bytes"`
	ContentType string            `json:"content_type"`
}

func toAvatarUploadIntentResponse(intent domain.UploadIntent) avatarUploadIntentResponse {
	return avatarUploadIntentResponse{
		UploadURL:   intent.URL,
		Method:      intent.Method,
		FormData:    intent.FormData,
		FileField:   intent.FileField,
		ObjectKey:   intent.ObjectKey,
		ExpiresAt:   intent.ExpiresAt,
		MaxBytes:    intent.MaxBytes,
		ContentType: intent.ContentType,
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

// exportResponse mirrors the ExportRequest schema in OpenAPI.
type exportResponse struct {
	ID          uuid.UUID           `json:"id"`
	Status      domain.ExportStatus `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	StartedAt   *time.Time          `json:"started_at,omitempty"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
}

func toExportResponse(req domain.ExportRequest) exportResponse {
	return exportResponse{
		ID:          req.ID,
		Status:      req.Status,
		CreatedAt:   req.CreatedAt,
		StartedAt:   req.StartedAt,
		CompletedAt: req.CompletedAt,
		ExpiresAt:   req.ExpiresAt,
	}
}

// deletionResponse mirrors the DeletionResponse schema in OpenAPI.
type deletionResponse struct {
	ID          uuid.UUID             `json:"id"`
	UserID      uuid.UUID             `json:"user_id"`
	Status      domain.DeletionStatus `json:"status"`
	RequestedAt time.Time             `json:"requested_at"`
	ExecuteAt   time.Time             `json:"execute_at"`
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	CancelledAt *time.Time            `json:"cancelled_at,omitempty"`
}

func toDeletionResponse(req domain.DeletionRequest) deletionResponse {
	return deletionResponse{
		ID:          req.ID,
		UserID:      req.UserID,
		Status:      req.Status,
		RequestedAt: req.RequestedAt,
		ExecuteAt:   req.ExecuteAt,
		StartedAt:   req.StartedAt,
		CompletedAt: req.CompletedAt,
		CancelledAt: req.CancelledAt,
	}
}
