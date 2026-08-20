package contract

import "regexp"

// Permission is a named capability.
//
// It is a distinct type, and not a string, for one reason: the zero value must
// not be usable. `Require(ctx, "")` on a string parameter is a call that
// compiles, reads plausibly, and grants everything if the implementation is
// careless. `Permission("")` is not in the catalogue, so it is denied by the
// same code path that denies any other permission the actor lacks — the guard
// has no special case to forget.
//
// Never write a permission as a literal at a call site. Use a constant below;
// a new one is a line here, a row in a migration, and a test.
type Permission string

// The Phase 1 permission catalogue. Every constant here has a matching row in
// db/migrations/rbac — a constant without a row denies, which is the safe
// direction, and a row without a constant is unreachable rather than dangerous.
const (
	// rbac
	PermRBACRead   Permission = "rbac.read"
	PermRBACAssign Permission = "rbac.assign"

	// user
	PermUserList           Permission = "user.list"
	PermUserRead           Permission = "user.read"
	PermUserSuspend        Permission = "user.suspend"
	PermUserReinstate      Permission = "user.reinstate"
	PermUserManageSessions Permission = "user.manage_sessions"
	PermUserImpersonate    Permission = "user.impersonate"

	// audit
	PermAuditRead   Permission = "audit.read"
	PermAuditExport Permission = "audit.export"
	PermAuditManage Permission = "audit.manage"

	// back office
	PermAdminDashboard Permission = "admin.dashboard"
	PermModerationRead Permission = "moderation.read"
	PermModerationAct  Permission = "moderation.act"
	PermSystemFlags    Permission = "system.flags"
	PermSystemJobs     Permission = "system.jobs"

	// content reading, authoring & publishing
	//
	// content.read.published gates published courses, lessons and content
	// versions. It is in the catalogue and held by admin; who else holds it is
	// open — see db/migrations/rbac/1700000180 and WP7 task P7.5. Declaring it
	// here is what lets a handler name it without a bare string.
	PermContentReadPublished Permission = "content.read.published"

	PermContentCreate  Permission = "content.create"
	PermContentEdit    Permission = "content.edit"
	PermContentReview  Permission = "content.review"
	PermContentPublish Permission = "content.publish"
)

// All is every permission this build knows about. An integration test compares
// it against the catalogue in the database, so a constant added without a
// migration — or a migration without a constant — fails rather than drifting.
func All() []Permission {
	return []Permission{
		PermRBACRead, PermRBACAssign,
		PermUserList, PermUserRead, PermUserSuspend, PermUserReinstate, PermUserManageSessions, PermUserImpersonate,
		PermAuditRead, PermAuditExport, PermAuditManage,
		PermAdminDashboard, PermModerationRead, PermModerationAct,
		PermSystemFlags, PermSystemJobs,
		PermContentReadPublished,
		PermContentCreate, PermContentEdit, PermContentReview, PermContentPublish,
	}
}

// permissionPattern is BR-RBAC-03: `<resource>.<action>[.<qualifier>]`. It
// matches the check constraint on core.permissions.name.
var permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)?$`)

// Valid reports whether p is a well-formed permission name. It says nothing
// about whether the permission exists or is granted — an unknown but
// well-formed name is still denied.
func (p Permission) Valid() bool { return permissionPattern.MatchString(string(p)) }

// String renders the permission for logs and error messages.
func (p Permission) String() string { return string(p) }

// Role is one of exactly two values (BR-RBAC-02).
type Role string

// The complete set of roles.
const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// ParseRole converts a submitted value into a Role, rejecting anything else.
// A third role is an ADR, not a request body.
func ParseRole(value string) (Role, bool) {
	switch Role(value) {
	case RoleAdmin, RoleUser:
		return Role(value), true
	default:
		return "", false
	}
}

// String renders the role.
func (r Role) String() string { return string(r) }
