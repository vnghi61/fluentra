package contract

import (
	"context"

	"github.com/google/uuid"
)

// The permissions this module's operations require.
//
// They are strings rather than rbac's Permission type because `audit` does not
// depend on `rbac` — every arrow in MODULE_INDEX.md §3 points the other way.
// The composition root adapts the guard, which is the one place that is
// allowed to see both modules.
const (
	PermissionRead   = "audit.read"
	PermissionExport = "audit.export"
	PermissionManage = "audit.manage"
)

// ActorRole is the role the actor held when they acted.
//
// It is recorded on the entry rather than resolved from `rbac` at read time.
// A revoked administrator must still read as an administrator in the entries
// they left behind — the trail describes what was true then, and resolving it
// later would quietly rewrite history every time somebody changed a role.
type ActorRole string

// The three roles an entry can be attributed to. `system` covers work with no
// human behind it: a scheduled job, a migration, a consumer.
const (
	ActorRoleAdmin  ActorRole = "admin"
	ActorRoleUser   ActorRole = "user"
	ActorRoleSystem ActorRole = "system"
)

// Valid reports whether the role is one of the three.
func (r ActorRole) Valid() bool {
	switch r {
	case ActorRoleAdmin, ActorRoleUser, ActorRoleSystem:
		return true
	default:
		return false
	}
}

// String returns the wire value.
func (r ActorRole) String() string { return string(r) }

// Severity is how much attention a security event deserves.
type Severity string

// The severity ladder. `critical` is reserved for events indicating a
// compromise in progress, such as refresh-token reuse.
const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Valid reports whether the severity is one of the four.
func (s Severity) Valid() bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

// String returns the wire value.
func (s Severity) String() string { return string(s) }

// Entry is one action to record.
//
// The actor and the trace context are not fields: they come from ctx, which is
// what stops a caller attributing an action to somebody else by passing a
// different id. There is no way to write an entry claiming to be another user
// without first becoming them.
type Entry struct {
	// Action names what happened, as `<module>.<verb>_<object>` — for example
	// `user.profile_updated`. An entry whose action does not match that shape
	// is refused, because the search filters on exact names.
	Action string

	// TargetType and TargetID say what was acted on. TargetID is a string
	// rather than a uuid because not every auditable target is keyed by one.
	TargetType string
	TargetID   string

	// Before and After carry the values of the fields that moved. Both are
	// optional, and in Phase 1 both are usually empty: the modules that emit
	// through the outbox send field *names* only (BR-AUDIT-04). Anything on
	// the PII deny-list is replaced with `[redacted]` before it is stored, so
	// passing a display name here does not put one in the table.
	Before map[string]any
	After  map[string]any

	// ChangedFields overrides the field list. Leave it nil and the recorder
	// derives it from Before and After; set it when the change is known by
	// name but not by value, which is the normal case for an outbox event.
	ChangedFields []string

	// Meta is context that is not a field of the target: the reason an
	// administrator gave, the id of the event that caused this. Redacted on
	// the same deny-list.
	Meta map[string]any
}

// SecurityEvent is one security-relevant occurrence to record.
type SecurityEvent struct {
	// Kind names it as `<module>.<event>`, for example `rbac.access_denied`.
	Kind string

	// Severity decides where it appears on the dashboard. An empty value is
	// treated as `low` rather than refused: losing the event would be worse
	// than filing it too quietly.
	Severity Severity

	// UserID is the account involved, when one is known. uuid.Nil means the
	// event was observed before anybody was identified, which is normal for a
	// failed sign-in.
	UserID uuid.UUID

	// Detail is structured context for triage, redacted like a diff. It must
	// never carry the request body: an event raised by an attacker would
	// otherwise store whatever they chose to send.
	Detail map[string]any
}

// Recorder writes to the audit trail.
//
// Record returns nothing. That is the interface expressing BR-AUDIT-02: an
// audit write must never fail the business operation it describes, so there is
// no error for a caller to accidentally propagate and no value to make a
// caller's success conditional on. A failure is logged by the implementation
// and the caller proceeds.
//
// Use this for actions where losing an occasional entry is acceptable — reads
// of personal data, administrative lookups. For changes where the record must
// not be lost (permissions, money, publishing), write an outbox event in the
// same transaction as the change instead and let the consumer file it; that is
// the half of BR-AUDIT-02 this interface deliberately cannot give you.
type Recorder interface {
	Record(ctx context.Context, entry Entry)
}

// SecurityRecorder writes to the security event stream, on the same
// never-fails-the-caller terms.
//
// It is a second interface rather than a second method on Recorder so that a
// module which only records actions depends only on what it uses.
type SecurityRecorder interface {
	RecordSecurityEvent(ctx context.Context, event SecurityEvent)
}
