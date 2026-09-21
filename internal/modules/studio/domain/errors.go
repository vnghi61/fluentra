// Package domain defines domain models and business errors for the creator studio module.
package domain

import (
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

var (
	// ErrDraftNotFound indicates that the requested course draft does not exist.
	ErrDraftNotFound = apperr.New(
		apperr.NotFound,
		"DRAFT_NOT_FOUND",
		"Course draft not found",
	)

	// ErrSubmissionNotFound means no submission exists with that id.
	// ErrSubmissionNotFound means no submission exists with that id.
	// ErrSubmissionNotFound means no submission exists with that id.
	// ErrSubmissionNotFound means no submission exists with that id.
	ErrSubmissionNotFound = apperr.New(
		apperr.NotFound,
		"SUBMISSION_NOT_FOUND",
		"Course submission not found",
	)

	// ErrProfileNotFound means the caller has not opened the studio yet.
	ErrProfileNotFound = apperr.New(
		apperr.NotFound,
		"CREATOR_PROFILE_NOT_FOUND",
		"Creator profile not found",
	)

	// ErrSelfReviewForbidden enforces BR-STUDIO-06 / BR-CONTENT-03:
	// A reviewer may not review or decide their own submission.
	ErrSelfReviewForbidden = apperr.New(
		apperr.Forbidden,
		"SELF_REVIEW_FORBIDDEN",
		"Reviewers cannot approve or reject their own course submissions",
	)

	// ErrCreatorSuspended means this creator may not submit or sell.
	ErrCreatorSuspended = apperr.New(
		apperr.Forbidden,
		"CREATOR_SUSPENDED",
		"This creator account is suspended and cannot submit or sell courses",
	)

	// ErrTakedownNotFound means the course is not currently taken down.
	ErrTakedownNotFound = apperr.New(
		apperr.NotFound,
		"TAKEDOWN_NOT_FOUND",
		"No open takedown exists for this course",
	)

	// ErrReasonRequired means a moderator acted without saying why. A takedown
	// or a suspension with no reason is one nobody can review or undo fairly.
	ErrReasonRequired = apperr.New(
		apperr.Validation,
		"REASON_REQUIRED",
		"A reason is required",
	)

	// ErrCannotSubmit means the draft is not in a state that can be submitted.
	ErrCannotSubmit = apperr.New(
		apperr.Conflict,
		"CANNOT_SUBMIT_DRAFT",
		"Only drafts in 'draft' or 'changes_requested' status can be submitted",
	)

	// ErrInvalidDraftStructure means the draft body is not a course tree.
	ErrInvalidDraftStructure = apperr.New(
		apperr.Validation,
		"INVALID_DRAFT_STRUCTURE",
		"Course draft structure fails validation",
	)

	// ErrFeedbackRequired — A decision other than approval must say what to change.
	ErrFeedbackRequired = apperr.New(
		apperr.Validation,
		"FEEDBACK_REQUIRED",
		"Reviewer feedback notes are required when requesting changes or rejecting",
	)

	// ErrInvalidStatusTransition means the draft or submission cannot move
	// to the requested state from the one it is in.
	ErrInvalidStatusTransition = apperr.New(
		apperr.Conflict,
		"INVALID_STATUS_TRANSITION",
		"Invalid submission or draft status transition",
	)

	// ErrListingNotFound — The course has no active listing.
	ErrListingNotFound = apperr.New(
		apperr.NotFound,
		"LISTING_NOT_FOUND",
		"Course listing not found",
	)

	// ErrPriceOutOfBounds — BR-STUDIO-01: the price is outside the configured bounds.
	ErrPriceOutOfBounds = apperr.New(
		apperr.Validation,
		"PRICE_OUT_OF_BOUNDS",
		"Price must be a whole number of VND between configured minimum and maximum",
	)

	// ErrCourseAlreadyPurchased — The learner already owns this course.
	ErrCourseAlreadyPurchased = apperr.New(
		apperr.Conflict,
		"ALREADY_PURCHASED",
		"Course is already owned by this learner",
	)

	// ErrCourseNotFree — The course is paid, so it cannot be claimed.
	ErrCourseNotFree = apperr.New(
		apperr.Validation,
		"COURSE_NOT_FREE",
		"Course is not free, use purchase endpoint",
	)

	// ErrCourseNotPaid — The course is free, so there is nothing to purchase.
	ErrCourseNotPaid = apperr.New(
		apperr.Validation,
		"COURSE_NOT_PAID",
		"Course is free, use claim endpoint",
	)

	// ErrPurchaseNotFound — No purchase with that id belongs to the caller.
	ErrPurchaseNotFound = apperr.New(
		apperr.NotFound,
		"PURCHASE_NOT_FOUND",
		"Course purchase not found",
	)

	// ErrRefundWindowExpired — Outside the seven-day self-service refund window.
	ErrRefundWindowExpired = apperr.New(
		apperr.Conflict,
		"REFUND_WINDOW_EXPIRED",
		"Refund window has expired (7 days from purchase)",
	)

	// ErrRefundProgressExceeded — Too much of the course has been completed to refund it.
	ErrRefundProgressExceeded = apperr.New(
		apperr.Conflict,
		"REFUND_PROGRESS_EXCEEDED",
		"Self-service refund is not available when more than 20% of the course is completed",
	)

	// ErrAlreadyRefunded — The purchase has already been refunded.
	ErrAlreadyRefunded = apperr.New(
		apperr.Conflict,
		"ALREADY_REFUNDED",
		"Purchase has already been refunded or revoked",
	)

	// ErrPaywallRestricted means the course must be bought before it opens.
	ErrPaywallRestricted = apperr.New(
		apperr.Forbidden,
		"PAYWALL_RESTRICTED",
		"Course must be purchased before access is granted",
	)

	// ErrPayoutAccountRequired — A paid course needs somewhere to send the money.
	ErrPayoutAccountRequired = apperr.New(
		apperr.Validation,
		"PAYOUT_ACCOUNT_REQUIRED",
		"Payout account must be registered before requesting payouts",
	)

	// ErrInsufficientBalance means the creator is asking to be paid more than
	// their ledger says they are owed.
	ErrInsufficientBalance = apperr.New(
		apperr.Conflict,
		"INSUFFICIENT_BALANCE",
		"Insufficient available balance for payout request",
	)

	// ErrPayoutBelowMinimum means the request is under the payout threshold.
	// Bank transfers are made by hand, so a trickle of tiny ones is a cost.
	ErrPayoutBelowMinimum = apperr.New(
		apperr.Validation,
		"PAYOUT_BELOW_MINIMUM",
		"Requested payout amount is below the minimum threshold (500,000 VND)",
	)

	// ErrLedgerEntryNotFound means a purchase has no sale credit to reverse.
	// A refund refuses rather than guessing the split from today's listing.
	ErrLedgerEntryNotFound = apperr.New(
		apperr.Conflict,
		"LEDGER_ENTRY_NOT_FOUND",
		"No sale was recorded for this purchase, so it cannot be refunded automatically",
	)
)
