// Package domain contains the domain models, state machine transitions, and business errors for the content module.
package domain

import (
	"fmt"
	"strings"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

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

	// ErrTaxonomyNotFound is returned when a taxonomy entry is not found.
	ErrTaxonomyNotFound = apperr.New(apperr.NotFound, "TAXONOMY_NOT_FOUND", "The taxonomy entry was not found.")

	// ErrInvalidSlug is returned when slug format is invalid.
	ErrInvalidSlug = apperr.New(apperr.Validation, "INVALID_SLUG", "Invalid slug format.")

	// ErrInvalidReviewDecision is returned when review decision is missing or invalid.
	ErrInvalidReviewDecision = apperr.New(apperr.Validation, "INVALID_REVIEW_DECISION", "Invalid review decision.")

	// ErrInvalidReportReason is returned when the reason for reporting is invalid.
	ErrInvalidReportReason = apperr.New(apperr.Validation, "INVALID_REPORT_REASON", "Invalid report reason.")

	// ErrReportNoteTooLong is returned when report note exceeds 500 characters.
	ErrReportNoteTooLong = apperr.New(apperr.Validation, "NOTE_TOO_LONG", "Note cannot exceed 500 characters.")

	// ErrTaxonomyNodeNotFound is returned when a requested taxonomy node does not exist.
	ErrTaxonomyNodeNotFound = apperr.New(apperr.NotFound, "TAXONOMY_NODE_NOT_FOUND", "The taxonomy node was not found.")

	// ErrTaxonomyCycle is returned when proposed prerequisites would create a cycle (BR-FOUNDATION-02).
	ErrTaxonomyCycle = apperr.New(apperr.Validation, "TAXONOMY_CYCLE",
		"The proposed prerequisites would create a cyclic dependency.")

	// ErrFoundationIncomplete is returned when publishing a foundation topic
	// with no exercises, quiz or review questions on its node (BR-FOUNDATION-05).
	ErrFoundationIncomplete = apperr.New(apperr.Conflict, "FOUNDATION_INCOMPLETE",
		"A foundation topic cannot be published without at least one exercise, "+
			"one quiz, and one review question tagged to its node.")
)

// AmbiguousTaxonomyCode reports a code claimed by more than one namespace.
//
// BR-FOUNDATION-01 makes a code permanent, so the fix is never to rename one:
// the caller names the namespace it meant.
func AmbiguousTaxonomyCode(code string, namespaces ...string) error {
	return apperr.New(
		apperr.Validation,
		"TAXONOMY_CODE_AMBIGUOUS",
		fmt.Sprintf("The code %q exists in more than one namespace (%s); name the one you mean.",
			code, strings.Join(namespaces, ", ")),
	)
}
