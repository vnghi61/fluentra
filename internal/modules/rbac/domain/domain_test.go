package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// roleAdminName is the wire form of contract.RoleAdmin, asserted on directly
// where the point of the test is that the string is rejected as a permission.
const roleAdminName = "admin"

// wildcardPermission is not a permission. It appears in several tests because
// a glob is the shape somebody reaches for when they want "everything", and
// the catalogue has no such thing.
const wildcardPermission = "rbac.*"

var (
	actorID  = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	targetID = uuid.MustParse("0199b2d3-4e5f-7a81-8bcd-ef0123456789")
)

// TestPermissionSet_ZeroValueGrantsNothing is the trap from the P1.3 card. The
// claim is that deny-by-default is structural, and the structure is this: the
// zero value of the type the guard consults is an empty set.
func TestPermissionSet_ZeroValueGrantsNothing(t *testing.T) {
	t.Parallel()

	var uninitialised domain.PermissionSet
	if uninitialised.Len() != 0 {
		t.Fatalf("the zero PermissionSet has %d entries, want 0", uninitialised.Len())
	}
	for _, permission := range contract.All() {
		if uninitialised.Has(permission) {
			t.Errorf("the zero PermissionSet grants %s", permission)
		}
	}
	// Including the ones that are not permissions at all.
	for _, raw := range []contract.Permission{"", roleAdminName, "*", wildcardPermission} {
		if uninitialised.Has(raw) {
			t.Errorf("the zero PermissionSet grants %q", raw)
		}
	}
}

// TestPermissionSet_MalformedPermissionIsNeverGranted covers the other half:
// a set that somehow contains a malformed name must still refuse it, because
// `Permission("")` is a value the type system allows a caller to construct.
func TestPermissionSet_MalformedPermissionIsNeverGranted(t *testing.T) {
	t.Parallel()

	// Deliberately seeded with rubbish, as a corrupted cache entry would be.
	granted := domain.NewPermissionSet([]contract.Permission{"", roleAdminName, wildcardPermission, "RBAC.READ"})
	for _, permission := range []contract.Permission{"", roleAdminName, wildcardPermission, "RBAC.READ"} {
		if granted.Has(permission) {
			t.Errorf("a malformed permission %q was granted", permission)
		}
	}
}

func TestPermissionSet_GrantsWhatItWasGiven(t *testing.T) {
	t.Parallel()

	granted := domain.NewPermissionSet([]contract.Permission{
		contract.PermRBACRead, contract.PermUserList,
	})
	if !granted.Has(contract.PermRBACRead) || !granted.Has(contract.PermUserList) {
		t.Fatal("a granted permission was refused")
	}
	if granted.Has(contract.PermRBACAssign) {
		t.Error("a permission that was never granted was allowed")
	}

	names := granted.Names()
	if len(names) != 2 || names[0] != contract.PermRBACRead || names[1] != contract.PermUserList {
		t.Errorf("Names() = %v, want them sorted", names)
	}
}

func TestPermission_ValidRejectsAnythingOutsideTheNamingRule(t *testing.T) {
	t.Parallel()

	for _, permission := range contract.All() {
		if !permission.Valid() {
			t.Errorf("catalogue permission %q is not valid by its own rule", permission)
		}
	}
	for _, raw := range []contract.Permission{
		"", roleAdminName, "rbac", "rbac.", ".read", "rbac..read", "RBAC.read",
		"rbac.read.extra.deep", "rbac read", wildcardPermission, "1rbac.read",
	} {
		if raw.Valid() {
			t.Errorf("%q was accepted as a permission name", raw)
		}
	}
}

func TestParseRole_AcceptsExactlyTwo(t *testing.T) {
	t.Parallel()

	for _, name := range []string{roleAdminName, "user"} {
		if _, ok := contract.ParseRole(name); !ok {
			t.Errorf("ParseRole(%q) was rejected", name)
		}
	}
	// BR-RBAC-02: a third role is an ADR, not a request body.
	for _, name := range []string{"", "ADMIN", "superadmin", "moderator", "root"} {
		if _, ok := contract.ParseRole(name); ok {
			t.Errorf("ParseRole(%q) was accepted; there are exactly two roles", name)
		}
	}
}

func TestRoleChange_ValidateGrant(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change domain.RoleChange
		want   error
	}{
		"granting somebody else is fine": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: targetID, Role: contract.RoleAdmin,
				ActorRoles: []contract.Role{contract.RoleAdmin},
			},
		},
		"granting yourself a role you already hold is a no-op": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: actorID, Role: contract.RoleAdmin,
				ActorRoles: []contract.Role{contract.RoleAdmin},
			},
		},
		"granting yourself a role you do not hold is elevation": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: actorID, Role: contract.RoleAdmin,
				ActorRoles: []contract.Role{contract.RoleUser},
			},
			want: domain.ErrSelfElevationForbidden,
		},
	}

	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			err := testCase.change.ValidateGrant()
			if testCase.want == nil {
				if err != nil {
					t.Fatalf("ValidateGrant() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, testCase.want) {
				t.Fatalf("ValidateGrant() = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestRoleChange_ValidateRevoke(t *testing.T) {
	t.Parallel()

	admin := []contract.Role{contract.RoleAdmin}

	cases := map[string]struct {
		change     domain.RoleChange
		adminCount int
		want       error
	}{
		"revoking a non-admin role is unguarded": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: targetID, Role: contract.RoleUser,
				TargetRoles: []contract.Role{contract.RoleUser},
			},
			adminCount: 1,
		},
		"an admin cannot demote themselves even with others present": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: actorID, Role: contract.RoleAdmin,
				ActorRoles: admin, TargetRoles: admin,
			},
			adminCount: 5,
			want:       domain.ErrSelfDemotionForbidden,
		},
		"the last admin cannot be demoted": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: targetID, Role: contract.RoleAdmin,
				ActorRoles: admin, TargetRoles: admin,
			},
			adminCount: 1,
			want:       domain.ErrLastAdminProtected,
		},
		"the second-to-last admin can be": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: targetID, Role: contract.RoleAdmin,
				ActorRoles: admin, TargetRoles: admin,
			},
			adminCount: 2,
		},
		"revoking admin from somebody who is not one changes no count": {
			change: domain.RoleChange{
				ActorID: actorID, TargetID: targetID, Role: contract.RoleAdmin,
				ActorRoles: admin, TargetRoles: []contract.Role{contract.RoleUser},
			},
			adminCount: 1,
		},
	}

	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			err := testCase.change.ValidateRevoke(testCase.adminCount)
			if testCase.want == nil {
				if err != nil {
					t.Fatalf("ValidateRevoke() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, testCase.want) {
				t.Fatalf("ValidateRevoke() = %v, want %v", err, testCase.want)
			}
		})
	}
}

// TestErrorsCarryTheDocumentedStatuses checks the codes in AGENT.md §12 against
// what a client would actually receive.
func TestErrorsCarryTheDocumentedStatuses(t *testing.T) {
	t.Parallel()

	cases := map[error]struct {
		code   string
		status int
	}{
		domain.ErrPermissionDenied:       {code: "PERMISSION_DENIED", status: 403},
		domain.ErrSelfElevationForbidden: {code: "SELF_ELEVATION_FORBIDDEN", status: 403},
		domain.ErrSelfDemotionForbidden:  {code: "SELF_DEMOTION_FORBIDDEN", status: 403},
		domain.ErrLastAdminProtected:     {code: "LAST_ADMIN_PROTECTED", status: 409},
	}
	for err, want := range cases {
		var appErr *apperr.Error
		if !errors.As(err, &appErr) {
			t.Fatalf("%v is not an apperr.Error", err)
		}
		if appErr.Code != want.code {
			t.Errorf("code = %q, want %q", appErr.Code, want.code)
		}
		if appErr.Status() != want.status {
			t.Errorf("%s status = %d, want %d", want.code, appErr.Status(), want.status)
		}
	}
}
