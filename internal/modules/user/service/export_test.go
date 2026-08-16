package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
	"github.com/fluentra/fluentra/internal/modules/user/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

func TestRequestExport_Success(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	enqueuer := &fakeEnqueuer{}
	serviceWithEnqueuer := service.New(service.Deps{
		Pool:   h.pool,
		Repo:   h.repo,
		Events: h.events,
		Clock:  clock.NewFake(testNow),
		NewID: func(context.Context) (uuid.UUID, error) {
			return uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def999999999"), nil
		},
		Enqueuer: enqueuer,
	})

	ctx := context.Background()
	req, err := serviceWithEnqueuer.RequestExport(ctx, h.actor)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.ID != uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def999999999") {
		t.Errorf("unexpected export ID: %v", req.ID)
	}
	if req.Status != domain.ExportStatusPending {
		t.Errorf("expected pending status, got: %v", req.Status)
	}
	if len(enqueuer.enqueued) != 1 {
		t.Errorf("expected 1 enqueued job, got: %d", len(enqueuer.enqueued))
	}
}

func TestRequestExport_AlreadyPendingReturns409(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	enqueuer := &fakeEnqueuer{}
	serviceWithEnqueuer := service.New(service.Deps{
		Pool:     h.pool,
		Repo:     h.repo,
		Events:   h.events,
		Clock:    clock.NewFake(testNow),
		NewID:    func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		Enqueuer: enqueuer,
	})

	ctx := context.Background()
	// First request succeeds
	_, err := serviceWithEnqueuer.RequestExport(ctx, h.actor)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request while first is pending must fail with ErrExportAlreadyPending
	_, err = serviceWithEnqueuer.RequestExport(ctx, h.actor)
	if err == nil {
		t.Fatal("expected error for duplicate export request, got nil")
	}
	if !errors.Is(err, domain.ErrExportAlreadyPending) {
		t.Errorf("expected ErrExportAlreadyPending, got: %v", err)
	}
}

func TestRequestExport_AccountNotUsable(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.repo.users[h.actor] = domain.User{
		ID:     h.actor,
		Email:  "suspended@example.com",
		Status: domain.StatusSuspended,
	}

	ctx := context.Background()
	_, err := h.service.RequestExport(ctx, h.actor)
	if err == nil {
		t.Fatal("expected error for suspended account, got nil")
	}
	if !errors.Is(err, domain.ErrAccountNotUsable) {
		t.Errorf("expected ErrAccountNotUsable, got: %v", err)
	}
}

func TestGetExportByID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	exportID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def111111111")
	otherUser := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def222222222")

	h.repo.exports[exportID] = domain.ExportRequest{
		ID:        exportID,
		UserID:    h.actor,
		Status:    domain.ExportStatusCompleted,
		CreatedAt: testNow,
	}

	ctx := context.Background()

	// Owner can read export
	req, err := h.service.GetExportByID(ctx, h.actor, exportID)
	if err != nil {
		t.Fatalf("expected no error reading own export, got: %v", err)
	}
	if req.ID != exportID {
		t.Errorf("unexpected export ID: %v", req.ID)
	}

	// Another user reading this export gets ErrUserNotFound (ownership privacy)
	_, err = h.service.GetExportByID(ctx, otherUser, exportID)
	if err == nil {
		t.Fatal("expected error reading other user's export, got nil")
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestExportUserData(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	dob := time.Date(1995, time.May, 20, 0, 0, 0, 0, time.UTC)
	h.repo.profiles[h.actor] = domain.Profile{
		UserID:      h.actor,
		DisplayName: "Test Learner",
		Timezone:    "UTC",
		DateOfBirth: &dob,
	}

	ctx := context.Background()
	data, err := h.service.ExportUserData(ctx, h.actor.String())
	if err != nil {
		t.Fatalf("ExportUserData failed: %v", err)
	}

	if data["email"] != "learner@example.com" {
		t.Errorf("expected email 'learner@example.com', got: %v", data["email"])
	}
	profileData, ok := data["profile"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected profile map in export data")
	}
	if profileData["display_name"] != "Test Learner" {
		t.Errorf("unexpected display_name: %v", profileData["display_name"])
	}
}
