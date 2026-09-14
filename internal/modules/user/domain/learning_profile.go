package domain

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TargetExam represents standard exam goals.
type TargetExam string

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

// ChangedFields identifies field names for audit events.
func (lp *LearningProfile) ChangedFields() []string {
	fields := []string{"target_exam", "motivations"}
	if lp.DeclaredLevel != nil {
		fields = append(fields, "declared_level")
	}
	if lp.TargetLevel != nil {
		fields = append(fields, "target_level")
	}
	if lp.WeeklyMinutesGoal != nil {
		fields = append(fields, "weekly_minutes_goal")
	}
	slices.Sort(fields)
	return fields
}
