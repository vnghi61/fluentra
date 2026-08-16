package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

func TestRequestDeletion_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	req, err := h.service.RequestDeletion(ctx, h.actor)
	if err != nil {
		t.Fatalf("RequestDeletion: %v", err)
	}

	if req.UserID != h.actor {
		t.Errorf("got user_id %s, want %s", req.UserID, h.actor)
	}
	if req.Status != domain.DeletionStatusPending {
		t.Errorf("got status %s, want pending", req.Status)
	}
	expectedExecuteAt := testNow.Add(domain.DeletionGracePeriod)
	if !req.ExecuteAt.Equal(expectedExecuteAt) {
		t.Errorf("got execute_at %s, want %s", req.ExecuteAt, expectedExecuteAt)
	}

	// Verify user status changed to pending_deletion
	user, err := h.repo.GetUser(ctx, h.actor)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Status != domain.StatusPendingDeletion {
		t.Errorf("got user status %s, want pending_deletion", user.Status)
	}

	// Verify outbox event written
	events := h.events.events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Event != contract.EventDeletionRequested {
		t.Errorf("got event topic %s, want %s", events[0].Event, contract.EventDeletionRequested)
	}
}

func TestRequestDeletion_AlreadyPending_Conflict(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	// First request succeeds
	_, err := h.service.RequestDeletion(ctx, h.actor)
	if err != nil {
		t.Fatalf("first RequestDeletion: %v", err)
	}

	// Second request fails with conflict
	_, err = h.service.RequestDeletion(ctx, h.actor)
	if !errors.Is(err, domain.ErrDeletionAlreadyPending) {
		t.Fatalf("got error %v, want ErrDeletionAlreadyPending", err)
	}
}

func TestCancelDeletion_Success(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	// First request deletion
	_, err := h.service.RequestDeletion(ctx, h.actor)
	if err != nil {
		t.Fatalf("RequestDeletion: %v", err)
	}

	// Cancel deletion
	cancelled, err := h.service.CancelDeletion(ctx, h.actor)
	if err != nil {
		t.Fatalf("CancelDeletion: %v", err)
	}
	if cancelled.Status != domain.DeletionStatusCancelled {
		t.Errorf("got status %s, want cancelled", cancelled.Status)
	}

	// Verify user status restored to active
	user, err := h.repo.GetUser(ctx, h.actor)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Status != domain.StatusActive {
		t.Errorf("got user status %s, want active", user.Status)
	}
}

func TestCancelDeletion_NotPending_Conflict(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	// Not pending deletion yet
	_, err := h.service.CancelDeletion(ctx, h.actor)
	if !errors.Is(err, domain.ErrDeletionNotCancellable) {
		t.Fatalf("got error %v, want ErrDeletionNotCancellable", err)
	}
}

func TestGetDeletion_SuccessAndPermissions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()
	otherUser := uuid.New()

	req, err := h.service.RequestDeletion(ctx, h.actor)
	if err != nil {
		t.Fatalf("RequestDeletion: %v", err)
	}

	// Owner can read
	got, err := h.service.GetDeletion(ctx, h.actor, req.ID)
	if err != nil {
		t.Fatalf("GetDeletion: %v", err)
	}
	if got.ID != req.ID {
		t.Errorf("got deletion id %s, want %s", got.ID, req.ID)
	}

	// Other user cannot read
	_, err = h.service.GetDeletion(ctx, otherUser, req.ID)
	if !errors.Is(err, domain.ErrDeletionNotFound) {
		t.Fatalf("got error %v, want ErrDeletionNotFound for other user", err)
	}
}
