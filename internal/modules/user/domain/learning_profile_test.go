package domain_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/user/domain"
)

func TestLearningProfile_Validate(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	validProfile := func() domain.LearningProfile {
		return domain.LearningProfile{
			ID:                uuid.New(),
			UserID:            uuid.New(),
			DeclaredLevel:     strPtr("B1"),
			TargetLevel:       strPtr("B2"),
			TargetExam:        domain.TargetExamIELTS,
			WeeklyMinutesGoal: intPtr(120),
			Motivations:       []string{"career", "travel"},
		}
	}

	t.Run("valid profile passes", func(t *testing.T) {
		lp := validProfile()
		if err := lp.Validate(); err != nil {
			t.Fatalf("expected valid profile, got: %v", err)
		}
	})

	t.Run("null levels and minutes pass", func(t *testing.T) {
		lp := domain.LearningProfile{
			ID:          uuid.New(),
			UserID:      uuid.New(),
			TargetExam:  domain.TargetExamNone,
			Motivations: []string{},
		}
		if err := lp.Validate(); err != nil {
			t.Fatalf("expected valid profile with nulls, got: %v", err)
		}
	})

	t.Run("invalid declared level fails", func(t *testing.T) {
		lp := validProfile()
		lp.DeclaredLevel = strPtr("X1")
		err := lp.Validate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertFieldCode(t, err, "declared_level", "INVALID_CEFR_LEVEL")
	})

	t.Run("invalid target level fails", func(t *testing.T) {
		lp := validProfile()
		lp.TargetLevel = strPtr("Z9")
		err := lp.Validate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertFieldCode(t, err, "target_level", "INVALID_CEFR_LEVEL")
	})

	t.Run("invalid target exam fails", func(t *testing.T) {
		lp := validProfile()
		lp.TargetExam = "toefl"
		err := lp.Validate()
		if err == nil {
			t.Fatal("expected error for invalid target exam")
		}
		assertFieldCode(t, err, "target_exam", "INVALID_TARGET_EXAM")
	})

	t.Run("minutes below 15 fails", func(t *testing.T) {
		lp := validProfile()
		lp.WeeklyMinutesGoal = intPtr(10)
		err := lp.Validate()
		if err == nil {
			t.Fatal("expected error for < 15 minutes")
		}
		assertFieldCode(t, err, "weekly_minutes_goal", "OUT_OF_RANGE")
	})

	t.Run("minutes above 10080 fails", func(t *testing.T) {
		lp := validProfile()
		lp.WeeklyMinutesGoal = intPtr(10081)
		err := lp.Validate()
		if err == nil {
			t.Fatal("expected error for > 10080 minutes")
		}
		assertFieldCode(t, err, "weekly_minutes_goal", "OUT_OF_RANGE")
	})

	t.Run("more than 10 motivations fails", func(t *testing.T) {
		lp := validProfile()
		lp.Motivations = make([]string, 11)
		for i := range lp.Motivations {
			lp.Motivations[i] = "item"
		}
		err := lp.Validate()
		if err == nil {
			t.Fatal("expected error for > 10 motivations")
		}
		assertFieldCode(t, err, "motivations", "TOO_MANY_ITEMS")
	})
}
