package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Theme is the UI appearance setting, mirroring core.ui_theme.
type Theme string

// The complete set of themes.
const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeSystem Theme = "system"
)

// Channel is a notification delivery route.
type Channel string

// The complete set of channels. The database check constraint holds the same
// list; adding one means changing both, which is deliberate.
const (
	ChannelInApp Channel = "in_app"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
)

// Preference bounds, matching the check constraints on core.user_preferences.
const (
	MinDailyGoalMinutes = 5
	MaxDailyGoalMinutes = 480
)

// DefaultLocale is the fallback language, matching the column default.
const DefaultLocale = "en"

var localePattern = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

// TimeOfDay is a wall-clock time with no date and no zone. Quiet hours are
// local to the learner, so binding them to an instant would be wrong: 22:00
// means 22:00 wherever they are, including after they travel.
type TimeOfDay struct {
	Hour   int
	Minute int
}

// timeOfDayPattern is the `HH:MM` form, matching the pattern in the OpenAPI
// schema. The pattern is checked before time.Parse rather than relying on it:
// time.Parse("15:04", "9:00") succeeds, so the parser alone would accept a
// value the published schema says is invalid.
var timeOfDayPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// ParseTimeOfDay reads the `HH:MM` form the API uses.
func ParseTimeOfDay(value string) (TimeOfDay, error) {
	if !timeOfDayPattern.MatchString(value) {
		return TimeOfDay{}, invalid("quiet_hours", "FORMAT", "Quiet hours must be times in HH:MM form.")
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return TimeOfDay{}, invalid("quiet_hours", "FORMAT", "Quiet hours must be times in HH:MM form.")
	}
	return TimeOfDay{Hour: parsed.Hour(), Minute: parsed.Minute()}, nil
}

// String renders the `HH:MM` form.
func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute) }

// QuietHours is a window in which no notification is delivered. A window that
// wraps past midnight is normal and valid; a window whose ends are equal is
// not, because it describes either nothing or everything depending on how it
// is read.
type QuietHours struct {
	Start TimeOfDay
	End   TimeOfDay
}

// Validate rejects the one ambiguous window.
func (q QuietHours) Validate() error {
	if q.Start == q.End {
		return invalid("quiet_hours", "EMPTY_WINDOW", "Quiet hours must start and end at different times.")
	}
	return nil
}

// Preferences is the settings record. It is replaced as a whole, never patched,
// so there is no Change type here to mirror ProfileChange.
type Preferences struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	Locale               string
	Theme                Theme
	DailyGoalMinutes     int
	NotificationChannels []Channel
	QuietHours           *QuietHours
	AIProcessingOptOut   bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Validate enforces every preference invariant. Unlike ProfileChange this is a
// full-resource check, because a PUT that omitted a field never reaches here.
func (p Preferences) Validate() error {
	if err := validateLocale(p.Locale); err != nil {
		return err
	}
	if err := validateTheme(p.Theme); err != nil {
		return err
	}
	if p.DailyGoalMinutes < MinDailyGoalMinutes || p.DailyGoalMinutes > MaxDailyGoalMinutes {
		return invalid("daily_goal_minutes", "OUT_OF_RANGE",
			fmt.Sprintf("Daily goal must be between %d and %d minutes.", MinDailyGoalMinutes, MaxDailyGoalMinutes))
	}
	if err := validateChannels(p.NotificationChannels); err != nil {
		return err
	}
	if p.QuietHours != nil {
		return p.QuietHours.Validate()
	}
	return nil
}

func validateLocale(locale string) error {
	if !localePattern.MatchString(locale) {
		return invalid("locale", "FORMAT", "Locale must be a language tag such as en or vi-VN.")
	}
	return nil
}

func validateTheme(theme Theme) error {
	switch theme {
	case ThemeLight, ThemeDark, ThemeSystem:
		return nil
	default:
		return invalid("theme", "UNKNOWN", "Theme must be light, dark or system.")
	}
}

// validateChannels rejects unknown and duplicate entries. Duplicates matter:
// the column is an array, not a set, and a channel listed twice would be a
// notification delivered twice by any consumer that iterates it.
func validateChannels(channels []Channel) error {
	if len(channels) == 0 {
		return invalid("notification_channels", "REQUIRED", "Select at least one notification channel.")
	}
	seen := make(map[Channel]struct{}, len(channels))
	for _, channel := range channels {
		switch channel {
		case ChannelInApp, ChannelEmail, ChannelPush:
		default:
			return invalid("notification_channels", "UNKNOWN",
				"Notification channels must be in_app, email or push.")
		}
		if _, duplicate := seen[channel]; duplicate {
			return invalid("notification_channels", "DUPLICATE", "Each notification channel may appear once.")
		}
		seen[channel] = struct{}{}
	}
	return nil
}

// CanonicalChannels returns the channels in the declared order, so that a
// stored value does not depend on the order the client happened to send.
func (p Preferences) CanonicalChannels() []Channel {
	order := []Channel{ChannelInApp, ChannelEmail, ChannelPush}
	canonical := make([]Channel, 0, len(p.NotificationChannels))
	for _, candidate := range order {
		if slices.Contains(p.NotificationChannels, candidate) {
			canonical = append(canonical, candidate)
		}
	}
	return canonical
}

// trimSpace is shared by the profile validators. It lives here rather than
// being called inline so that "what counts as blank" has one answer.
func trimSpace(value string) string { return strings.TrimSpace(value) }
