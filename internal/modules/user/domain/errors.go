package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

// The error codes this module owns, as declared in its AGENT.md §12. They are
// package-level sentinels so a caller can match on them; apperr.Error copies
// itself on every With* call, so decorating one here cannot leak into another
// request.
var (
	// ErrUserNotFound is returned for an account that does not exist. It is a
	// 404 even when the caller is the owner: there is no state in which a valid
	// token points at a missing user that the client can do anything about.
	ErrUserNotFound = apperr.New(apperr.NotFound, "USER_NOT_FOUND", "The account was not found.")

	// ErrProfileNotFound separates "no user" from "user with no profile row",
	// which is a registration bug rather than a client error.
	ErrProfileNotFound = apperr.New(apperr.NotFound, "PROFILE_NOT_FOUND", "The profile was not found.")

	// ErrPreferencesNotFound is the same distinction for the preference row.
	ErrPreferencesNotFound = apperr.New(
		apperr.NotFound, "PREFERENCES_NOT_FOUND", "The preferences were not found.")

	// ErrDisplayNameNotAllowed enforces BR-USER-02.
	ErrDisplayNameNotAllowed = apperr.New(
		apperr.Validation, "DISPLAY_NAME_NOT_ALLOWED", "That display name is not allowed.")

	// ErrUnknownStatus means the database and this package disagree about the
	// core.user_status enum.
	ErrUnknownStatus = apperr.New(apperr.Internal, "UNKNOWN_USER_STATUS", "The account state is not recognised.")

	// ErrAccountNotUsable is returned when a suspended or deleting account
	// tries to change its own data.
	ErrAccountNotUsable = apperr.New(apperr.Forbidden, "ACCOUNT_NOT_USABLE", "This account cannot be modified.")

	// ErrEmailAlreadyRegistered is the citext unique constraint, translated.
	// It is a 409 rather than a 422 because nothing about the request is
	// malformed — the address is simply taken.
	ErrEmailAlreadyRegistered = apperr.New(
		apperr.Conflict, "EMAIL_ALREADY_REGISTERED", "That email address is already registered.")

	// ErrExportAlreadyPending is returned when an export is already pending or processing.
	ErrExportAlreadyPending = apperr.New(
		apperr.Conflict, "EXPORT_ALREADY_PENDING", "A data export is already in progress.")

	// ErrDeletionAlreadyPending is returned when a deletion request is already pending or processing.
	ErrDeletionAlreadyPending = apperr.New(
		apperr.Conflict, "DELETION_ALREADY_PENDING", "An account deletion is already in progress.")

	// ErrDeletionNotFound is returned when no deletion request exists.
	ErrDeletionNotFound = apperr.New(
		apperr.NotFound, "DELETION_NOT_FOUND", "The deletion request was not found.")

	// ErrDeletionNotCancellable is returned when trying to cancel a non-pending deletion.
	ErrDeletionNotCancellable = apperr.New(
		apperr.Conflict, "DELETION_NOT_CANCELLABLE", "This deletion request cannot be cancelled.")
)

// invalid builds a 422 carrying exactly one field violation. Every validation
// failure in this package goes through it, so the transport layer never has to
// decide a status code and the client always gets the field name.
func invalid(field, code, message string) *apperr.Error {
	return apperr.New(apperr.Validation, "VALIDATION_FAILED", "One or more request fields are invalid.").
		WithFields(fieldViolation(field, code, message))
}

// fieldViolation names the failing field on an error that already has its own
// code — DISPLAY_NAME_NOT_ALLOWED, say, where the code carries more than
// VALIDATION_FAILED would but the client still needs to know which field.
func fieldViolation(field, code, message string) apperr.FieldViolation {
	return apperr.FieldViolation{Field: field, Code: code, Message: message}
}
