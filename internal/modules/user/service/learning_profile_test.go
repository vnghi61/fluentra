package service_test

import (
	"context"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

func TestService_LearningProfileLifecycle(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ctx := context.Background()

	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	// 1. Initial read: no profile exists yet
	_, err := h.service.GetLearningProfile(ctx, h.actor)
	if err == nil || err != domain.ErrLearningProfileNotFound {
		t.Fatalf("expected ErrLearningProfileNotFound, got: %v", err)
	}

	dto, found, err := h.service.LearningProfileReader().GetLearningProfile(ctx, h.actor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatalf("expected found=false for new user, got: %+v", dto)
	}

	// 2. Replace (create) learning profile
	wanted := domain.LearningProfile{
		DeclaredLevel:     strPtr("B1"),
		TargetLevel:       strPtr("B2"),
		TargetExam:        domain.TargetExamIELTS,
		WeeklyMinutesGoal: intPtr(120),
		Motivations:       []string{"career", "travel"},
	}

	saved, err := h.service.ReplaceLearningProfile(ctx, h.actor, wanted)
	if err != nil {
		t.Fatalf("ReplaceLearningProfile failed: %v", err)
	}
	if saved.UserID != h.actor {
		t.Errorf("saved.UserID = %v, want %v", saved.UserID, h.actor)
	}
	if saved.TargetExam != domain.TargetExamIELTS {
		t.Errorf("saved.TargetExam = %v, want %v", saved.TargetExam, domain.TargetExamIELTS)
	}

	// Verify event emitted
	if len(h.events.written) != 1 {
		t.Fatalf("events written = %d, want 1", len(h.events.written))
	}
	evt := h.events.written[0]
	if evt.Event != contract.EventLearningProfileUpdated {
		t.Errorf("event name = %q, want %q", evt.Event, contract.EventLearningProfileUpdated)
	}
	payload, ok := evt.Payload.(contract.LearningProfileUpdated)
	if !ok {
		t.Fatalf("event payload type = %T, want LearningProfileUpdated", evt.Payload)
	}
	if payload.UserID != h.actor {
		t.Errorf("payload.UserID = %v, want %v", payload.UserID, h.actor)
	}

	// 3. Read profile after creation
	domProfile, err := h.service.GetLearningProfile(ctx, h.actor)
	if err != nil {
		t.Fatalf("GetLearningProfile failed: %v", err)
	}
	if domProfile.TargetExam != domain.TargetExamIELTS {
		t.Errorf("domProfile.TargetExam = %v, want ielts", domProfile.TargetExam)
	}

	dto, found, err = h.service.LearningProfileReader().GetLearningProfile(ctx, h.actor)
	if err != nil {
		t.Fatalf("GetLearningProfile failed: %v", err)
	}
	if !found {
		t.Fatal("expected found=true after replacement")
	}
	if dto.TargetExam != "ielts" {
		t.Errorf("dto.TargetExam = %v, want ielts", dto.TargetExam)
	}
	if dto.DeclaredLevel == nil || *dto.DeclaredLevel != "B1" {
		t.Errorf("dto.DeclaredLevel = %v, want B1", dto.DeclaredLevel)
	}

	// 4. Suspended user cannot update learning profile
	suspendedUser := h.repo.users[h.actor]
	suspendedUser.Status = domain.StatusSuspended
	h.repo.users[h.actor] = suspendedUser

	_, err = h.service.ReplaceLearningProfile(ctx, h.actor, wanted)
	if err == nil {
		t.Fatal("expected error for suspended user, got nil")
	}
	if err != domain.ErrAccountNotUsable {
		t.Errorf("err = %v, want ErrAccountNotUsable", err)
	}
}
