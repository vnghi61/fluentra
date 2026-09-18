package service_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

func learningProfileWithGoal(goal int) domain.LearningProfile {
	declared, target := "B1", "B2"
	return domain.LearningProfile{
		DeclaredLevel:     &declared,
		TargetLevel:       &target,
		TargetExam:        domain.TargetExamIELTS,
		WeeklyMinutesGoal: &goal,
		Motivations:       []string{"career", "travel"},
	}
}

func TestService_ALearningProfileIsNotFoundBeforeItIsSaved(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.GetLearningProfile(ctx, h.actor); !errors.Is(err, domain.ErrLearningProfileNotFound) {
		t.Fatalf("expected ErrLearningProfileNotFound, got: %v", err)
	}
	dto, found, err := h.service.LearningProfileReader().GetLearningProfile(ctx, h.actor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatalf("the reader reports found=false rather than an error, got: %+v", dto)
	}
}

func TestService_ReplacingALearningProfileStoresItAndNamesEveryField(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	saved, err := h.service.ReplaceLearningProfile(ctx, h.actor, learningProfileWithGoal(120))
	if err != nil {
		t.Fatalf("ReplaceLearningProfile failed: %v", err)
	}
	if saved.UserID != h.actor || saved.TargetExam != domain.TargetExamIELTS {
		t.Errorf("saved = %+v, want the caller's IELTS profile", saved)
	}

	if len(h.events.written) != 1 || h.events.written[0].Event != contract.EventLearningProfileUpdated {
		t.Fatalf("events = %+v, want one %s", h.events.written, contract.EventLearningProfileUpdated)
	}
	payload, ok := h.events.written[0].Payload.(contract.LearningProfileUpdated)
	if !ok {
		t.Fatalf("event payload type = %T, want LearningProfileUpdated", h.events.written[0].Payload)
	}
	if payload.UserID != h.actor || len(payload.ChangedFields) != 5 {
		t.Errorf("payload = %+v, want the caller and every field for a new profile", payload)
	}

	dto, found, err := h.service.LearningProfileReader().GetLearningProfile(ctx, h.actor)
	if err != nil || !found {
		t.Fatalf("reader after save: found=%v err=%v", found, err)
	}
	if dto.TargetExam != "ielts" || dto.DeclaredLevel == nil || *dto.DeclaredLevel != "B1" {
		t.Errorf("dto = %+v, want ielts and B1", dto)
	}
}

func TestService_AReplacementNamesOnlyWhatChangedAndNothingWhenNothingDid(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.service.ReplaceLearningProfile(ctx, h.actor, learningProfileWithGoal(120)); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	if _, err := h.service.ReplaceLearningProfile(ctx, h.actor, learningProfileWithGoal(150)); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if len(h.events.written) != 2 {
		t.Fatalf("events written = %d, want 2", len(h.events.written))
	}
	second, _ := h.events.written[1].Payload.(contract.LearningProfileUpdated)
	if !slices.Equal(second.ChangedFields, []string{"weekly_minutes_goal"}) {
		t.Errorf("changed fields = %v, want [weekly_minutes_goal]", second.ChangedFields)
	}

	if _, err := h.service.ReplaceLearningProfile(ctx, h.actor, learningProfileWithGoal(150)); err != nil {
		t.Fatalf("third replace: %v", err)
	}
	if len(h.events.written) != 2 {
		t.Errorf("an unchanged profile published an event")
	}
}

func TestService_ASuspendedLearnerCannotReplaceTheirLearningProfile(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	suspended := h.repo.users[h.actor]
	suspended.Status = domain.StatusSuspended
	h.repo.users[h.actor] = suspended

	_, err := h.service.ReplaceLearningProfile(context.Background(), h.actor, learningProfileWithGoal(120))
	if !errors.Is(err, domain.ErrAccountNotUsable) {
		t.Errorf("err = %v, want ErrAccountNotUsable", err)
	}
}
