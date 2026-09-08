package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
)

type fakeUserReader struct {
	users map[uuid.UUID]*usercontract.UserDetail
}

func (f *fakeUserReader) SearchUsers(
	_ context.Context,
	_ usercontract.UserFilter,
	_ string,
	_ int,
) (usercontract.UserPage, error) {
	var list []usercontract.UserSummary
	for id, u := range f.users {
		list = append(list, usercontract.UserSummary{
			ID:          id,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Status:      u.Status,
			CreatedAt:   u.CreatedAt,
		})
	}
	return usercontract.UserPage{Items: list, Total: len(list)}, nil
}

func (f *fakeUserReader) GetUserByID(_ context.Context, id uuid.UUID) (*usercontract.UserDetail, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, admindomain.ErrUserNotFound
	}
	return u, nil
}

type fakeUserManager struct {
	suspended   map[uuid.UUID]bool
	reinstated  map[uuid.UUID]bool
	softDeleted map[uuid.UUID]bool
}

func (f *fakeUserManager) SuspendUser(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.suspended[id] = true
	return nil
}

func (f *fakeUserManager) ReinstateUser(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.reinstated[id] = true
	return nil
}

func (f *fakeUserManager) SoftDeleteUser(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	if f.softDeleted == nil {
		f.softDeleted = make(map[uuid.UUID]bool)
	}
	f.softDeleted[id] = true
	return nil
}

type fakeSessionRevoker struct {
	revoked map[uuid.UUID]bool
}

func (f *fakeSessionRevoker) RevokeAll(_ context.Context, userID uuid.UUID) (int, error) {
	f.revoked[userID] = true
	return 1, nil
}

// failingSessionRevoker always fails, for the case where the admin is told the
// suspension succeeded while the sessions are still live.
type failingSessionRevoker struct{}

func (f failingSessionRevoker) RevokeAll(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, errors.New("revoke failed")
}

func TestSelfSuspensionRefused(t *testing.T) {
	adminID := uuid.New()
	uReader := &fakeUserReader{users: make(map[uuid.UUID]*usercontract.UserDetail)}
	uMgr := &fakeUserManager{suspended: make(map[uuid.UUID]bool), reinstated: make(map[uuid.UUID]bool)}
	revoker := &fakeSessionRevoker{revoked: make(map[uuid.UUID]bool)}

	svc := adminsvc.New(adminsvc.Deps{
		UserReader:     uReader,
		UserManager:    uMgr,
		SessionRevoker: revoker,
	})

	err := svc.SuspendUser(context.Background(), adminID, adminID, "A valid reason for self suspension")
	if err == nil {
		t.Fatalf("expected error when admin suspends self, got nil")
	}
	if !errors.Is(err, admindomain.ErrSelfAdminActionForbidden) {
		t.Fatalf("expected ErrSelfAdminActionForbidden, got %v", err)
	}
}

func TestSelfRevocationRefused(t *testing.T) {
	adminID := uuid.New()
	uReader := &fakeUserReader{users: make(map[uuid.UUID]*usercontract.UserDetail)}
	uMgr := &fakeUserManager{suspended: make(map[uuid.UUID]bool), reinstated: make(map[uuid.UUID]bool)}
	revoker := &fakeSessionRevoker{revoked: make(map[uuid.UUID]bool)}

	svc := adminsvc.New(adminsvc.Deps{
		UserReader:     uReader,
		UserManager:    uMgr,
		SessionRevoker: revoker,
	})

	err := svc.RevokeUserSessions(context.Background(), adminID, adminID, "A valid reason for self session revocation")
	if err == nil {
		t.Fatalf("expected error when admin revokes own sessions, got nil")
	}
	if !errors.Is(err, admindomain.ErrSelfAdminActionForbidden) {
		t.Fatalf("expected ErrSelfAdminActionForbidden, got %v", err)
	}
	if revoker.revoked[adminID] {
		t.Fatalf("expected SessionRevoker.RevokeAll NOT to be called")
	}
}

func TestReasonRequirement(t *testing.T) {
	adminID := uuid.New()
	targetID := uuid.New()
	uReader := &fakeUserReader{users: make(map[uuid.UUID]*usercontract.UserDetail)}
	uMgr := &fakeUserManager{suspended: make(map[uuid.UUID]bool), reinstated: make(map[uuid.UUID]bool)}
	revoker := &fakeSessionRevoker{revoked: make(map[uuid.UUID]bool)}

	svc := adminsvc.New(adminsvc.Deps{
		UserReader:     uReader,
		UserManager:    uMgr,
		SessionRevoker: revoker,
	})

	err := svc.SuspendUser(context.Background(), adminID, targetID, "short")
	if err == nil {
		t.Fatalf("expected error for reason < 10 chars, got nil")
	}
}

// TestSuspendUserPropagatesSessionRevocationError is REVIEW-FIXES-P4.1 Issue 4:
// a RevokeAll failure must fail the suspension rather than silently succeed, or
// the admin is told the account is suspended while its sessions stay live.
func TestSuspendUserPropagatesSessionRevocationError(t *testing.T) {
	adminID := uuid.New()
	targetID := uuid.New()
	uMgr := &fakeUserManager{suspended: make(map[uuid.UUID]bool), reinstated: make(map[uuid.UUID]bool)}

	svc := adminsvc.New(adminsvc.Deps{
		Repo:           &fakeFlagRepo{},
		UserManager:    uMgr,
		SessionRevoker: failingSessionRevoker{},
	})

	err := svc.SuspendUser(context.Background(), adminID, targetID, "A valid reason for suspension")
	if err == nil {
		t.Fatal("expected session revocation error to propagate, got nil")
	}
}

// newSoftDeleteFixture builds the three fakes the deletion tests share.
func newSoftDeleteFixture() (*fakeUserManager, *fakeSessionRevoker, *adminsvc.Service) {
	uMgr := &fakeUserManager{
		suspended:   make(map[uuid.UUID]bool),
		reinstated:  make(map[uuid.UUID]bool),
		softDeleted: make(map[uuid.UUID]bool),
	}
	revoker := &fakeSessionRevoker{revoked: make(map[uuid.UUID]bool)}
	svc := adminsvc.New(adminsvc.Deps{
		// Repo is not optional here. The two refusal tests below return before
		// they reach it, so a nil repo looks fine until the first test that
		// actually completes an action — which is the one this fixture exists for.
		Repo:           &fakeFlagRepo{},
		UserReader:     &fakeUserReader{users: make(map[uuid.UUID]*usercontract.UserDetail)},
		UserManager:    uMgr,
		SessionRevoker: revoker,
	})
	return uMgr, revoker, svc
}

// TestSelfSoftDeleteRefused. The self-service deletion path exists and has its
// own confirmations; reaching the same state through the admin endpoint would
// skip them, and would let an administrator remove the account holding the
// permission that is removing it.
func TestSelfSoftDeleteRefused(t *testing.T) {
	adminID := uuid.New()
	uMgr, _, svc := newSoftDeleteFixture()

	err := svc.SoftDeleteUser(context.Background(), adminID, adminID, "A valid reason for deleting my own account")
	if !errors.Is(err, admindomain.ErrSelfAdminActionForbidden) {
		t.Fatalf("expected ErrSelfAdminActionForbidden, got %v", err)
	}
	if uMgr.softDeleted[adminID] {
		t.Fatal("the account was soft-deleted despite the refusal")
	}
}

// TestSoftDeleteRequiresAReason holds soft deletion to the same bar as
// suspension. It is the stricter of the two acts, so a shorter justification
// than suspension needs would be the wrong way round.
func TestSoftDeleteRequiresAReason(t *testing.T) {
	adminID, targetID := uuid.New(), uuid.New()
	uMgr, _, svc := newSoftDeleteFixture()

	err := svc.SoftDeleteUser(context.Background(), adminID, targetID, "too short")
	if !errors.Is(err, admindomain.ErrReasonRequired) {
		t.Fatalf("expected ErrReasonRequired, got %v", err)
	}
	if uMgr.softDeleted[targetID] {
		t.Fatal("the account was soft-deleted without a reason")
	}
}

// TestSoftDeleteRevokesSessions. The endpoint's promise is that the account
// cannot be signed into once it returns. user.deletion_requested asks for the
// same thing downstream, but the outbox is asynchronous and a promise that
// depends on a worker having run is not a promise.
func TestSoftDeleteRevokesSessions(t *testing.T) {
	adminID, targetID := uuid.New(), uuid.New()
	uMgr, revoker, svc := newSoftDeleteFixture()

	reason := "Account closure requested by the owner over verified support email"
	if err := svc.SoftDeleteUser(context.Background(), adminID, targetID, reason); err != nil {
		t.Fatalf("SoftDeleteUser: %v", err)
	}
	if !uMgr.softDeleted[targetID] {
		t.Fatal("the account was not soft-deleted")
	}
	if !revoker.revoked[targetID] {
		t.Fatal("sessions survived a soft delete; the account can still be signed into")
	}
}
