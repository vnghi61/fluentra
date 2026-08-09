package domain

import (
	"regexp"
	"time"

	// tzdata is embedded rather than read from the host. BR-USER-03 says a
	// timezone must be a valid IANA name, and time.LoadLocation reads the
	// operating system's zoneinfo — which a scratch or distroless container
	// does not have. Without this import the same request validates on a
	// developer's machine and 500s in production, which is the worst possible
	// place to discover a validation rule is environment-dependent.
	_ "time/tzdata"

	"github.com/google/uuid"
)

// Profile is the descriptive half of an account: everything a learner can
// change about how they appear, and nothing that authenticates them.
type Profile struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	DisplayName   string
	AvatarAssetID *uuid.UUID
	Country       *string
	Timezone      string
	DateOfBirth   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DefaultTimezone is what a profile gets before the learner picks one. It
// matches the column default, so a row created by either side agrees.
const DefaultTimezone = "UTC"

// countryPattern is ISO 3166-1 alpha-2, upper case, matching the check
// constraint on core.profiles.country.
var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// earliestDateOfBirth bounds the age gate below. The database check stops at
// the same date; this exists so the client is told which field was wrong.
var earliestDateOfBirth = time.Date(1900, time.January, 2, 0, 0, 0, 0, time.UTC)

// ValidateCountry enforces the alpha-2 shape.
func ValidateCountry(country string) error {
	if !countryPattern.MatchString(country) {
		return invalid("country", "FORMAT", "Country must be a two-letter ISO 3166-1 alpha-2 code.")
	}
	return nil
}

// ValidateTimezone enforces BR-USER-03: a real IANA name, resolved against the
// tzdata compiled into this binary.
func ValidateTimezone(timezone string) error {
	if timezone == "" {
		return invalid("timezone", "REQUIRED", "Timezone is required.")
	}
	// "Local" resolves successfully and means "whatever the server is set to",
	// which is not a learner's timezone and would silently change under them
	// when the API moves host.
	if timezone == "Local" {
		return invalid("timezone", "NOT_IANA", "Timezone must be an IANA name, for example Asia/Ho_Chi_Minh.")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return invalid("timezone", "NOT_IANA", "Timezone must be an IANA name, for example Asia/Ho_Chi_Minh.")
	}
	return nil
}

// ValidateDateOfBirth bounds the value at both ends. The upper bound is here
// rather than in the database because a CHECK constraint cannot call now()
// and stay meaningful: it would be evaluated once, at insert.
func ValidateDateOfBirth(dateOfBirth, now time.Time) error {
	day := truncateToDay(dateOfBirth)
	if !day.After(earliestDateOfBirth) {
		return invalid("date_of_birth", "OUT_OF_RANGE", "Date of birth is not a plausible date.")
	}
	if day.After(truncateToDay(now)) {
		return invalid("date_of_birth", "IN_FUTURE", "Date of birth cannot be in the future.")
	}
	return nil
}

// truncateToDay drops the time component so a date-only value compares the way
// a reader expects regardless of the location it arrived in.
func truncateToDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

// ProfileChange is a partial update. A nil field was not supplied and must be
// left alone; that distinction is the whole difference between PATCH and PUT,
// and collapsing it is how a partial update quietly erases data.
type ProfileChange struct {
	DisplayName *string
	Country     *string
	Timezone    *string
	DateOfBirth *time.Time
}

// IsEmpty reports whether the change would write nothing.
func (c ProfileChange) IsEmpty() bool {
	return c.DisplayName == nil && c.Country == nil && c.Timezone == nil && c.DateOfBirth == nil
}

// ChangedFields lists the field names the change touches, in a stable order.
// It is what the `user.profile_updated` event carries, and therefore what the
// audit trail records: the names of what changed, never the values, because
// the audit log must not become a second copy of the personal data.
func (c ProfileChange) ChangedFields() []string {
	fields := make([]string, 0, 4)
	if c.DisplayName != nil {
		fields = append(fields, "display_name")
	}
	if c.Country != nil {
		fields = append(fields, "country")
	}
	if c.Timezone != nil {
		fields = append(fields, "timezone")
	}
	if c.DateOfBirth != nil {
		fields = append(fields, "date_of_birth")
	}
	return fields
}

// Validate checks every supplied field. It stops at the first failure: the
// fields are independent, so reporting them all would be better, but the
// database applies its constraints in an unspecified order anyway and one
// clear violation is more actionable than an arbitrary subset.
func (c ProfileChange) Validate(now time.Time) error {
	if c.IsEmpty() {
		return invalid("", "EMPTY", "Supply at least one field to update.")
	}
	if c.DisplayName != nil {
		if err := ValidateDisplayName(*c.DisplayName); err != nil {
			return err
		}
	}
	if c.Country != nil {
		if err := ValidateCountry(*c.Country); err != nil {
			return err
		}
	}
	if c.Timezone != nil {
		if err := ValidateTimezone(*c.Timezone); err != nil {
			return err
		}
	}
	if c.DateOfBirth != nil {
		if err := ValidateDateOfBirth(*c.DateOfBirth, now); err != nil {
			return err
		}
	}
	return nil
}

// Normalised returns the change with values put into the form the database
// stores, so that validation and persistence cannot disagree about what a
// value is.
func (c ProfileChange) Normalised() ProfileChange {
	normalised := c
	if c.DisplayName != nil {
		trimmed := trimSpace(*c.DisplayName)
		normalised.DisplayName = &trimmed
	}
	if c.DateOfBirth != nil {
		day := truncateToDay(*c.DateOfBirth)
		normalised.DateOfBirth = &day
	}
	return normalised
}
