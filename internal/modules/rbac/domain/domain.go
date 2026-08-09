// Package domain holds the rbac module's value objects and invariants. It
// imports no infrastructure, which is what lets the rules below be stated as
// plain functions and tested with no setup at all.
package domain

import (
	"slices"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// The error codes this module owns, as declared in its AGENT.md §12.
var (
	// ErrPermissionDenied is the outcome of every failed check: no caller,
	// malformed permission, unknown permission, or one simply not granted.
	// They are one error on purpose — telling a caller which of the four it
	// was tells them something about the catalogue they have not earned.
	ErrPermissionDenied = apperr.New(
		apperr.Forbidden, "PERMISSION_DENIED", "You do not have permission to perform this action.")

	// ErrSelfElevationForbidden is BR-RBAC-04.
	ErrSelfElevationForbidden = apperr.New(
		apperr.Forbidden, "SELF_ELEVATION_FORBIDDEN", "You cannot grant yourself a role you do not hold.")

	// ErrSelfDemotionForbidden is the other half of BR-RBAC-04: an admin
	// removing their own admin role. It is separate from the last-admin guard
	// because it applies even when other admins exist.
	ErrSelfDemotionForbidden = apperr.New(
		apperr.Forbidden, "SELF_DEMOTION_FORBIDDEN", "You cannot remove your own admin role.")

	// ErrLastAdminProtected is BR-RBAC-05. It is a 409 rather than a 403: the
	// actor is allowed to do this, the system's state is what forbids it.
	ErrLastAdminProtected = apperr.New(
		apperr.Conflict, "LAST_ADMIN_PROTECTED", "The last administrator cannot be demoted.")

	// ErrUnknownRole rejects anything outside the two-role set.
	ErrUnknownRole = apperr.New(apperr.Validation, "VALIDATION_FAILED", "One or more request fields are invalid.").
			WithFields(apperr.FieldViolation{
			Field: "role", Code: "UNKNOWN", Message: "Role must be admin or user.",
		})
)

// PermissionSet is the resolved set of permissions an actor holds.
//
// The zero value is an empty set that grants nothing, which is the whole point:
// deny-by-default has to survive a nil map, a failed cache read and a caller
// who forgot to populate it. There is no constructor that produces a set
// meaning "everything".
type PermissionSet struct {
	granted map[contract.Permission]struct{}
}

// NewPermissionSet builds a set from resolved permission names.
func NewPermissionSet(permissions []contract.Permission) PermissionSet {
	granted := make(map[contract.Permission]struct{}, len(permissions))
	for _, permission := range permissions {
		granted[permission] = struct{}{}
	}
	return PermissionSet{granted: granted}
}

// Has reports whether the set grants permission.
//
// A malformed permission is refused before the lookup. That is not defensive
// noise: `Permission("")` and `Permission("admin")` are values the type system
// permits, and a set that happened to contain one would otherwise grant it.
func (s PermissionSet) Has(permission contract.Permission) bool {
	if !permission.Valid() {
		return false
	}
	_, ok := s.granted[permission]
	return ok
}

// Names returns the granted permissions in a stable order, for rendering.
func (s PermissionSet) Names() []contract.Permission {
	names := make([]contract.Permission, 0, len(s.granted))
	for permission := range s.granted {
		names = append(names, permission)
	}
	slices.Sort(names)
	return names
}

// Len reports how many permissions the set grants.
func (s PermissionSet) Len() int { return len(s.granted) }

// RoleWithPermissions is a role and everything it grants — the shape the role
// catalogue is read as.
//
// It lives here rather than in the repository so that the service and the
// transport can both name it without either importing the repository. A
// transport package that imports the repository has skipped a layer, and the
// linter cannot tell the difference between skipping one deliberately and
// doing it by accident.
type RoleWithPermissions struct {
	Name        contract.Role
	Description string
	Permissions []contract.Permission
}

// RoleChange is a grant or revocation, with everything the rules need to
// judge it. Passing one value rather than four arguments is what keeps the
// checks below from being called with the actor and the target swapped.
type RoleChange struct {
	// ActorID is who is making the change.
	ActorID uuid.UUID
	// TargetID is whose roles are changing.
	TargetID uuid.UUID
	// Role is the role being granted or revoked.
	Role contract.Role
	// ActorRoles is what the actor currently holds.
	ActorRoles []contract.Role
	// TargetRoles is what the target currently holds.
	TargetRoles []contract.Role
}

// SelfTargeted reports whether the actor is changing their own roles.
func (c RoleChange) SelfTargeted() bool { return c.ActorID == c.TargetID }

// ValidateGrant enforces BR-RBAC-04 for a grant: an actor may not give
// themselves a role they do not already hold.
//
// Granting yourself a role you already have is allowed because it is a no-op —
// refusing it would mean an admin re-running a script gets an error for
// changing nothing.
func (c RoleChange) ValidateGrant() error {
	if !c.SelfTargeted() {
		return nil
	}
	if slices.Contains(c.ActorRoles, c.Role) {
		return nil
	}
	return ErrSelfElevationForbidden
}

// ValidateRevoke enforces the rest of BR-RBAC-04 and BR-RBAC-05.
//
// adminCount is the number of accounts currently holding `admin`, read under a
// row lock in the same transaction as the delete. Reading it outside one would
// let two administrators revoke each other simultaneously, each seeing two.
func (c RoleChange) ValidateRevoke(adminCount int) error {
	if c.Role != contract.RoleAdmin {
		return nil
	}
	if c.SelfTargeted() {
		return ErrSelfDemotionForbidden
	}
	// Only a target who actually holds the role can reduce the count; revoking
	// from somebody who does not hold it is a no-op and must not be refused as
	// though it would empty the system.
	if !slices.Contains(c.TargetRoles, contract.RoleAdmin) {
		return nil
	}
	if adminCount <= 1 {
		return ErrLastAdminProtected
	}
	return nil
}
