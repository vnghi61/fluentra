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
) ([]usercontract.UserSummary, string, error) {
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
	return list, "", nil
}

func (f *fakeUserReader) GetUserByID(_ context.Context, id uuid.UUID) (*usercontract.UserDetail, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, admindomain.ErrUserNotFound
	}
	return u, nil
}

type fakeUserManager struct {
	suspended  map[uuid.UUID]bool
	reinstated map[uuid.UUID]bool
}

func (f *fakeUserManager) SuspendUser(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.suspended[id] = true
	return nil
}

func (f *fakeUserManager) ReinstateUser(_ context.Context, id uuid.UUID, _ uuid.UUID, _ string) error {
	f.reinstated[id] = true
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
