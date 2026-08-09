package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	"github.com/fluentra/fluentra/internal/modules/rbac/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

var (
	adminID   = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	learnerID = uuid.MustParse("0199b2d3-4e5f-7a81-8bcd-ef0123456789")
	otherID   = uuid.MustParse("0199c3e4-5f60-7b82-9cde-f01234567890")
	testNow   = time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)
)

type harness struct {
	service *service.Service
	repo    *fakeRepo
	cache   *fakeCache
	events  *recordingEvents
	pool    *fakeBeginner
}

// newHarness builds the service over fakes, with one admin and one learner.
func newHarness(t *testing.T) *harness {
	t.Helper()

	repo := newFakeRepo()
	repo.assignments[adminID] = []contract.Role{contract.RoleAdmin, contract.RoleUser}
	repo.assignments[learnerID] = []contract.Role{contract.RoleUser}
	repo.assignments[otherID] = []contract.Role{contract.RoleUser}

	cache := newFakeCache()
	events := &recordingEvents{}
	pool := &fakeBeginner{}

	roles := service.New(service.Deps{
		Pool: pool, Repo: repo, Cache: cache, Events: events,
		Clock: clock.NewFake(testNow), Env: "test",
	})
	return &harness{service: roles, repo: repo, cache: cache, events: events, pool: pool}
}

func asActor(id uuid.UUID) context.Context {
	return httpx.WithActor(context.Background(), httpx.Actor{UserID: id, Role: "user"})
}

// TestRequire_DeniesWithoutAnActor is the first of the four ways to be
// refused. A context with no caller must not resolve to an empty-but-present
// actor whose permissions happen to be looked up for the nil uuid.
func TestRequire_DeniesWithoutAnActor(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	err := h.service.Require(context.Background(), contract.PermRBACRead)
	if !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("error = %v, want forbidden", err)
	}
	if h.repo.callCount("PermissionsOf") != 0 {
		t.Error("an unauthenticated request still queried the permission set")
	}
}

// TestRequire_DeniesAPermissionNobodyHolds is the ordinary case, and the one
// the card names: an operation whose permission is not granted is refused.
func TestRequire_DeniesAPermissionNobodyHolds(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	if err := h.service.Require(asActor(learnerID), contract.PermRBACAssign); !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("a learner was allowed rbac.assign: %v", err)
	}
	if err := h.service.Require(asActor(adminID), contract.PermRBACAssign); err != nil {
		t.Fatalf("an admin was refused rbac.assign: %v", err)
	}
}

// TestRequire_DeniesAnUndeclaredPermission is BR-RBAC-01 stated exactly: an
// operation with no declared permission is refused, not allowed. The empty
// permission is what a forgotten constant looks like at the call site.
func TestRequire_DeniesAnUndeclaredPermission(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	// The admin holds every permission in the catalogue. Even so:
	for _, undeclared := range []contract.Permission{"", "admin", "rbac.*", "not.in.catalogue"} {
		if err := h.service.Require(asActor(adminID), undeclared); err == nil {
			t.Errorf("Require(%q) allowed an undeclared permission, for an actor holding everything", undeclared)
		}
	}
}

// TestRequire_DeniesWhenResolutionFails is the failure mode that turns an
// outage into an open door if it is got wrong.
func TestRequire_DeniesWhenResolutionFails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.repo.failOn["PermissionsOf"] = errors.New("database unreachable")

	err := h.service.Require(asActor(adminID), contract.PermRBACRead)
	if !apperr.Is(err, apperr.Forbidden) {
		t.Fatalf("error = %v, want the check to fail closed", err)
	}
}

// TestCan_MirrorsRequire keeps the convenience wrapper from drifting into a
// second, more permissive implementation.
func TestCan_MirrorsRequire(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	if !h.service.Can(asActor(adminID), contract.PermUserSuspend) {
		t.Error("Can said no for a permission Require allows")
	}
	if h.service.Can(asActor(learnerID), contract.PermUserSuspend) {
		t.Error("Can said yes for a permission Require refuses")
	}
	if h.service.Can(context.Background(), contract.PermUserSuspend) {
		t.Error("Can said yes with no actor at all")
	}
}

// TestPermissionsAreCachedAndTheCacheIsUsed shows the resolution is cached,
// which is the reason the module depends on Redis at all.
func TestPermissionsAreCachedAndTheCacheIsUsed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := asActor(adminID)

	for range 5 {
		if err := h.service.Require(ctx, contract.PermRBACRead); err != nil {
			t.Fatalf("Require: %v", err)
		}
	}
	if got := h.repo.callCount("PermissionsOf"); got != 1 {
		t.Errorf("resolved from the database %d times for 5 checks, want 1", got)
	}
	if _, cached := h.cache.stored("fluentra:test:rbac:perms:" + adminID.String() + ":v1"); !cached {
		t.Error("the resolved set was not cached under the documented key")
	}
}

// TestCacheFailureFallsBackToTheDatabase — a cache that is down makes the
// system slow, not wrong, and certainly not permissive.
func TestCacheFailureFallsBackToTheDatabase(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.cache.getErr = errors.New("redis down")
	h.cache.setErr = errors.New("redis down")

	if err := h.service.Require(asActor(adminID), contract.PermRBACRead); err != nil {
		t.Fatalf("a cache outage refused a permission the actor holds: %v", err)
	}
	if err := h.service.Require(asActor(learnerID), contract.PermRBACRead); err == nil {
		t.Fatal("a cache outage allowed a permission the actor does not hold")
	}
}

// TestRevokingARoleBustsTheCachedSet is the acceptance criterion about
// revocation taking effect immediately rather than at the end of the TTL.
func TestRevokingARoleBustsTheCachedSet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.repo.assignments[otherID] = []contract.Role{contract.RoleAdmin, contract.RoleUser}

	targetCtx := asActor(otherID)
	if err := h.service.Require(targetCtx, contract.PermRBACAssign); err != nil {
		t.Fatalf("the second admin could not use rbac.assign: %v", err)
	}
	key := "fluentra:test:rbac:perms:" + otherID.String() + ":v1"
	if _, cached := h.cache.stored(key); !cached {
		t.Fatal("the permission set was not cached before the revocation")
	}

	if _, err := h.service.RevokeRole(context.Background(), adminID, otherID, contract.RoleAdmin); err != nil {
		t.Fatalf("RevokeRole: %v", err)
	}

	if _, cached := h.cache.stored(key); cached {
		t.Error("the cached permission set survived the revocation")
	}
	// And the next check reflects reality rather than the evicted entry.
	if err := h.service.Require(targetCtx, contract.PermRBACAssign); err == nil {
		t.Error("a revoked admin still passed the permission check")
	}
}

func TestAssignRole_PublishesAndBustsOnlyWhenSomethingChanged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	roles, err := h.service.AssignRole(context.Background(), adminID, learnerID, contract.RoleAdmin)
	if err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("roles = %v, want admin and user", roles)
	}

	written := h.events.events()
	if len(written) != 1 || written[0].Event != contract.EventRoleAssigned {
		t.Fatalf("events = %+v, want one %s", written, contract.EventRoleAssigned)
	}
	payload, ok := written[0].Payload.(contract.RoleAssigned)
	if !ok {
		t.Fatalf("payload = %T, want contract.RoleAssigned", written[0].Payload)
	}
	if payload.UserID != learnerID || payload.ActorID != adminID || payload.Role != contract.RoleAdmin {
		t.Errorf("payload = %+v, want the grant it describes", payload)
	}
	bustsAfterFirst := h.cache.deleteCount()

	// Granting again changes nothing, so it must not publish or bust again —
	// an idempotent call that emits an event every time makes the audit trail
	// a log of retries rather than of changes.
	if _, err := h.service.AssignRole(context.Background(), adminID, learnerID, contract.RoleAdmin); err != nil {
		t.Fatalf("second AssignRole: %v", err)
	}
	if len(h.events.events()) != 1 {
		t.Errorf("a no-op grant published an event: %+v", h.events.events())
	}
	if h.cache.deleteCount() != bustsAfterFirst {
		t.Error("a no-op grant busted the cache")
	}
}

func TestAssignRole_RefusesSelfElevation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	_, err := h.service.AssignRole(context.Background(), learnerID, learnerID, contract.RoleAdmin)
	if err == nil {
		t.Fatal("a learner granted themselves admin")
	}
	if !errors.Is(err, domain.ErrSelfElevationForbidden) {
		t.Fatalf("error = %v, want SELF_ELEVATION_FORBIDDEN", err)
	}
	if len(h.events.events()) != 0 {
		t.Error("a refused grant published an event")
	}
	if h.pool.rollbacks == 0 {
		t.Error("the transaction was not rolled back")
	}
}

func TestAssignRole_RejectsARoleOutsideTheTwo(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	if _, err := h.service.AssignRole(context.Background(), adminID, learnerID, "superadmin"); err == nil {
		t.Fatal("a third role was accepted")
	}
	if h.pool.begins != 0 {
		t.Error("an unknown role opened a transaction")
	}
}

func TestRevokeRole_RefusesSelfDemotionAndProtectsTheLastAdmin(t *testing.T) {
	t.Parallel()

	t.Run("an admin cannot demote themselves", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.repo.assignments[otherID] = []contract.Role{contract.RoleAdmin}

		_, err := h.service.RevokeRole(context.Background(), adminID, adminID, contract.RoleAdmin)
		if !errors.Is(err, domain.ErrSelfDemotionForbidden) {
			t.Fatalf("error = %v, want SELF_DEMOTION_FORBIDDEN", err)
		}
	})

	t.Run("the last admin is protected", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		// adminID is the only admin; a second actor tries to demote them.
		_, err := h.service.RevokeRole(context.Background(), learnerID, adminID, contract.RoleAdmin)
		if !errors.Is(err, domain.ErrLastAdminProtected) {
			t.Fatalf("error = %v, want LAST_ADMIN_PROTECTED", err)
		}
		if !apperr.Is(err, apperr.Conflict) {
			t.Errorf("status is not 409: %v", err)
		}
	})

	t.Run("the second-to-last admin can be demoted", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.repo.assignments[otherID] = []contract.Role{contract.RoleAdmin}

		roles, err := h.service.RevokeRole(context.Background(), adminID, otherID, contract.RoleAdmin)
		if err != nil {
			t.Fatalf("RevokeRole: %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("roles = %v, want none left", roles)
		}
	})
}

// TestRevokeRole_CountsAdminsUnderALock proves the guard reads the count
// inside the transaction. Without the lock, two concurrent revocations each
// see two admins and both succeed, leaving none.
func TestRevokeRole_CountsAdminsUnderALock(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.repo.assignments[otherID] = []contract.Role{contract.RoleAdmin}

	if _, err := h.service.RevokeRole(context.Background(), adminID, otherID, contract.RoleAdmin); err != nil {
		t.Fatalf("RevokeRole: %v", err)
	}
	if h.repo.callCount("LockAndCountRole") != 1 {
		t.Errorf("LockAndCountRole was called %d times, want 1 per revocation",
			h.repo.callCount("LockAndCountRole"))
	}
	if h.pool.begins != 1 {
		t.Errorf("began %d transactions, want 1", h.pool.begins)
	}
}

func TestForgetUser_RemovesEveryRoleAndTheCachedSet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	// Warm the cache first, so the test shows the eviction rather than a miss.
	if err := h.service.Require(asActor(adminID), contract.PermRBACRead); err != nil {
		t.Fatalf("Require: %v", err)
	}

	if err := h.service.ForgetUser(context.Background(), adminID); err != nil {
		t.Fatalf("ForgetUser: %v", err)
	}
	if roles := h.repo.assignments[adminID]; len(roles) != 0 {
		t.Errorf("roles = %v, want none after the account was forgotten", roles)
	}
	if _, cached := h.cache.stored("fluentra:test:rbac:perms:" + adminID.String() + ":v1"); cached {
		t.Error("the cached permission set outlived the account")
	}
}

func TestPermissionsOf_ReturnsASortedFlatSet(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	permissions, err := h.service.PermissionsOf(context.Background(), adminID)
	if err != nil {
		t.Fatalf("PermissionsOf: %v", err)
	}
	if len(permissions) != len(contract.All()) {
		t.Fatalf("admin holds %d permissions, want the whole catalogue (%d)",
			len(permissions), len(contract.All()))
	}
	for index := 1; index < len(permissions); index++ {
		if permissions[index-1] >= permissions[index] {
			t.Fatalf("permissions are not sorted: %v", permissions)
		}
	}

	// A learner holds none: access to your own data is not a named permission.
	learnerPermissions, err := h.service.PermissionsOf(context.Background(), learnerID)
	if err != nil {
		t.Fatalf("PermissionsOf: %v", err)
	}
	if len(learnerPermissions) != 0 {
		t.Errorf("a learner holds %v, want none", learnerPermissions)
	}
}

func TestHasRole(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	isAdmin, err := h.service.HasRole(context.Background(), adminID, contract.RoleAdmin)
	if err != nil || !isAdmin {
		t.Errorf("HasRole(admin) = %v, %v", isAdmin, err)
	}
	isAdmin, err = h.service.HasRole(context.Background(), learnerID, contract.RoleAdmin)
	if err != nil || isAdmin {
		t.Errorf("HasRole(learner, admin) = %v, %v", isAdmin, err)
	}
}
