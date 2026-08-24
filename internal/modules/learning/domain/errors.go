package domain

import (
	"errors"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Domain errors for the learning module.
var (
	// ErrAttemptNotFound is returned when an attempt row is not found.
	ErrAttemptNotFound = apperr.New(
		apperr.NotFound, "ATTEMPT_NOT_FOUND", "The attempt was not found.",
	)

	// ErrAlreadyGraded is returned when an attempt is already graded under a different idempotency key.
	ErrAlreadyGraded = apperr.New(
		apperr.Conflict, "ALREADY_GRADED", "The attempt has already been graded.",
	)

	// ErrIdempotencyConflict is returned when an idempotency key conflicts with an existing submission.
	ErrIdempotencyConflict = apperr.New(
		apperr.Conflict, "IDEMPOTENCY_CONFLICT", "A different submission with that idempotency key exists.",
	)

	// ErrGraderNotRegistered is returned when no grader is registered for an activity kind.
	// A kind with no grader is a request for something this deployment does not
	// support, not a fault in the deployment: the kinds it does support are
	// validated at startup, so reaching here means the activity asked for one
	// that was never declared. 422 with the kind named, not a 500.
	ErrGraderNotRegistered = apperr.New(
		apperr.Validation, "UNSUPPORTED_ACTIVITY_KIND", "No grader is registered for this activity kind.",
	)

	// ErrInvalidStatus is returned when an attempt or progress status is invalid.
	ErrInvalidStatus = apperr.New(
		apperr.Validation, "INVALID_STATUS", "Invalid status.",
	)

	// ErrInvalidIdempotencyKey is returned when the required Idempotency-Key header is missing or empty.
	ErrInvalidIdempotencyKey = apperr.New(
		apperr.Validation, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key header is required.",
	)

	// ErrUnauthorizedAttemptAccess is returned when an actor tries to access or submit an attempt they do not own.
	ErrUnauthorizedAttemptAccess = apperr.New(
		apperr.Forbidden, "FORBIDDEN", "You do not have access to this attempt.",
	)

	// ErrAlreadyEnrolled is returned when enrolling in a course the user is already enrolled in.
	ErrAlreadyEnrolled = apperr.New(
		apperr.Conflict, "ALREADY_ENROLLED", "You are already enrolled in this course.",
	)

	// ErrLessonLocked is returned when attempting an activity in a locked lesson.
	ErrLessonLocked = apperr.New(
		apperr.Forbidden, "LESSON_LOCKED", "This lesson is locked.",
	)

	// ErrNotEnrolled is returned when attempting an activity in a course the user has not enrolled in.
	//
	// The code is its own, not the generic FORBIDDEN that ErrUnauthorizedAttemptAccess
	// carries: a client that cannot tell "enrol first" from "this is not your attempt"
	// cannot offer the learner the one action that fixes it.
	ErrNotEnrolled = apperr.New(
		apperr.Forbidden, "NOT_ENROLLED", "You are not enrolled in this course.",
	)

	// ErrCourseNotFound is returned when enrolling in a course that does not exist.
	// enrollments.course_id has a foreign key to learn.courses, so the database is
	// the one that knows; mapPgError turns that violation into this.
	ErrCourseNotFound = apperr.New(
		apperr.NotFound, "COURSE_NOT_FOUND", "The course was not found.",
	)

	// ErrSessionNotFound is returned when a learning session cannot be found.
	ErrSessionNotFound = apperr.New(
		apperr.NotFound, "SESSION_NOT_FOUND", "The learning session was not found.",
	)

	// ErrInvalidActivityCount is returned when a session is completed with a negative
	// activity count. learning_sessions.activities_completed has a >= 0 CHECK, and a
	// constraint violation would surface as a 500 for what is a bad request.
	ErrInvalidActivityCount = apperr.New(
		apperr.Validation, "INVALID_ACTIVITY_COUNT", "activities_completed cannot be negative.",
	)

	// ErrInvalidDuration is returned when a session end time is before its start time.
	ErrInvalidDuration = apperr.New(
		apperr.Validation, "INVALID_DURATION", "Session end time cannot be before start time.",
	)
)

// IsAlreadyEnrolled reports whether err represents ErrAlreadyEnrolled.
func IsAlreadyEnrolled(err error) bool {
	var e *apperr.Error
	return errors.As(err, &e) && e.Code == "ALREADY_ENROLLED"
}

// IsLessonLocked reports whether err represents ErrLessonLocked.
func IsLessonLocked(err error) bool {
	var e *apperr.Error
	return errors.As(err, &e) && e.Code == "LESSON_LOCKED"
}

// IsNotEnrolled reports whether err represents ErrNotEnrolled.
func IsNotEnrolled(err error) bool {
	var e *apperr.Error
	return errors.As(err, &e) && e.Code == "NOT_ENROLLED"
}

// IsCourseNotFound reports whether err represents ErrCourseNotFound.
func IsCourseNotFound(err error) bool {
	var e *apperr.Error
	return errors.As(err, &e) && e.Code == "COURSE_NOT_FOUND"
}

// IsSessionNotFound reports whether err represents ErrSessionNotFound.
func IsSessionNotFound(err error) bool {
	var e *apperr.Error
	return errors.As(err, &e) && e.Code == "SESSION_NOT_FOUND"
}
