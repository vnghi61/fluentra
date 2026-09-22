package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
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
	// Material is present only for a lesson_material activity: the resource the
	// creator uploaded, which type it is, and the rights declaration.
	Material *MaterialDraft `json:"material,omitempty"`
}

// MaterialDraft is a lesson_material activity's authored shape. resource_id is
// the creator's own private resource; material_kind is a hint the server never
// trusts — the real kind is derived from the resource's detected MIME.
type MaterialDraft struct {
	ResourceID      *uuid.UUID `json:"resource_id,omitempty"`
	MaterialKind    string     `json:"material_kind,omitempty"`
	Title           string     `json:"title,omitempty"`
	Description     string     `json:"description,omitempty"`
	RightsConfirmed bool       `json:"rights_confirmed,omitempty"`
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
	Passed       bool `json:"passed"`
	ItemsChecked int  `json:"items_checked"`
	// BlindSolved is how many items were answered independently by a model and
	// compared against their key (check 4), and BlindSolveRejects how many
	// disagreed. Reported because a creator reading "12 items checked" deserves
	// to know how many of them were checked the expensive way.
	BlindSolved       int                   `json:"blind_solved"`
	BlindSolveRejects int                   `json:"blind_solve_rejects"`
	Failures          []VerificationFailure `json:"failures"`
}

// The checks a failure is attributed to, named because they appear in the
// report a creator reads and in the tests that assert it.
const (
	checkStructure = "structure"
	checkSafety    = "safety"
)

var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	//nolint:lll // one literal; splitting it would make it unreadable and easy to break
	phoneRegex = regexp.MustCompile(`(?:\+?84|0)(?:\d{9}|\d{10})\b|\b(?:\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
	urlRegex   = regexp.MustCompile(`https?://[^\s]+`)
	// uuidRegex matches the ids a body may carry (a material's resource_id). A
	// UUID with a long run of digits reads as a phone number to phoneRegex, which
	// failed a valid course at random; ids are not contact details.
	uuidRegex = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
)

// ValidateStructureAndSafety runs Check 1 (Structure), Check 2 (Minimum size),
// Check 3 (Kind allowed), and Check 6 (Language and safety) on the draft.
// Course shape limits (WO 15 §7.2, checks 1 and 2).
const (
	minUnits              = 1
	maxUnits              = 12
	minLessonsPerUnit     = 1
	maxLessonsPerUnit     = 20
	minActivitiesPerUnit  = 3
	maxActivitiesPerUnit  = 30
	minLessonsPerCourse   = 3
	minActivitiesPerCours = 20
)

const (
	checkKindAllowed = "kind_allowed"
	checkMinimumSize = "minimum_size"
)

// hasContactDetails reports whether text carries an email address, a phone
// number or a URL. A course body is not a place for a creator's contact
// details: it is how a marketplace becomes a directory of ways to pay somebody
// outside it.
func hasContactDetails(text string) bool {
	return emailRegex.MatchString(text) || phoneRegex.MatchString(text) || urlRegex.MatchString(text)
}

// ValidateStructureAndSafety runs Gate 1's free checks over a draft: the shape
// of the course, the kinds it uses, and whether any of its text carries contact
// details.
//
// It reports every failure it finds rather than the first, because a creator
// who fixes one problem per submission round-trips forever.
func ValidateStructureAndSafety(
	title, description string, structureRaw []byte,
) (*CourseStructure, []VerificationFailure, error) {
	var failures []VerificationFailure

	if hasContactDetails(title) {
		failures = append(failures, VerificationFailure{
			Check:   checkSafety,
			Message: "Course title contains contact info or URLs",
		})
	}
	if hasContactDetails(description) {
		failures = append(failures, VerificationFailure{
			Check:   checkSafety,
			Message: "Course description contains contact info or URLs",
		})
	}

	if len(structureRaw) == 0 {
		return nil, append(failures, VerificationFailure{
			Check:   checkStructure,
			Message: "Course structure is empty",
		}), nil
	}

	var structure CourseStructure
	if err := json.Unmarshal(structureRaw, &structure); err != nil {
		return nil, nil, fmt.Errorf("unmarshal course structure: %w", err)
	}

	if n := len(structure.Units); n < minUnits || n > maxUnits {
		failures = append(failures, VerificationFailure{
			Check:   checkStructure,
			Message: fmt.Sprintf("Course must have %d-%d units, found %d", minUnits, maxUnits, n),
		})
	}

	totalLessons, totalExercises := 0, 0
	for uIdx, unit := range structure.Units {
		unitFailures, lessons, exercises := validateUnit(uIdx, unit)
		failures = append(failures, unitFailures...)
		totalLessons += lessons
		totalExercises += exercises
	}

	if totalLessons < minLessonsPerCourse {
		failures = append(failures, VerificationFailure{
			Check: checkMinimumSize,
			Message: fmt.Sprintf("Course must have at least %d lessons in total, found %d",
				minLessonsPerCourse, totalLessons),
		})
	}
	// D20-3: the twenty are graded exercises. A material is not practice, so a
	// course of twenty videos is not a practice course.
	if totalExercises < minActivitiesPerCours {
		failures = append(failures, VerificationFailure{
			Check: checkMinimumSize,
			Message: fmt.Sprintf("Course must have at least %d exercises in total, found %d",
				minActivitiesPerCours, totalExercises),
		})
	}

	return &structure, failures, nil
}

// validateUnit checks one unit and its lessons, and reports how many lessons
// and graded exercises it holds so the course totals can be summed. Materials
// are counted for neither total (D20-3).
func validateUnit(uIdx int, unit UnitDraft) (failures []VerificationFailure, lessons, exercises int) {
	if strings.TrimSpace(unit.Title) == "" {
		failures = append(failures, VerificationFailure{
			UnitIndex: uIdx,
			Check:     checkStructure,
			Message:   fmt.Sprintf("Unit %d has empty title", uIdx+1),
		})
	}
	if hasContactDetails(unit.Title) {
		failures = append(failures, VerificationFailure{
			UnitIndex: uIdx,
			Check:     checkSafety,
			Message:   fmt.Sprintf("Unit %d title contains contact info or URLs", uIdx+1),
		})
	}

	lessons = len(unit.Lessons)
	if lessons < minLessonsPerUnit || lessons > maxLessonsPerUnit {
		failures = append(failures, VerificationFailure{
			UnitIndex: uIdx,
			Check:     checkStructure,
			Message: fmt.Sprintf("Unit %d must have %d-%d lessons, found %d",
				uIdx+1, minLessonsPerUnit, maxLessonsPerUnit, lessons),
		})
	}

	for lIdx, lesson := range unit.Lessons {
		lessonFailures, count := validateLesson(uIdx, lIdx, lesson)
		failures = append(failures, lessonFailures...)
		exercises += count
	}
	return failures, lessons, exercises
}

// validateLesson checks one lesson and its activities, returning the number of
// graded exercises it holds. A lesson is valid with at least one material, or
// with 3-30 exercises; materials do not count towards the exercise minimum
// (D20-3).
func validateLesson(uIdx, lIdx int, lesson LessonDraft) (failures []VerificationFailure, exercises int) {
	if strings.TrimSpace(lesson.Title) == "" {
		failures = append(failures, VerificationFailure{
			UnitIndex:   uIdx,
			LessonIndex: lIdx,
			Check:       checkStructure,
			Message:     fmt.Sprintf("Unit %d Lesson %d has empty title", uIdx+1, lIdx+1),
		})
	}
	if hasContactDetails(lesson.Title) {
		failures = append(failures, VerificationFailure{
			UnitIndex:   uIdx,
			LessonIndex: lIdx,
			Check:       checkSafety,
			Message:     fmt.Sprintf("Unit %d Lesson %d title contains contact info or URLs", uIdx+1, lIdx+1),
		})
	}

	materials := 0
	for _, act := range lesson.Activities {
		if act.Kind == KindLessonMaterial {
			materials++
		}
	}
	exercises = len(lesson.Activities) - materials
	total := len(lesson.Activities)

	// A lesson with a material may be nothing but that material ("Chapter 3:
	// watch a video"). Without one, the 3-30 exercise rule stands. The upper
	// bound applies either way: a lesson is still a bounded run of activities.
	switch {
	case materials > 0 && total > maxActivitiesPerUnit:
		failures = append(failures, VerificationFailure{
			UnitIndex:   uIdx,
			LessonIndex: lIdx,
			Check:       checkStructure,
			Message: fmt.Sprintf("Unit %d Lesson %d must have at most %d activities, found %d",
				uIdx+1, lIdx+1, maxActivitiesPerUnit, total),
		})
	case materials == 0 && (total < minActivitiesPerUnit || total > maxActivitiesPerUnit):
		failures = append(failures, VerificationFailure{
			UnitIndex:   uIdx,
			LessonIndex: lIdx,
			Check:       checkStructure,
			Message: fmt.Sprintf("Unit %d Lesson %d must have %d-%d exercises, or at least one material, found %d",
				uIdx+1, lIdx+1, minActivitiesPerUnit, maxActivitiesPerUnit, exercises),
		})
	}

	for aIdx, act := range lesson.Activities {
		failures = append(failures, validateActivity(uIdx, lIdx, aIdx, act)...)
	}
	return failures, exercises
}

// validateActivity checks one activity's kind and its text.
func validateActivity(uIdx, lIdx, aIdx int, act ActivityDraft) []VerificationFailure {
	var failures []VerificationFailure
	if !IsAllowedActivityKind(act.Kind) {
		failures = append(failures, VerificationFailure{
			UnitIndex:     uIdx,
			LessonIndex:   lIdx,
			ActivityIndex: aIdx,
			Kind:          act.Kind,
			Check:         checkKindAllowed,
			Message:       fmt.Sprintf("Activity kind %q is not supported or permitted", act.Kind),
		})
	}
	if hasContactDetails(uuidRegex.ReplaceAllString(string(act.Body), "")) {
		failures = append(failures, VerificationFailure{
			UnitIndex:     uIdx,
			LessonIndex:   lIdx,
			ActivityIndex: aIdx,
			Kind:          act.Kind,
			Check:         checkSafety,
			Message:       "Activity body contains prohibited contact details or promotional URLs",
		})
	}
	// A material's text is creator-authored too, so the same contact-detail
	// rule runs over its title and description (WO 20 Stage B trap 2).
	if act.Material != nil {
		if hasContactDetails(act.Material.Title) || hasContactDetails(act.Material.Description) {
			failures = append(failures, VerificationFailure{
				UnitIndex:     uIdx,
				LessonIndex:   lIdx,
				ActivityIndex: aIdx,
				Kind:          act.Kind,
				Check:         checkSafety,
				Message:       "Material title or description contains contact info or URLs",
			})
		}
	}
	return failures
}
