package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Accounts is the service surface these handlers need. Every method takes the
// actor's id as its first argument and there is no method that takes a
// different target id — the interface itself is where "you can only read
// yourself" is enforced, so no handler can get it wrong.
type Accounts interface {
	GetAccount(ctx context.Context, actorID uuid.UUID) (service.Account, error)
	UpdateProfile(ctx context.Context, actorID uuid.UUID, change domain.ProfileChange) (service.Account, error)
	GetPreferences(ctx context.Context, actorID uuid.UUID) (domain.Preferences, error)
	ReplacePreferences(
		ctx context.Context, actorID uuid.UUID, wanted domain.Preferences,
	) (domain.Preferences, error)
	RequestAvatarUploadIntent(ctx context.Context, actorID uuid.UUID, contentType string) (domain.UploadIntent, error)
	ConfirmAvatar(ctx context.Context, actorID uuid.UUID, objectKey string) (service.Account, error)
}

// Handler serves the user module's HTTP operations.
type Handler struct {
	accounts Accounts
}

// NewHandler creates the handler.
func NewHandler(accounts Accounts) *Handler { return &Handler{accounts: accounts} }

// Routes mounts this module's operations on router. The paths are relative to
// wherever the composition root mounts it, which is /api/v1.
func (h *Handler) Routes(router chi.Router) {
	router.Route("/me", func(me chi.Router) {
		me.Get("/", h.getMe)
		me.Patch("/", h.updateMe)
		me.Get("/preferences", h.getPreferences)
		me.Put("/preferences", h.replacePreferences)
		me.Post("/avatar/upload-intent", h.requestAvatarUploadIntent)
		me.Put("/avatar", h.confirmAvatar)
	})
}

// The preference and avatar members named in more than one of the lists below.
const (
	fieldAIProcessingOptOut = "ai_processing_opt_out"
	fieldDailyGoalMinutes   = "daily_goal_minutes"
	fieldLocale             = "locale"
	fieldChannels           = "notification_channels"
	fieldTheme              = "theme"
	fieldObjectKey          = "object_key"
)

// updateMeFields is the complete set of members PATCH /me accepts. Anything
// else is a 422 naming the field.
var updateMeFields = []string{"display_name", "country", "timezone", "date_of_birth"}

// replacePreferencesFields is the complete set for PUT /me/preferences. Every
// one but quiet_hours is required, because this is a replacement.
var (
	replacePreferencesFields = []string{
		fieldLocale, fieldTheme, fieldDailyGoalMinutes, fieldChannels, "quiet_hours",
		fieldAIProcessingOptOut,
	}
	requiredPreferencesFields = []string{
		fieldLocale, fieldTheme, fieldDailyGoalMinutes, fieldChannels, fieldAIProcessingOptOut,
	}
)

func (h *Handler) getMe(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	account, err := h.accounts.GetAccount(request.Context(), actor.UserID)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toMeResponse(account))
}

func (h *Handler) updateMe(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	change, err := decodeProfileChange(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	account, err := h.accounts.UpdateProfile(request.Context(), actor.UserID, change)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toMeResponse(account))
}

func (h *Handler) getPreferences(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	preferences, err := h.accounts.GetPreferences(request.Context(), actor.UserID)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toPreferencesResponse(preferences))
}

func (h *Handler) replacePreferences(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	wanted, err := decodePreferences(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	stored, err := h.accounts.ReplacePreferences(request.Context(), actor.UserID, wanted)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	httpx.WriteJSON(writer, request, http.StatusOK, toPreferencesResponse(stored))
}

// requireActor reads the caller from the request context.
//
// This is the whole of the "impossible by construction" claim: the actor comes
// from the token that middleware verified, the routes carry no user id, and
// there is no code path that reads one from the request. Changing a path
// segment cannot reach another account because there is no path segment.
func requireActor(request *http.Request) (httpx.Actor, error) {
	actor, ok := httpx.ActorFrom(request.Context())
	if !ok {
		return httpx.Actor{}, apperr.New(
			apperr.Unauthenticated, "UNAUTHENTICATED", "Provide a valid access token.")
	}
	return actor, nil
}

func decodeProfileChange(request *http.Request) (domain.ProfileChange, error) {
	fields, err := decodeBody(request, updateMeFields)
	if err != nil {
		return domain.ProfileChange{}, err
	}
	if err := rejectNulls(fields, updateMeFields); err != nil {
		return domain.ProfileChange{}, err
	}

	var change domain.ProfileChange
	if fields.present("display_name") {
		var displayName string
		if err := readString(fields, "display_name", &displayName); err != nil {
			return domain.ProfileChange{}, err
		}
		change.DisplayName = &displayName
	}
	if fields.present("country") {
		var country string
		if err := readString(fields, "country", &country); err != nil {
			return domain.ProfileChange{}, err
		}
		change.Country = &country
	}
	if fields.present("timezone") {
		var timezone string
		if err := readString(fields, "timezone", &timezone); err != nil {
			return domain.ProfileChange{}, err
		}
		change.Timezone = &timezone
	}
	if fields.present("date_of_birth") {
		dateOfBirth, err := readDate(fields, "date_of_birth")
		if err != nil {
			return domain.ProfileChange{}, err
		}
		change.DateOfBirth = dateOfBirth
	}
	return change, nil
}

func readDate(fields body, name string) (*time.Time, error) {
	var raw string
	if err := readString(fields, name, &raw); err != nil {
		return nil, err
	}
	parsed, err := time.Parse(dateLayout, raw)
	if err != nil {
		return nil, validationFailed().WithFields(apperr.FieldViolation{
			Field: name, Code: "FORMAT", Message: name + " must be a date in YYYY-MM-DD form.",
		})
	}
	return &parsed, nil
}

func decodePreferences(request *http.Request) (domain.Preferences, error) {
	fields, err := decodeBody(request, replacePreferencesFields)
	if err != nil {
		return domain.Preferences{}, err
	}
	if err := rejectNulls(fields, requiredPreferencesFields); err != nil {
		return domain.Preferences{}, err
	}
	if err := requireFields(fields, requiredPreferencesFields); err != nil {
		return domain.Preferences{}, err
	}

	var payload struct {
		Locale               string   `json:"locale"`
		Theme                string   `json:"theme"`
		DailyGoalMinutes     int      `json:"daily_goal_minutes"`
		NotificationChannels []string `json:"notification_channels"`
		AIProcessingOptOut   bool     `json:"ai_processing_opt_out"`
	}
	for name, target := range map[string]any{
		fieldLocale:             &payload.Locale,
		fieldTheme:              &payload.Theme,
		fieldDailyGoalMinutes:   &payload.DailyGoalMinutes,
		fieldChannels:           &payload.NotificationChannels,
		fieldAIProcessingOptOut: &payload.AIProcessingOptOut,
	} {
		if err := readInto(fields, name, target); err != nil {
			return domain.Preferences{}, err
		}
	}

	channels := make([]domain.Channel, 0, len(payload.NotificationChannels))
	for _, channel := range payload.NotificationChannels {
		channels = append(channels, domain.Channel(channel))
	}

	preferences := domain.Preferences{
		Locale:               payload.Locale,
		Theme:                domain.Theme(payload.Theme),
		DailyGoalMinutes:     payload.DailyGoalMinutes,
		NotificationChannels: channels,
		AIProcessingOptOut:   payload.AIProcessingOptOut,
	}

	quietHours, err := decodeQuietHours(fields)
	if err != nil {
		return domain.Preferences{}, err
	}
	preferences.QuietHours = quietHours
	return preferences, nil
}

// decodeQuietHours reads the optional window. Absent and explicitly null both
// mean "no quiet hours": unlike the profile fields this one is nullable in the
// schema, and for a replacement there is no "leave it alone" to confuse it with.
func decodeQuietHours(fields body) (*domain.QuietHours, error) {
	if !fields.present("quiet_hours") {
		return nil, nil
	}
	var payload quietHoursPayload
	if err := readInto(fields, "quiet_hours", &payload); err != nil {
		return nil, err
	}
	start, err := domain.ParseTimeOfDay(payload.Start)
	if err != nil {
		return nil, err
	}
	end, err := domain.ParseTimeOfDay(payload.End)
	if err != nil {
		return nil, err
	}
	return &domain.QuietHours{Start: start, End: end}, nil
}

func (h *Handler) requestAvatarUploadIntent(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	var contentType string
	if request.Body != nil && request.ContentLength > 0 {
		fields, decodeErr := decodeBody(request, []string{"content_type"})
		if decodeErr == nil && fields.present("content_type") {
			_ = readString(fields, "content_type", &contentType)
		}
	}

	intent, err := h.accounts.RequestAvatarUploadIntent(request.Context(), actor.UserID, contentType)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	httpx.WriteJSON(writer, request, http.StatusOK, toAvatarUploadIntentResponse(intent))
}

func (h *Handler) confirmAvatar(writer http.ResponseWriter, request *http.Request) {
	actor, err := requireActor(request)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	fields, err := decodeBody(request, []string{fieldObjectKey})
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	if err := rejectNulls(fields, []string{fieldObjectKey}); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}
	if err := requireFields(fields, []string{fieldObjectKey}); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	var objectKey string
	if err := readString(fields, fieldObjectKey, &objectKey); err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	account, err := h.accounts.ConfirmAvatar(request.Context(), actor.UserID, objectKey)
	if err != nil {
		httpx.WriteProblem(writer, request, err)
		return
	}

	httpx.WriteJSON(writer, request, http.StatusOK, toMeResponse(account))
}
