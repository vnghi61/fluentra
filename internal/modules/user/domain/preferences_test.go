package domain_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

// fieldNotificationChannels is asserted on by three cases below.
const fieldNotificationChannels = "notification_channels"

func validPreferences() domain.Preferences {
	return domain.Preferences{
		Locale:               "en",
		Theme:                domain.ThemeSystem,
		DailyGoalMinutes:     15,
		NotificationChannels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
	}
}

func TestPreferences_ValidateAcceptsTheDefaults(t *testing.T) {
	t.Parallel()

	if err := validPreferences().Validate(); err != nil {
		t.Fatalf("the column defaults do not pass validation: %v", err)
	}
}

func TestPreferences_ValidateRejectsEachInvariant(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*domain.Preferences)
		field  string
		code   string
	}{
		"locale is not a language tag": {
			mutate: func(p *domain.Preferences) { p.Locale = "english" },
			field:  "locale", code: "FORMAT",
		},
		"locale region is lower case": {
			mutate: func(p *domain.Preferences) { p.Locale = "vi-vn" },
			field:  "locale", code: "FORMAT",
		},
		"unknown theme": {
			mutate: func(p *domain.Preferences) { p.Theme = "solarized" },
			field:  "theme", code: "UNKNOWN",
		},
		"daily goal below the floor": {
			mutate: func(p *domain.Preferences) { p.DailyGoalMinutes = 4 },
			field:  "daily_goal_minutes", code: "OUT_OF_RANGE",
		},
		"daily goal above the ceiling": {
			mutate: func(p *domain.Preferences) { p.DailyGoalMinutes = 481 },
			field:  "daily_goal_minutes", code: "OUT_OF_RANGE",
		},
		"no channels": {
			mutate: func(p *domain.Preferences) { p.NotificationChannels = nil },
			field:  fieldNotificationChannels, code: codeRequired,
		},
		"unknown channel": {
			mutate: func(p *domain.Preferences) {
				p.NotificationChannels = []domain.Channel{"carrier_pigeon"}
			},
			field: fieldNotificationChannels, code: "UNKNOWN",
		},
		"duplicate channel": {
			mutate: func(p *domain.Preferences) {
				p.NotificationChannels = []domain.Channel{domain.ChannelPush, domain.ChannelPush}
			},
			field: fieldNotificationChannels, code: "DUPLICATE",
		},
	}

	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			preferences := validPreferences()
			testCase.mutate(&preferences)
			err := preferences.Validate()
			if err == nil {
				t.Fatalf("%s was accepted", label)
			}
			assertFieldCode(t, err, testCase.field, testCase.code)
		})
	}
}

// TestPreferences_QuietHoursMayWrapPastMidnight is the case a naive
// start-before-end check gets wrong, and it is the common case: people sleep
// across midnight.
func TestPreferences_QuietHoursMayWrapPastMidnight(t *testing.T) {
	t.Parallel()

	preferences := validPreferences()
	preferences.QuietHours = &domain.QuietHours{
		Start: domain.TimeOfDay{Hour: 22},
		End:   domain.TimeOfDay{Hour: 7},
	}
	if err := preferences.Validate(); err != nil {
		t.Fatalf("a window across midnight was rejected: %v", err)
	}
}

func TestPreferences_QuietHoursRejectsAZeroLengthWindow(t *testing.T) {
	t.Parallel()

	preferences := validPreferences()
	same := domain.TimeOfDay{Hour: 22, Minute: 30}
	preferences.QuietHours = &domain.QuietHours{Start: same, End: same}

	err := preferences.Validate()
	if err == nil {
		t.Fatal("a window whose ends are equal was accepted; it means nothing or everything")
	}
	assertFieldCode(t, err, "quiet_hours", "EMPTY_WINDOW")
}

func TestPreferences_CanonicalChannelsIsOrderIndependent(t *testing.T) {
	t.Parallel()

	preferences := validPreferences()
	preferences.NotificationChannels = []domain.Channel{domain.ChannelPush, domain.ChannelInApp}

	canonical := preferences.CanonicalChannels()
	want := []domain.Channel{domain.ChannelInApp, domain.ChannelPush}
	if len(canonical) != len(want) {
		t.Fatalf("CanonicalChannels() = %v, want %v", canonical, want)
	}
	for index := range want {
		if canonical[index] != want[index] {
			t.Fatalf("CanonicalChannels() = %v, want %v", canonical, want)
		}
	}
}

func TestParseTimeOfDay(t *testing.T) {
	t.Parallel()

	parsed, err := domain.ParseTimeOfDay("22:05")
	if err != nil {
		t.Fatalf("ParseTimeOfDay: %v", err)
	}
	if parsed.Hour != 22 || parsed.Minute != 5 {
		t.Errorf("parsed = %+v, want 22:05", parsed)
	}
	if parsed.String() != "22:05" {
		t.Errorf("String() = %q, want 22:05", parsed.String())
	}

	for _, value := range []string{"", "9:00", "24:00", "22:60", "22:00:00", "10 PM"} {
		if _, err := domain.ParseTimeOfDay(value); err == nil {
			t.Errorf("ParseTimeOfDay(%q) = nil error, want rejection", value)
		}
	}
}

func TestStatus_UsableOnlyWhenActive(t *testing.T) {
	t.Parallel()

	if !domain.StatusActive.Usable() {
		t.Error("an active account is not usable")
	}
	for _, status := range []domain.Status{
		domain.StatusSuspended, domain.StatusPendingDeletion, domain.StatusDeleted,
	} {
		if status.Usable() {
			t.Errorf("%s reports as usable", status)
		}
	}
}

func TestParseStatus_RejectsUnknownValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"active", "suspended", "pending_deletion", "deleted"} {
		if _, err := domain.ParseStatus(value); err != nil {
			t.Errorf("ParseStatus(%q) = %v, want nil", value, err)
		}
	}
	// A value the database can hold that this package does not know is a bug
	// in one of the two; a zero-value Status would hide it.
	if _, err := domain.ParseStatus("archived"); err == nil {
		t.Error("ParseStatus(\"archived\") = nil error, want rejection")
	}
}
