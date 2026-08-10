// Package domain holds the audit module's rules: what an entry may contain,
// what must never reach the table, and how a search is bounded. It is pure Go
// with no I/O, so every rule here is testable without a database — which
// matters, because "the value was redacted" is a claim you want proven by a
// test rather than by reading the calling code.
package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// namePattern is the shape of an action and of a security event kind:
// `<module>.<verb>_<object>`. The search filters on exact names, so a name
// that does not match this is one nobody can find again.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

// Errors this module returns across the HTTP boundary.
var (
	// ErrEventNotFound is also what a caller sees when they name an event in a
	// partition that retention has detached: gone and never-there are the same
	// answer to the client, and distinguishing them would leak how long the
	// trail is kept.
	ErrEventNotFound = apperr.New(apperr.NotFound, "NOT_FOUND",
		"The requested resource was not found.")

	// ErrAlreadyResolved is a conflict rather than a silent overwrite: the
	// first administrator's explanation describes what was actually
	// investigated, and the second would replace it with a guess.
	ErrAlreadyResolved = apperr.New(apperr.Conflict, "SECURITY_EVENT_ALREADY_RESOLVED",
		"This security event has already been resolved.")

	// ErrInvalidAction guards the write path. It is never rendered to a
	// learner — only a module calling Recorder with a malformed name sees it,
	// and it sees it in a log rather than a response.
	ErrInvalidAction = apperr.New(apperr.Validation, "VALIDATION_FAILED",
		"An audit action must be named <module>.<verb>_<object>.")
)

// ValidName reports whether an action or event kind is well formed.
func ValidName(name string) bool { return namePattern.MatchString(name) }

// ---------------------------------------------------------------- redaction

// redactedValue replaces anything the deny-list catches. It is a marker rather
// than a removal so that the entry still records that the field moved, which
// is the part the trail is actually for.
const redactedValue = "[redacted]"

// deniedExact are field names whose value must never be stored. They are
// matched on the whole name, case-insensitively.
var deniedExact = map[string]struct{}{
	"email": {}, "email_address": {}, "display_name": {}, "full_name": {},
	"first_name": {}, "last_name": {}, "given_name": {}, "family_name": {},
	"phone": {}, "phone_number": {}, "address": {}, "street": {}, "postcode": {},
	"date_of_birth": {}, "dob": {}, "birthday": {}, "national_id": {},
	"avatar_url": {}, "ip": {}, "ip_address": {}, "user_agent": {},
	"pan": {}, "card_number": {}, "cvv": {}, "iban": {},
}

// deniedSubstrings catch the families rather than the members. A new column
// called `reset_token_hash` or `argon2_password` is redacted the day it is
// added, by nobody having to remember to add it here.
//
// This is the fail-closed half of BR-AUDIT-04. The exact list above is a
// convenience; this is the rule.
var deniedSubstrings = []string{
	"password", "secret", "token", "credential", "api_key", "apikey",
	"private_key", "signature", "otp", "session_id", "cookie",
}

// Denied reports whether a field's value must be redacted.
func Denied(field string) bool {
	lowered := strings.ToLower(strings.TrimSpace(field))
	if _, found := deniedExact[lowered]; found {
		return true
	}
	for _, fragment := range deniedSubstrings {
		if strings.Contains(lowered, fragment) {
			return true
		}
	}
	return false
}

// maxRedactDepth bounds recursion. A payload deep enough to hit it is either a
// mistake or an attempt to make the redactor give up part-way; either way the
// remainder is replaced wholesale rather than stored unexamined.
const maxRedactDepth = 8

// Redact returns a copy with every denied field's value replaced, descending
// into nested objects and arrays.
//
// It never mutates its argument. The caller usually owns that map — it is the
// diff they are about to use for something else — and a redactor that edited
// it in place would silently change the caller's data as a side effect of
// logging it.
func Redact(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	return redactMap(values, 0)
}

func redactMap(values map[string]any, depth int) map[string]any {
	redacted := make(map[string]any, len(values))
	for field, value := range values {
		switch {
		case Denied(field):
			redacted[field] = redactedValue
		case depth >= maxRedactDepth:
			// Too deep to inspect: keep the key, drop the contents. A field
			// this nested is not something the trail needs verbatim.
			redacted[field] = redactedValue
		default:
			redacted[field] = redactValue(value, depth+1)
		}
	}
	return redacted
}

func redactValue(value any, depth int) any {
	switch typed := value.(type) {
	case map[string]any:
		if depth >= maxRedactDepth {
			return redactedValue
		}
		return redactMap(typed, depth)
	case []any:
		if depth >= maxRedactDepth {
			return redactedValue
		}
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, redactValue(item, depth+1))
		}
		return items
	default:
		return value
	}
}

// ChangedFields returns the sorted union of the keys in before and after.
//
// Names are not redacted — knowing that the display name changed is the record
// (BR-AUDIT-04); knowing what it changed to would be a second copy of it.
func ChangedFields(before, after map[string]any) []string {
	seen := make(map[string]struct{}, len(before)+len(after))
	for field := range before {
		seen[field] = struct{}{}
	}
	for field := range after {
		seen[field] = struct{}{}
	}
	fields := make([]string, 0, len(seen))
	for field := range seen {
		fields = append(fields, field)
	}
	slices.Sort(fields)
	return fields
}

// MaxChangedFields matches the check constraint on the column. A change
// touching more fields than this is a bulk operation, and the names of the
// first 64 describe it well enough.
const MaxChangedFields = 64

// NormaliseFields sorts, de-duplicates and caps a field list.
func NormaliseFields(fields []string) []string {
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	slices.Sort(cleaned)
	cleaned = slices.Compact(cleaned)
	if len(cleaned) > MaxChangedFields {
		cleaned = cleaned[:MaxChangedFields]
	}
	return cleaned
}

// ------------------------------------------------------------- ip addresses

// HashIP returns the keyed hash of an address, or "" when there is nothing to
// hash or no key to hash it with.
//
// It is an HMAC rather than a plain digest because the IPv4 space is 2^32: an
// unkeyed SHA-256 of an address is reversible by anybody with a machine and an
// afternoon, so a column full of them would be a column full of addresses.
//
// Returning "" for a missing key is deliberate. The alternative — hashing with
// an empty key — produces a value that looks correct in every test and
// protects nothing in production.
func HashIP(address netip.Addr, key []byte) string {
	if !address.IsValid() || len(key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(address.Unmap().String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// ------------------------------------------------------------------- search

// Window is the time range a search covers.
type Window struct {
	Start time.Time
	End   time.Time
}

// Search window defaults. The tables are partitioned monthly on created_at, so
// a search with no bound on it reads every partition that has ever existed.
// The window is therefore not optional — it is defaulted.
const (
	DefaultWindow = 90 * 24 * time.Hour
	MaxWindow     = 400 * 24 * time.Hour
)

// NewWindow resolves the optional bounds a client sent into a real range.
//
// `to` defaults to now, `from` to DefaultWindow before it, and a range longer
// than MaxWindow is trimmed from the far end rather than refused: an
// administrator who asked for too much gets the recent part of it, which is
// the part they were looking at.
func NewWindow(from, to *time.Time, now time.Time) (Window, error) {
	end := now
	if to != nil {
		end = *to
	}
	start := end.Add(-DefaultWindow)
	if from != nil {
		start = *from
	}
	if !start.Before(end) {
		return Window{}, apperr.New(apperr.Validation, "VALIDATION_FAILED",
			"One or more request fields are invalid.").
			WithFields(apperr.FieldViolation{
				Field: "from", Code: "RANGE", Message: "from must be earlier than to.",
			})
	}
	if end.Sub(start) > MaxWindow {
		start = end.Add(-MaxWindow)
	}
	return Window{Start: start.UTC(), End: end.UTC()}, nil
}

// Page size bounds, matching the OpenAPI schema.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// NormaliseLimit clamps a requested page size. Zero means "not supplied".
func NormaliseLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultLimit
	case limit > MaxLimit:
		return MaxLimit
	default:
		return limit
	}
}

// Position is a keyset cursor over (created_at, id), matching the primary key
// of both tables so that paging deep costs what paging shallow costs.
type Position struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// LogQuery is a search of the audit trail. A nil pointer means "no filter";
// the window and the limit are always set.
type LogQuery struct {
	Window     Window
	ActorID    *uuid.UUID
	Action     *string
	TargetType *string
	TargetID   *string
	After      *Position
	Limit      int
}

// SecurityQuery is a search of the security event stream.
type SecurityQuery struct {
	Window   Window
	Kind     *string
	Severity *contract.Severity
	UserID   *uuid.UUID
	Resolved *bool
	After    *Position
	Limit    int
}

// ------------------------------------------------------------------ records

// LogEntry is one row of audit.audit_logs.
type LogEntry struct {
	ID            uuid.UUID
	CreatedAt     time.Time
	EventID       uuid.UUID
	ActorID       *uuid.UUID
	ActorRole     *contract.ActorRole
	Action        string
	TargetType    *string
	TargetID      *string
	ChangedFields []string
	Before        map[string]any
	After         map[string]any
	Meta          map[string]any
	IPHash        *string
	TraceID       *string
}

// SecurityRecord is one row of audit.security_events.
type SecurityRecord struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	EventID        uuid.UUID
	Kind           string
	Severity       contract.Severity
	UserID         *uuid.UUID
	Detail         map[string]any
	IPHash         *string
	TraceID        *string
	ResolvedAt     *time.Time
	ResolvedBy     *uuid.UUID
	ResolutionNote *string
}

// Resolved reports whether the event has been triaged.
func (r SecurityRecord) Resolved() bool { return r.ResolvedAt != nil }

// Resolution is the triage of one security event.
type Resolution struct {
	ID        uuid.UUID
	CreatedAt time.Time
	By        uuid.UUID
	Note      string
	At        time.Time
}

// MaxNoteLength matches the check constraint on the column.
const MaxNoteLength = 500

// ValidateNote checks the explanation an administrator gave.
func ValidateNote(note string) (string, error) {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" || len([]rune(trimmed)) > MaxNoteLength {
		return "", apperr.New(apperr.Validation, "VALIDATION_FAILED",
			"One or more request fields are invalid.").
			WithFields(apperr.FieldViolation{
				Field: "note", Code: "LENGTH",
				Message: "note is required and must be at most 500 characters.",
			})
	}
	return trimmed, nil
}

// ------------------------------------------------------------- event timing

// TimeFromUUIDv7 extracts the millisecond timestamp a version 7 UUID carries
// in its first 48 bits.
//
// The audit consumer needs a *deterministic* created_at: it is the partition
// key, and the unique index that makes redelivery produce one row instead of
// two only catches the duplicate if both attempts land in the same partition
// with the same key. An event whose payload carries no occurred_at therefore
// falls back to the time inside its own id rather than to now().
//
// It is decoded here rather than through uuid.UUID.Time, which assumes the
// version 1 layout and returns something meaningless for a v7 value.
func TimeFromUUIDv7(id uuid.UUID) (time.Time, bool) {
	if id.Version() != 7 {
		return time.Time{}, false
	}
	var millis int64
	for _, octet := range id[:6] {
		millis = millis<<8 | int64(octet)
	}
	return time.UnixMilli(millis).UTC(), true
}
