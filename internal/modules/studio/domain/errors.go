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
)
