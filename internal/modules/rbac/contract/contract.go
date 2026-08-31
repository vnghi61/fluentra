package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Authorizer is the guard every service method calls.
//
// It is called in the service layer and not only in middleware. Middleware
// protects a route; the guard protects an operation, and an operation reached
// by a job, an event consumer or a second route is otherwise unprotected.
type Authorizer interface {
	// Require returns nil when the caller in ctx holds permission, and an
	// error rendering as 403 otherwise. It denies when there is no caller,
	// when the permission is malformed, and when it is simply not granted —
	// three different mistakes with one safe outcome.
	Require(ctx context.Context, permission Permission) error

	// Can is Require without the error, for deciding what to render rather
	// than whether to proceed. Anything that acts on the answer should call
	// Require: `if !Can(...) { return }` inverts to allow-by-default the
	// moment somebody forgets the `!`.
	Can(ctx context.Context, permission Permission) bool
}

// RoleReader exposes role membership. `auth` uses it when minting a token.
type RoleReader interface {
	// RolesOf returns the roles assigned to userID, or an empty slice.
	RolesOf(ctx context.Context, userID uuid.UUID) ([]Role, error)

	// PermissionsOf returns the flat set of permissions those roles grant.
	PermissionsOf(ctx context.Context, userID uuid.UUID) ([]Permission, error)
}

// RoleMembers answers who holds a role.
//
// One caller, and a narrow interface rather than a method on RoleReader because
// `auth` uses RoleReader when minting a token and has no business being able to
// enumerate role holders.
type RoleMembers interface {
	// FirstHolderOf returns the longest-standing holder of a role, or
	// uuid.Nil when nobody holds it. Ordered by grant time so the answer does
	// not move when a second holder is added.
	FirstHolderOf(ctx context.Context, role Role) (uuid.UUID, error)
}

// Event names published by this module.
const (
	EventRoleAssigned = "rbac.role_assigned"
	EventRoleRevoked  = "rbac.role_revoked"
	EventAccessDenied = "rbac.access_denied"
)

// Aggregate is the outbox aggregate name for the events above.
const Aggregate = "rbac"

// RoleAssigned is published when a role is granted.
type RoleAssigned struct {
	UserID     uuid.UUID `json:"user_id"`
	Role       Role      `json:"role"`
	ActorID    uuid.UUID `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// RoleRevoked is published when a role is removed.
type RoleRevoked struct {
	UserID     uuid.UUID `json:"user_id"`
	Role       Role      `json:"role"`
	ActorID    uuid.UUID `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// AccessDenied is published when a permission check fails (BR-RBAC-07).
//
// It carries the permission that was refused and not the request that asked
// for it: the audit trail needs to show a pattern of refusals, and a copy of
// the request body would make it a store of whatever the attacker sent.
type AccessDenied struct {
	UserID     uuid.UUID  `json:"user_id"`
	Permission Permission `json:"permission"`
	OccurredAt time.Time  `json:"occurred_at"`
}
