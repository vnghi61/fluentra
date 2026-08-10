package domain

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Literals the deny-list tests reach for repeatedly.
const (
	fieldDisplayName = "display_name"
	fieldEmail       = "email"
	fieldTheme       = "theme"
	fieldTimezone    = "timezone"
	fieldLocale      = "locale"
	sampleName       = "Nghi"
	sampleTimezone   = "Asia/Ho_Chi_Minh"
	sampleEmail      = "learner@example.com"
)

func TestValidName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want bool
	}{
		{"user.profile_updated", true},
		{"rbac.role_assigned", true},
		{"a.b", true},
		{"user.profile.updated", false}, // two dots: the column check rejects it too
		{"User.Profile_Updated", false},
		{"user", false},
		{".updated", false},
		{"user.", false},
		{"1user.updated", false},
		{"", false},
		{"user.profile_updated ", false},
	}
	for _, testCase := range cases {
		if got := ValidName(testCase.name); got != testCase.want {
			t.Errorf("ValidName(%q) = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

// TestRedactCoversTheDenyList is BR-AUDIT-04 stated as a test. Every name here
// is one that would put personal data in a table with a two-year retention
// period if it went through unredacted.
func TestRedactCoversTheDenyList(t *testing.T) {
	t.Parallel()

	denied := []string{
		fieldEmail, "Email", "EMAIL", fieldDisplayName, "date_of_birth", "phone",
		"password", "password_hash", "argon2_password", "reset_token",
		"refresh_token_hash", "api_key", "client_secret", "otp_code",
		"session_id", "ip_address", "user_agent", "card_number",
	}
	for _, field := range denied {
		if !Denied(field) {
			t.Errorf("Denied(%q) = false; that value would be stored verbatim", field)
		}
		redacted := Redact(map[string]any{field: "the actual value"})
		if redacted[field] != redactedValue {
			t.Errorf("Redact kept %q as %v", field, redacted[field])
		}
	}

	allowed := []string{"status", fieldLocale, fieldTimezone, "role", fieldTheme, "id"}
	for _, field := range allowed {
		if Denied(field) {
			t.Errorf("Denied(%q) = true; the trail would lose a field it is meant to record", field)
		}
	}
}

func TestRedactDescendsIntoNestedValues(t *testing.T) {
	t.Parallel()

	redacted := Redact(map[string]any{
		"profile": map[string]any{
			fieldDisplayName: sampleName,
			fieldTimezone:    sampleTimezone,
		},
		"contacts": []any{
			map[string]any{fieldEmail: sampleEmail, "kind": "primary"},
		},
	})

	profile, ok := redacted["profile"].(map[string]any)
	if !ok {
		t.Fatalf("profile = %T, want map", redacted["profile"])
	}
	if profile[fieldDisplayName] != redactedValue {
		t.Errorf("nested display_name = %v, want redacted", profile[fieldDisplayName])
	}
	if profile[fieldTimezone] != sampleTimezone {
		t.Errorf("nested timezone = %v, want it kept", profile[fieldTimezone])
	}

	contacts, ok := redacted["contacts"].([]any)
	if !ok || len(contacts) != 1 {
		t.Fatalf("contacts = %v", redacted["contacts"])
	}
	contact, ok := contacts[0].(map[string]any)
	if !ok {
		t.Fatalf("contact = %T, want map", contacts[0])
	}
	if contact[fieldEmail] != redactedValue {
		t.Errorf("email inside an array = %v, want redacted", contact[fieldEmail])
	}
	if contact["kind"] != "primary" {
		t.Errorf("kind inside an array = %v, want it kept", contact["kind"])
	}
}

// TestRedactDoesNotMutateItsArgument matters because the caller usually still
// owns that map — it is the diff they built for their own use, and a redactor
// that edited it in place would change their data as a side effect.
func TestRedactDoesNotMutateItsArgument(t *testing.T) {
	t.Parallel()

	original := map[string]any{
		fieldEmail: sampleEmail,
		"profile":  map[string]any{fieldDisplayName: sampleName},
	}
	Redact(original)

	if original[fieldEmail] != sampleEmail {
		t.Errorf("Redact overwrote the caller's map: email = %v", original[fieldEmail])
	}
	nested, _ := original["profile"].(map[string]any)
	if nested[fieldDisplayName] != sampleName {
		t.Errorf("Redact overwrote a nested map the caller owns: %v", nested)
	}
}

// TestRedactStopsAtMaxDepth proves the recursion is bounded rather than
// trusting that no payload is ever that deep.
func TestRedactStopsAtMaxDepth(t *testing.T) {
	t.Parallel()

	deepest := map[string]any{fieldEmail: sampleEmail}
	current := deepest
	for range maxRedactDepth + 4 {
		current = map[string]any{"nested": current}
	}

	redacted := Redact(current)
	// Walk down until the redactor gave up; whatever is there must not be a
	// live map still carrying the address.
	value := any(redacted)
	for range maxRedactDepth + 4 {
		nested, ok := value.(map[string]any)
		if !ok {
			break
		}
		next, present := nested["nested"]
		if !present {
			if nested[fieldEmail] != nil && nested[fieldEmail] != redactedValue {
				t.Fatalf("an unredacted email survived at depth: %v", nested[fieldEmail])
			}
			return
		}
		value = next
	}
	if value != redactedValue {
		t.Errorf("recursion bottomed out at %v, want %q", value, redactedValue)
	}
}

func TestChangedFieldsIsTheSortedUnion(t *testing.T) {
	t.Parallel()

	got := ChangedFields(
		map[string]any{fieldTimezone: "UTC", fieldLocale: "en"},
		map[string]any{fieldTimezone: sampleTimezone, fieldTheme: "dark"},
	)
	want := []string{fieldLocale, fieldTheme, fieldTimezone}
	if !slices.Equal(got, want) {
		t.Errorf("ChangedFields = %v, want %v", got, want)
	}

	if got := ChangedFields(nil, nil); len(got) != 0 {
		t.Errorf("ChangedFields(nil, nil) = %v, want empty", got)
	}
}

func TestNormaliseFieldsSortsDeduplicatesAndCaps(t *testing.T) {
	t.Parallel()

	got := NormaliseFields([]string{fieldTimezone, " ", fieldLocale, fieldTimezone, "  theme  "})
	want := []string{fieldLocale, fieldTheme, fieldTimezone}
	if !slices.Equal(got, want) {
		t.Errorf("NormaliseFields = %v, want %v", got, want)
	}

	overflowing := make([]string, 0, MaxChangedFields*2)
	for index := range MaxChangedFields * 2 {
		overflowing = append(overflowing, "field_"+string(rune('a'+index%26))+strconv.Itoa(index))
	}
	if got := NormaliseFields(overflowing); len(got) != MaxChangedFields {
		t.Errorf("NormaliseFields kept %d fields, want the column cap of %d", len(got), MaxChangedFields)
	}
}

// TestHashIPRefusesToWorkWithoutAKey is the important one. An unkeyed digest
// of an IPv4 address is reversible by anybody willing to hash four billion
// values, so a hash produced without a key would look like protection and be
// none. The function must produce nothing rather than something useless.
func TestHashIPRefusesToWorkWithoutAKey(t *testing.T) {
	t.Parallel()

	address := netip.MustParseAddr("203.0.113.7")

	if got := HashIP(address, nil); got != "" {
		t.Errorf("HashIP with no key = %q, want the empty string", got)
	}
	if got := HashIP(address, []byte{}); got != "" {
		t.Errorf("HashIP with an empty key = %q, want the empty string", got)
	}
	if got := HashIP(netip.Addr{}, []byte("k")); got != "" {
		t.Errorf("HashIP of an invalid address = %q, want the empty string", got)
	}
}

func TestHashIPIsStableKeyedAndOpaque(t *testing.T) {
	t.Parallel()

	key := []byte("a-test-key-that-is-not-a-secret")
	address := netip.MustParseAddr("203.0.113.7")

	first := HashIP(address, key)
	if len(first) != 64 {
		t.Fatalf("HashIP = %q (%d chars), want 64 hex characters", first, len(first))
	}
	if first != HashIP(address, key) {
		t.Error("HashIP is not stable for the same address and key")
	}
	if first == HashIP(address, []byte("a-different-key")) {
		t.Error("HashIP ignored the key")
	}
	if first == HashIP(netip.MustParseAddr("203.0.113.8"), key) {
		t.Error("HashIP collided across two different addresses")
	}
	if strings.Contains(first, "203") {
		t.Errorf("HashIP = %q, which contains part of the address", first)
	}

	// An IPv4-mapped IPv6 address is the same address, and must hash the same.
	mapped := netip.MustParseAddr("::ffff:203.0.113.7")
	if HashIP(mapped, key) != first {
		t.Error("the IPv4-mapped form of an address hashed differently")
	}
}

func TestNewWindowDefaultsToABoundedRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	window, err := NewWindow(nil, nil, now)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if !window.End.Equal(now) {
		t.Errorf("End = %s, want now", window.End)
	}
	if got := window.End.Sub(window.Start); got != DefaultWindow {
		t.Errorf("default span = %s, want %s", got, DefaultWindow)
	}
}

// TestNewWindowTrimsAnOverlongRange keeps the search inside a bounded number of
// monthly partitions even when a client asks for everything.
func TestNewWindowTrimsAnOverlongRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	ancient := now.Add(-20 * 365 * 24 * time.Hour)

	window, err := NewWindow(&ancient, nil, now)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if got := window.End.Sub(window.Start); got != MaxWindow {
		t.Errorf("span = %s, want it trimmed to %s", got, MaxWindow)
	}
	if !window.End.Equal(now) {
		t.Errorf("trimming moved the recent end to %s; it should trim the far end", window.End)
	}
}

func TestNewWindowRejectsAnInvertedRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	from := now
	to := now.Add(-time.Hour)

	if _, err := NewWindow(&from, &to, now); !apperr.Is(err, apperr.Validation) {
		t.Errorf("NewWindow(from > to) error = %v, want a validation error", err)
	}
}

func TestNormaliseLimit(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want int }{
		{0, DefaultLimit}, {-5, DefaultLimit}, {1, 1},
		{MaxLimit, MaxLimit}, {MaxLimit + 1, MaxLimit}, {10_000, MaxLimit},
	}
	for _, testCase := range cases {
		if got := NormaliseLimit(testCase.in); got != testCase.want {
			t.Errorf("NormaliseLimit(%d) = %d, want %d", testCase.in, got, testCase.want)
		}
	}
}

func TestValidateNote(t *testing.T) {
	t.Parallel()

	if _, err := ValidateNote("   "); !apperr.Is(err, apperr.Validation) {
		t.Errorf("a blank note was accepted: %v", err)
	}
	if _, err := ValidateNote(strings.Repeat("x", MaxNoteLength+1)); !apperr.Is(err, apperr.Validation) {
		t.Errorf("an overlong note was accepted: %v", err)
	}
	got, err := ValidateNote("  Known load test.  ")
	if err != nil || got != "Known load test." {
		t.Errorf("ValidateNote = %q, %v; want the trimmed note", got, err)
	}
}

// TestTimeFromUUIDv7 underpins the consumer's idempotency: an event with no
// occurred_at must still land in the same partition on every redelivery, and
// the id is where that determinism comes from.
func TestTimeFromUUIDv7(t *testing.T) {
	t.Parallel()

	generated, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	extracted, ok := TimeFromUUIDv7(generated)
	if !ok {
		t.Fatal("TimeFromUUIDv7 refused a version 7 uuid")
	}
	if delta := time.Since(extracted); delta < 0 || delta > time.Minute {
		t.Errorf("extracted %s, which is %s away from now", extracted, delta)
	}
	// Determinism is the whole point: same id, same answer, every time.
	again, _ := TimeFromUUIDv7(generated)
	if !again.Equal(extracted) {
		t.Errorf("TimeFromUUIDv7 is not deterministic: %s then %s", extracted, again)
	}

	if _, ok := TimeFromUUIDv7(uuid.New()); ok {
		t.Error("TimeFromUUIDv7 accepted a version 4 uuid, whose bytes are random")
	}
	if _, ok := TimeFromUUIDv7(uuid.Nil); ok {
		t.Error("TimeFromUUIDv7 accepted the nil uuid")
	}
}
