package domain_test

import (
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

// TestValidateTimezone_AcceptsIANANames is BR-USER-03. The point of the test
// is not that these strings are spelled right — it is that time.LoadLocation
// resolves them from the tzdata compiled into the binary, so the rule behaves
// the same in a scratch container as on a developer machine.
func TestValidateTimezone_AcceptsIANANames(t *testing.T) {
	t.Parallel()

	for _, timezone := range []string{
		"UTC",
		timezoneHoChiMinh,
		"Europe/London",
		"America/Argentina/Buenos_Aires",
		"Pacific/Chatham",
		"Australia/Lord_Howe",
	} {
		t.Run(timezone, func(t *testing.T) {
			t.Parallel()
			if err := domain.ValidateTimezone(timezone); err != nil {
				t.Errorf("ValidateTimezone(%q) = %v, want nil", timezone, err)
			}
		})
	}
}

func TestValidateTimezone_RejectsAnythingElse(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		timezone string
		code     string
	}{
		"empty":             {timezone: "", code: codeRequired},
		"offset":            {timezone: "+07:00", code: codeNotIANA},
		"abbreviation":      {timezone: "ICT", code: codeNotIANA},
		"windows name":      {timezone: "SE Asia Standard Time", code: codeNotIANA},
		"misspelled":        {timezone: "Asia/Ho_Chi_Min", code: codeNotIANA},
		"path traversal":    {timezone: "../../etc/passwd", code: codeNotIANA},
		"server local time": {timezone: "Local", code: codeNotIANA},
	}
	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			err := domain.ValidateTimezone(testCase.timezone)
			if err == nil {
				t.Fatalf("ValidateTimezone(%q) = nil, want rejection", testCase.timezone)
			}
			assertFieldCode(t, err, "timezone", testCase.code)
		})
	}
}

func TestValidateCountry(t *testing.T) {
	t.Parallel()

	for _, country := range []string{"VN", "GB", "US"} {
		if err := domain.ValidateCountry(country); err != nil {
			t.Errorf("ValidateCountry(%q) = %v, want nil", country, err)
		}
	}
	for _, country := range []string{"vn", "VNM", "V", "", "V1", "  "} {
		err := domain.ValidateCountry(country)
		if err == nil {
			t.Errorf("ValidateCountry(%q) = nil, want rejection", country)
			continue
		}
		assertFieldCode(t, err, "country", "FORMAT")
	}
}

func TestValidateDateOfBirth(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

	if err := domain.ValidateDateOfBirth(time.Date(1998, time.March, 4, 0, 0, 0, 0, time.UTC), now); err != nil {
		t.Errorf("a plausible date was rejected: %v", err)
	}
	// Today is allowed: a newborn is a valid, if unlikely, learner, and the
	// age gate handles what that means.
	if err := domain.ValidateDateOfBirth(now, now); err != nil {
		t.Errorf("today was rejected: %v", err)
	}

	tomorrow := now.AddDate(0, 0, 1)
	err := domain.ValidateDateOfBirth(tomorrow, now)
	if err == nil {
		t.Fatal("a future date of birth was accepted")
	}
	assertFieldCode(t, err, "date_of_birth", "IN_FUTURE")

	err = domain.ValidateDateOfBirth(time.Date(1899, time.January, 1, 0, 0, 0, 0, time.UTC), now)
	if err == nil {
		t.Fatal("an implausibly old date of birth was accepted")
	}
	assertFieldCode(t, err, "date_of_birth", "OUT_OF_RANGE")
}

func TestProfileChange_ChangedFieldsIsStableAndValueFree(t *testing.T) {
	t.Parallel()

	name := nameNghi
	timezone := "UTC"
	change := domain.ProfileChange{DisplayName: &name, Timezone: &timezone}

	got := change.ChangedFields()
	want := []string{"display_name", "timezone"}
	if len(got) != len(want) {
		t.Fatalf("ChangedFields() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("ChangedFields() = %v, want %v", got, want)
		}
	}

	// The audit trail records this list. If a value ever appears in it, the
	// audit log has become a second copy of the personal data it is meant to
	// describe.
	for _, field := range got {
		if field == name || field == timezone {
			t.Errorf("ChangedFields() leaked a value: %q", field)
		}
	}
}

func TestProfileChange_EmptyIsRejected(t *testing.T) {
	t.Parallel()

	err := domain.ProfileChange{}.Validate(time.Now())
	if err == nil {
		t.Fatal("an empty change was accepted; it would write nothing and report success")
	}
	assertFieldCode(t, err, "", "EMPTY")
}

// TestProfileChange_NormalisedTrimsAndTruncates checks that what validation
// saw is what persistence gets. A name that passes because it was trimmed and
// is then stored untrimmed would violate the length check in the database.
func TestProfileChange_NormalisedTrimsAndTruncates(t *testing.T) {
	t.Parallel()

	padded := "  Nghi  "
	withTime := time.Date(1998, time.March, 4, 13, 45, 0, 0, time.UTC)
	normalised := domain.ProfileChange{DisplayName: &padded, DateOfBirth: &withTime}.Normalised()

	if *normalised.DisplayName != nameNghi {
		t.Errorf("display name = %q, want it trimmed", *normalised.DisplayName)
	}
	if normalised.DateOfBirth.Hour() != 0 || normalised.DateOfBirth.Minute() != 0 {
		t.Errorf("date of birth = %s, want the time component dropped", normalised.DateOfBirth)
	}
}
