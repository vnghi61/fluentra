package domain

import (
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

var (
	ErrDraftNotFound = apperr.New(
		apperr.NotFound,
		"DRAFT_NOT_FOUND",
		"Course draft not found",
	)

	ErrSubmissionNotFound = apperr.New(
		apperr.NotFound,
		"SUBMISSION_NOT_FOUND",
		"Course submission not found",
	)

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

	ErrCannotSubmit = apperr.New(
		apperr.Conflict,
		"CANNOT_SUBMIT_DRAFT",
		"Only drafts in 'draft' or 'changes_requested' status can be submitted",
	)

	ErrInvalidDraftStructure = apperr.New(
		apperr.Validation,
		"INVALID_DRAFT_STRUCTURE",
		"Course draft structure fails validation",
	)

	ErrFeedbackRequired = apperr.New(
		apperr.Validation,
		"FEEDBACK_REQUIRED",
		"Reviewer feedback notes are required when requesting changes or rejecting",
	)

	ErrInvalidStatusTransition = apperr.New(
		apperr.Conflict,
		"INVALID_STATUS_TRANSITION",
		"Invalid submission or draft status transition",
	)

	ErrListingNotFound = apperr.New(
		apperr.NotFound,
		"LISTING_NOT_FOUND",
		"Course listing not found",
	)

	ErrPriceOutOfBounds = apperr.New(
		apperr.Validation,
		"PRICE_OUT_OF_BOUNDS",
		"Price must be a whole number of VND between configured minimum and maximum",
	)

	ErrCourseAlreadyPurchased = apperr.New(
		apperr.Conflict,
		"ALREADY_PURCHASED",
		"Course is already owned by this learner",
	)

	ErrCourseNotFree = apperr.New(
		apperr.Validation,
		"COURSE_NOT_FREE",
		"Course is not free, use purchase endpoint",
	)

	ErrCourseNotPaid = apperr.New(
		apperr.Validation,
		"COURSE_NOT_PAID",
		"Course is free, use claim endpoint",
	)

	ErrPurchaseNotFound = apperr.New(
		apperr.NotFound,
		"PURCHASE_NOT_FOUND",
		"Course purchase not found",
	)

	ErrRefundWindowExpired = apperr.New(
		apperr.Conflict,
		"REFUND_WINDOW_EXPIRED",
		"Refund window has expired (7 days from purchase)",
	)

	ErrRefundProgressExceeded = apperr.New(
		apperr.Conflict,
		"REFUND_PROGRESS_EXCEEDED",
		"Self-service refund is not available when more than 20% of the course is completed",
	)

	ErrAlreadyRefunded = apperr.New(
		apperr.Conflict,
		"ALREADY_REFUNDED",
		"Purchase has already been refunded or revoked",
	)

	ErrPaywallRestricted = apperr.New(
		apperr.Forbidden,
		"PAYWALL_RESTRICTED",
		"Course must be purchased before access is granted",
	)
)

