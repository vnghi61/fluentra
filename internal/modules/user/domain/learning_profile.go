package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TargetExam represents standard exam goals.
type TargetExam string

// The exams a learner may aim for, matching core.target_exam.
const (
	TargetExamNone  TargetExam = "none"
	TargetExamIELTS TargetExam = "ielts"
	TargetExamTOEIC TargetExam = "toeic"
)

// Allowed target exams.
var allowedTargetExams = []TargetExam{
	TargetExamNone,
	TargetExamIELTS,
	TargetExamTOEIC,
}

// Allowed CEFR levels in upper-case for learning profiles.
var allowedCEFRLevels = []string{"A1", "A2", "B1", "B2", "C1", "C2"}

// Learning profile bounds matching database check constraints.
const (
	MinWeeklyMinutesGoal = 15
	MaxWeeklyMinutesGoal = 10080
	MaxMotivations       = 10
)

// LearningProfile holds self-declared learning parameters and goals.
type LearningProfile struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	DeclaredLevel     *string
	TargetLevel       *string
	TargetExam        TargetExam
	WeeklyMinutesGoal *int
	Motivations       []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Validate ensures the learning profile obeys domain bounds and constraints.
func (lp *LearningProfile) Validate() error {
	if lp.DeclaredLevel != nil {
		normalized := strings.ToUpper(strings.TrimSpace(*lp.DeclaredLevel))
		if !slices.Contains(allowedCEFRLevels, normalized) {
			return invalid("declared_level", "INVALID_CEFR_LEVEL", "Declared level must be a valid CEFR level (A1-C2).")
		}
		lp.DeclaredLevel = &normalized
	}

	if lp.TargetLevel != nil {
		normalized := strings.ToUpper(strings.TrimSpace(*lp.TargetLevel))
		if !slices.Contains(allowedCEFRLevels, normalized) {
			return invalid("target_level", "INVALID_CEFR_LEVEL", "Target level must be a valid CEFR level (A1-C2).")
		}
		lp.TargetLevel = &normalized
	}

	exam := TargetExam(strings.ToLower(strings.TrimSpace(string(lp.TargetExam))))
	if exam == "" {
		exam = TargetExamNone
	}
	if !slices.Contains(allowedTargetExams, exam) {
		return invalid("target_exam", "INVALID_TARGET_EXAM", "Target exam must be one of: none, ielts, toeic.")
	}
	lp.TargetExam = exam

	if lp.WeeklyMinutesGoal != nil {
		goal := *lp.WeeklyMinutesGoal
		if goal < MinWeeklyMinutesGoal || goal > MaxWeeklyMinutesGoal {
			return invalid("weekly_minutes_goal", "OUT_OF_RANGE", "Weekly minutes goal must be between 15 and 10080.")
		}
	}

	if len(lp.Motivations) > MaxMotivations {
		return invalid("motivations", "TOO_MANY_ITEMS", "Motivations cannot have more than 10 items.")
	}

	cleaned := make([]string, 0, len(lp.Motivations))
	for _, m := range lp.Motivations {
		trimmed := strings.TrimSpace(m)
		if trimmed != "" && !slices.Contains(cleaned, trimmed) {
			cleaned = append(cleaned, trimmed)
		}
	}
	lp.Motivations = cleaned

	return nil
}

// ChangedLearningProfileFields lists the names of the fields a replacement
// changes, in a stable order. It is what user.learning_profile_updated carries:
// the names of what changed, never the values, the rule ProfileUpdated follows.
// With no stored profile, every field is new.
func ChangedLearningProfileFields(before *LearningProfile, after LearningProfile) []string {
	if before == nil {
		return []string{"declared_level", "motivations", "target_exam", "target_level", "weekly_minutes_goal"}
	}
	var fields []string
	if !sameValue(before.DeclaredLevel, after.DeclaredLevel) {
		fields = append(fields, "declared_level")
	}
	if !slices.Equal(before.Motivations, after.Motivations) {
		fields = append(fields, "motivations")
	}
	if before.TargetExam != after.TargetExam {
		fields = append(fields, "target_exam")
	}
	if !sameValue(before.TargetLevel, after.TargetLevel) {
		fields = append(fields, "target_level")
	}
	if !sameValue(before.WeeklyMinutesGoal, after.WeeklyMinutesGoal) {
		fields = append(fields, "weekly_minutes_goal")
	}
	return fields
}

func sameValue[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
