package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

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
)
