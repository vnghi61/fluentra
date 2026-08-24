// Package domain contains domain models, graph validation, duration invariants,
// and business errors for the lesson module.
package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

// Error codes owned by the lesson module, per AGENT.md §12, ERROR_HANDLING.md, and P7.4 brief.
var (
	// ErrCourseNotFound is returned when a course does not exist.
	ErrCourseNotFound = apperr.New(apperr.NotFound, "COURSE_NOT_FOUND", "The course was not found.")

	// ErrUnitNotFound is returned when a course unit does not exist.
	ErrUnitNotFound = apperr.New(apperr.NotFound, "UNIT_NOT_FOUND", "The unit was not found.")

	// ErrLessonNotFound is returned when a lesson does not exist.
	ErrLessonNotFound = apperr.New(apperr.NotFound, "LESSON_NOT_FOUND", "The lesson was not found.")

	// ErrActivityNotFound is returned when an activity does not exist.
	ErrActivityNotFound = apperr.New(apperr.NotFound, "ACTIVITY_NOT_FOUND", "The activity was not found.")

	// ErrLessonLocked is returned when a learner attempts to access a lesson
	// before completing its prerequisites (BR-LESSON-07).
	ErrLessonLocked = apperr.New(apperr.Forbidden, "LESSON_LOCKED", "Prerequisites not met.")

	// ErrPrerequisiteCycle is returned when prerequisite relationships contain a cycle (BR-LESSON-03).
	ErrPrerequisiteCycle = apperr.New(apperr.Validation, "PREREQUISITE_CYCLE", "The proposed graph contains a cycle.")

	// ErrActivityContentUnpublished enforces BR-LESSON-02: publishing is refused when any activity
	// points to draft or archived content.
	ErrActivityContentUnpublished = apperr.New(
		apperr.Conflict, "ACTIVITY_CONTENT_UNPUBLISHED", "Cannot publish a lesson pointing at draft or archived content.",
	)

	// ErrSlugAlreadyExists is returned when a course slug violates the unique constraint.
	ErrSlugAlreadyExists = apperr.New(apperr.Conflict, "SLUG_ALREADY_EXISTS", "A course with that slug already exists.")

	// ErrInvalidCEFRLevel is returned when a CEFR level is not one of A1..C2.
	ErrInvalidCEFRLevel = apperr.New(apperr.Validation, "INVALID_CEFR_LEVEL", "Invalid CEFR level.")

	// ErrInvalidSlug is returned when a slug does not match the required kebab-case format.
	ErrInvalidSlug = apperr.New(apperr.Validation, "INVALID_SLUG", "Invalid slug format.")

	// ErrInvalidPosition is returned when position is not positive.
	ErrInvalidPosition = apperr.New(apperr.Validation, "INVALID_POSITION", "Position must be positive.")

	// ErrInvalidActivityKind is returned when activity kind is invalid.
	ErrInvalidActivityKind = apperr.New(apperr.Validation, "INVALID_ACTIVITY_KIND", "Invalid activity kind.")

	// ErrEmptyActivities is returned when an update request provides an empty activity list.
	ErrEmptyActivities = apperr.New(apperr.Validation, "EMPTY_ACTIVITIES", "Lesson activities cannot be empty.")

	// ErrTooManyActivities is returned when an update request carries more
	// activities than MaxActivitiesPerLesson.
	ErrTooManyActivities = apperr.New(
		apperr.Validation, "TOO_MANY_ACTIVITIES", "A lesson may not carry more than 100 activities.",
	)

	// ErrInvalidWeight is returned when an activity weight is outside the
	// 0..100 range the spec declares.
	ErrInvalidWeight = apperr.New(apperr.Validation, "INVALID_WEIGHT", "Weight must be between 0 and 100.")

	// ErrInvalidStatus is returned when status is not draft, published, or archived.
	ErrInvalidStatus = apperr.New(apperr.Validation, "INVALID_STATUS", "Invalid status.")

	// ErrInvalidTitle is returned when a course, unit or lesson title is empty
	// or longer than the column allows.
	ErrInvalidTitle = apperr.New(apperr.Validation, "INVALID_TITLE", "Title must be between 1 and 255 characters.")

	// ErrInvalidMinScore is returned when a prerequisite's minimum score is
	// outside the 0..100 range the schema allows.
	ErrInvalidMinScore = apperr.New(apperr.Validation, "INVALID_MIN_SCORE", "Minimum score must be between 0 and 100.")
)

// MaxTitleLength matches ck_courses_title_length and its siblings.
const MaxTitleLength = 255

// IsValidTitle reports whether a title fits the column the migration declares.
func IsValidTitle(title string) bool {
	return len(title) > 0 && len(title) <= MaxTitleLength
}
