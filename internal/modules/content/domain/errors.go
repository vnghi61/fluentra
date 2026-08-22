// Package domain contains the domain models, state machine transitions, and business errors for the content module.
package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

// Error codes owned by the content module, per AGENT.md §12 and ERROR_HANDLING.md.
var (
	// ErrItemNotFound is returned when a content item does not exist.
	ErrItemNotFound = apperr.New(apperr.NotFound, "CONTENT_ITEM_NOT_FOUND", "The content item was not found.")

	// ErrVersionNotFound is returned when a content version does not exist.
	ErrVersionNotFound = apperr.New(
		apperr.NotFound, "CONTENT_VERSION_NOT_FOUND", "The content version was not found.",
	)

	// ErrContentNotPublished is returned when a draft or archived content is requested by a learner.
	ErrContentNotPublished = apperr.New(
		apperr.NotFound, "CONTENT_NOT_PUBLISHED", "The requested content is not published.",
	)

	// ErrInvalidStateTransition is returned when an illegal state machine move is attempted.
	ErrInvalidStateTransition = apperr.New(
		apperr.Conflict, "INVALID_STATE_TRANSITION", "The requested state transition is not allowed.",
	)

	// ErrSelfApprovalForbidden enforces BR-CONTENT-03: an author cannot approve their own version.
	ErrSelfApprovalForbidden = apperr.New(
		apperr.Forbidden, "SELF_APPROVAL_FORBIDDEN", "Authors cannot approve their own content.",
	)

	// ErrContentInUse is returned when archiving is blocked because published material references it.
	ErrContentInUse = apperr.New(
		apperr.Conflict, "CONTENT_IN_USE", "Cannot archive content that is in use by published material.",
	)

	// ErrMediaNotReady enforces BR-CONTENT-04: publishing is blocked until referenced media assets are ready.
	ErrMediaNotReady = apperr.New(
		apperr.Conflict, "MEDIA_NOT_READY", "Referenced media assets are not ready.",
	)

	// ErrSlugAlreadyExists is returned when an item slug violates the unique constraint.
	ErrSlugAlreadyExists = apperr.New(
		apperr.Conflict, "SLUG_ALREADY_EXISTS", "A content item with that slug already exists.",
	)

	// ErrInvalidKind is returned when kind is malformed or invalid.
	ErrInvalidKind = apperr.New(apperr.Validation, "INVALID_CONTENT_KIND", "Invalid content kind.")

	// ErrInvalidCEFRLevel is returned when CEFR level is not one of A1..C2.
	ErrInvalidCEFRLevel = apperr.New(apperr.Validation, "INVALID_CEFR_LEVEL", "Invalid CEFR level.")

	// ErrInvalidSlug is returned when slug format is invalid.
	ErrInvalidSlug = apperr.New(apperr.Validation, "INVALID_SLUG", "Invalid slug format.")

	// ErrInvalidReviewDecision is returned when review decision is missing or invalid.
	ErrInvalidReviewDecision = apperr.New(apperr.Validation, "INVALID_REVIEW_DECISION", "Invalid review decision.")
)
