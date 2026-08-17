// Package domain holds the admin module's error vocabulary, so the transport
// layer maps failures to HTTP status codes without knowing why they happened.
package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

var (
	// ErrSelfAdminActionForbidden is returned when an administrator attempts to
	// perform an administrative action on themselves.
	ErrSelfAdminActionForbidden = apperr.New(
		apperr.Forbidden, "SELF_ADMIN_ACTION_FORBIDDEN", "An admin may not administer their own account.")

	// ErrReasonRequired is returned when a required reason field is missing or too short.
	ErrReasonRequired = apperr.New(
		apperr.Validation, "REASON_REQUIRED", "Reason is required and must be at least 10 characters.")

	// ErrUserNotFound is returned when the target user account does not exist.
	ErrUserNotFound = apperr.New(
		apperr.NotFound, "USER_NOT_FOUND", "The target user account was not found.")

	// ErrFlagNotFound is returned when a requested feature flag is not found.
	ErrFlagNotFound = apperr.New(
		apperr.NotFound, "FEATURE_FLAG_NOT_FOUND", "The feature flag was not found.")

	// ErrFlagAlreadyExists is returned when trying to create a flag key that exists.
	ErrFlagAlreadyExists = apperr.New(
		apperr.Conflict, "FEATURE_FLAG_ALREADY_EXISTS", "A feature flag with that key already exists.")
)
