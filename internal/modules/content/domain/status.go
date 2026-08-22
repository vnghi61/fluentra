package domain

import (
	"fmt"
)

// AuthoringStatus represents the state of a content item or version in the authoring lifecycle.
type AuthoringStatus string

// AuthoringStatus constants.
const (
	// StatusDraft indicates initial editable draft.
	StatusDraft AuthoringStatus = "draft"
	// StatusInReview indicates content awaiting review.
	StatusInReview AuthoringStatus = "in_review"
	// StatusApproved indicates content approved by reviewer.
	StatusApproved AuthoringStatus = "approved"
	// StatusPublished indicates publicly visible content.
	StatusPublished AuthoringStatus = "published"
	// StatusArchived indicates deactivated content.
	StatusArchived AuthoringStatus = "archived"
)

// AllAuthoringStatuses lists all known lifecycle statuses.
var AllAuthoringStatuses = []AuthoringStatus{
	StatusDraft,
	StatusInReview,
	StatusApproved,
	StatusPublished,
	StatusArchived,
}

// ParseAuthoringStatus parses a raw status string into AuthoringStatus.
func ParseAuthoringStatus(s string) (AuthoringStatus, error) {
	status := AuthoringStatus(s)
	switch status {
	case StatusDraft, StatusInReview, StatusApproved, StatusPublished, StatusArchived:
		return status, nil
	default:
		return "", fmt.Errorf("invalid authoring status: %q", s)
	}
}

// String returns the string representation.
func (s AuthoringStatus) String() string {
	return string(s)
}

// legalTransitions defines the explicit state machine transition matrix.
// Key is the current status; value is the set of permissible target statuses.
//
// This single table covers both content_items.status and content_versions.status,
// but the two entities do not share the same legal moves. The DB enforces the
// difference: trg_content_versions_immutable refuses every UPDATE on a published
// version row, so version rows can never transition at all once published.
//
//   - draft → draft, draft → in_review, in_review → approved/draft,
//     approved → published/draft are valid for both items and versions.
//   - published → published is idempotent publish and is valid for both, but
//     version-level immutability means it is a no-op in the DB.
//   - published → archived is valid for items only; Version.ValidateTransition
//     would return true here but the trigger blocks it at the version row level.
//     Archive is an item-level action (sets content_items.status).
//
// Callers that transition a version should treat published → archived as illegal
// even though the map says otherwise; callers that transition an item may use it.
var legalTransitions = map[AuthoringStatus]map[AuthoringStatus]bool{
	StatusDraft: {
		StatusDraft:    true, // Editing draft
		StatusInReview: true, // Submit for review
	},
	StatusInReview: {
		StatusApproved: true, // Review approved
		StatusDraft:    true, // Changes requested
	},
	StatusApproved: {
		StatusPublished: true, // Publish approved version
		StatusDraft:     true, // Retract/edit approved draft
	},
	StatusPublished: {
		StatusPublished: true, // Idempotent publish (item and version)
		StatusArchived:  true, // Archive item only — version rows are immutable
	},
	StatusArchived: {}, // Terminal state: no moves permitted
}

// ValidateTransition validates whether moving from 'from' to 'to' is allowed.
// Returns ErrInvalidStateTransition if the move is illegal.
func ValidateTransition(from, to AuthoringStatus) error {
	targets, ok := legalTransitions[from]
	if !ok || !targets[to] {
		return ErrInvalidStateTransition.WithInternal(
			fmt.Sprintf("cannot transition content status from %q to %q", from, to),
		)
	}
	return nil
}

// CanTransition returns true if the transition from -> to is legal.
func CanTransition(from, to AuthoringStatus) bool {
	return ValidateTransition(from, to) == nil
}

// ReviewDecision represents an admin's decision during a content review.
type ReviewDecision string

// ReviewDecision constants.
const (
	// ReviewDecisionApproved approves the content version.
	ReviewDecisionApproved ReviewDecision = "approved"
	// ReviewDecisionChangesRequested rejects the content version and requests revisions.
	ReviewDecisionChangesRequested ReviewDecision = "changes_requested"
)

// ParseReviewDecision parses a raw decision string into ReviewDecision.
func ParseReviewDecision(s string) (ReviewDecision, error) {
	d := ReviewDecision(s)
	switch d {
	case ReviewDecisionApproved, ReviewDecisionChangesRequested:
		return d, nil
	default:
		return "", fmt.Errorf("invalid review decision: %q", s)
	}
}
