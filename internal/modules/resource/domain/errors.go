package domain

import (
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

var (
	// ErrResourceNotFound is returned when a resource does not exist or belongs to another user.
	// 404 is used rather than 403 to prevent resource ID enumeration (BR-RESOURCE-01, D17-7).
	ErrResourceNotFound = apperr.New(
		apperr.NotFound, "RESOURCE_NOT_FOUND", "Resource not found.",
	)

	// ErrResourceQuotaExceeded is returned when the user reaches their resource count or byte limit (BR-RESOURCE-08).
	ErrResourceQuotaExceeded = apperr.New(
		apperr.RateLimited, "RESOURCE_QUOTA_EXCEEDED", "Resource count or storage limit exceeded.",
	)

	// ErrResourceTypeNotSupported is returned when detected MIME is not in the allowlist
	// or kind mismatches declared type.
	ErrResourceTypeNotSupported = apperr.New(
		apperr.Validation, "RESOURCE_TYPE_NOT_SUPPORTED", "Resource file type is not supported.",
	)

	// ErrResourceURLNotAllowed is returned when a URL has an unsupported scheme or
	// resolves to private/loopback IP (SSRF guard).
	ErrResourceURLNotAllowed = apperr.New(
		apperr.Validation, "RESOURCE_URL_NOT_ALLOWED", "Resource URL is not permitted or resolves to a private network.",
	)

	// ErrResourceNotUploaded is returned when confirm arrives for an object key that does not exist in storage.
	ErrResourceNotUploaded = apperr.New(
		apperr.Conflict, "RESOURCE_NOT_UPLOADED", "Uploaded file was not found in storage.",
	)

	// ErrInvalidStatusTransition is returned when an invalid status transition is attempted.
	ErrInvalidStatusTransition = apperr.New(
		apperr.BadRequest, "INVALID_STATUS_TRANSITION", "Invalid resource status transition.",
	)

	// ErrResourceStateChanged is returned when a status-guarded update matched no
	// row: the resource was deleted or moved on while a job was working on it.
	// It is an internal signal and never reaches a learner.
	ErrResourceStateChanged = apperr.New(
		apperr.Conflict, "RESOURCE_STATE_CHANGED", "The resource changed while it was being processed.",
	)
)
