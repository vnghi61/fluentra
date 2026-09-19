package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CourseStructure represents the hierarchical draft outline: units -> lessons -> activities.
type CourseStructure struct {
	Units []UnitDraft `json:"units"`
}

// UnitDraft represents a draft unit.
type UnitDraft struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Lessons     []LessonDraft `json:"lessons"`
}

// LessonDraft represents a draft lesson.
type LessonDraft struct {
	Title            string          `json:"title"`
	SkillFocus       string          `json:"skill_focus"`
	EstimatedMinutes int             `json:"estimated_minutes"`
	CEFRLevel        string          `json:"cefr_level"`
	Activities       []ActivityDraft `json:"activities"`
}

// ActivityDraft represents an activity in the draft.
type ActivityDraft struct {
	Kind     string          `json:"kind"`
	TaskType string          `json:"task_type,omitempty"`
	Weight   int             `json:"weight"`
	Config   json.RawMessage `json:"config,omitempty"`
	Body     json.RawMessage `json:"body"`
}

// VerificationFailure records one failure identified during Gate 1.
type VerificationFailure struct {
	UnitIndex     int    `json:"unit_index"`
	LessonIndex   int    `json:"lesson_index"`
	ActivityIndex int    `json:"activity_index"`
	Kind          string `json:"kind,omitempty"`
	Check         string `json:"check"`
	Message       string `json:"message"`
}

// VerificationReport captures the full automated verification outcome.
type VerificationReport struct {
	Passed       bool                  `json:"passed"`
	ItemsChecked int                   `json:"items_checked"`
	Failures     []VerificationFailure `json:"failures"`
}

var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	phoneRegex = regexp.MustCompile(`(?:\+?84|0)(?:\d{9}|\d{10})\b|\b(?:\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
	urlRegex   = regexp.MustCompile(`https?://[^\s]+`)
)

// ValidateStructureAndSafety runs Check 1 (Structure), Check 2 (Minimum size),
// Check 3 (Kind allowed), and Check 6 (Language and safety) on the draft.
func ValidateStructureAndSafety(title, description string, structureRaw []byte) (*CourseStructure, []VerificationFailure, error) {
	var failures []VerificationFailure

	// Safety check on title & description
	if emailRegex.MatchString(title) || phoneRegex.MatchString(title) || urlRegex.MatchString(title) {
		failures = append(failures, VerificationFailure{
			Check:   "safety",
			Message: "Course title contains contact info or URLs",
		})
	}
	if emailRegex.MatchString(description) || phoneRegex.MatchString(description) || urlRegex.MatchString(description) {
		failures = append(failures, VerificationFailure{
			Check:   "safety",
			Message: "Course description contains contact info or URLs",
		})
	}

	if len(structureRaw) == 0 {
		failures = append(failures, VerificationFailure{
			Check:   "structure",
			Message: "Course structure is empty",
		})
		return nil, failures, nil
	}

	var structure CourseStructure
	if err := json.Unmarshal(structureRaw, &structure); err != nil {
		return nil, nil, fmt.Errorf("unmarshal course structure: %w", err)
	}

	totalUnits := len(structure.Units)
	if totalUnits < 1 || totalUnits > 12 {
		failures = append(failures, VerificationFailure{
			Check:   "structure",
			Message: fmt.Sprintf("Course must have 1-12 units, found %d", totalUnits),
		})
	}

	totalLessons := 0
	totalActivities := 0

	for uIdx, unit := range structure.Units {
		if strings.TrimSpace(unit.Title) == "" {
			failures = append(failures, VerificationFailure{
				UnitIndex: uIdx,
				Check:     "structure",
				Message:   fmt.Sprintf("Unit %d has empty title", uIdx+1),
			})
		}
		if emailRegex.MatchString(unit.Title) || phoneRegex.MatchString(unit.Title) || urlRegex.MatchString(unit.Title) {
			failures = append(failures, VerificationFailure{
				UnitIndex: uIdx,
				Check:     "safety",
				Message:   fmt.Sprintf("Unit %d title contains contact info or URLs", uIdx+1),
			})
		}

		unitLessonCount := len(unit.Lessons)
		if unitLessonCount < 1 || unitLessonCount > 20 {
			failures = append(failures, VerificationFailure{
				UnitIndex: uIdx,
				Check:     "structure",
				Message:   fmt.Sprintf("Unit %d must have 1-20 lessons, found %d", uIdx+1, unitLessonCount),
			})
		}
		totalLessons += unitLessonCount

		for lIdx, lesson := range unit.Lessons {
			if strings.TrimSpace(lesson.Title) == "" {
				failures = append(failures, VerificationFailure{
					UnitIndex:   uIdx,
					LessonIndex: lIdx,
					Check:       "structure",
					Message:     fmt.Sprintf("Unit %d Lesson %d has empty title", uIdx+1, lIdx+1),
				})
			}
			if emailRegex.MatchString(lesson.Title) || phoneRegex.MatchString(lesson.Title) || urlRegex.MatchString(lesson.Title) {
				failures = append(failures, VerificationFailure{
					UnitIndex:   uIdx,
					LessonIndex: lIdx,
					Check:       "safety",
					Message:     fmt.Sprintf("Unit %d Lesson %d title contains contact info or URLs", uIdx+1, lIdx+1),
				})
			}

			actCount := len(lesson.Activities)
			if actCount < 3 || actCount > 30 {
				failures = append(failures, VerificationFailure{
					UnitIndex:   uIdx,
					LessonIndex: lIdx,
					Check:       "structure",
					Message:     fmt.Sprintf("Unit %d Lesson %d must have 3-30 activities, found %d", uIdx+1, lIdx+1, actCount),
				})
			}
			totalActivities += actCount

			for aIdx, act := range lesson.Activities {
				if !IsAllowedActivityKind(act.Kind) {
					failures = append(failures, VerificationFailure{
						UnitIndex:     uIdx,
						LessonIndex:   lIdx,
						ActivityIndex: aIdx,
						Kind:          act.Kind,
						Check:         "kind_allowed",
						Message:       fmt.Sprintf("Activity kind %q is not supported or permitted", act.Kind),
					})
				}

				bodyStr := string(act.Body)
				if emailRegex.MatchString(bodyStr) || phoneRegex.MatchString(bodyStr) || urlRegex.MatchString(bodyStr) {
					failures = append(failures, VerificationFailure{
						UnitIndex:     uIdx,
						LessonIndex:   lIdx,
						ActivityIndex: aIdx,
						Kind:          act.Kind,
						Check:         "safety",
						Message:       "Activity body contains prohibited contact details or promotional URLs",
					})
				}
			}
		}
	}

	// Check 2: Minimum size: at least 3 lessons and at least 20 activities in total
	if totalLessons < 3 {
		failures = append(failures, VerificationFailure{
			Check:   "minimum_size",
			Message: fmt.Sprintf("Course must have at least 3 lessons in total, found %d", totalLessons),
		})
	}
	if totalActivities < 20 {
		failures = append(failures, VerificationFailure{
			Check:   "minimum_size",
			Message: fmt.Sprintf("Course must have at least 20 activities in total, found %d", totalActivities),
		})
	}

	return &structure, failures, nil
}
