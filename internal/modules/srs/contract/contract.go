// Package contract exposes the public interfaces, DTOs and event payloads
// of the srs (Spaced Repetition System) module.
package contract

import (
	"context"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// Aggregate identifies srs domain events in outbox/eventbus.
	Aggregate = "srs"

	// EventReviewCardAnswered is emitted whenever a review card is graded and rescheduled.
	EventReviewCardAnswered = "review.card_answered"

	// EventReviewSessionCompleted is emitted when a review study session finishes.
	EventReviewSessionCompleted = "review.session_completed"

	// EventReviewDueSoon is emitted by background jobs for reminder notifications.
	EventReviewDueSoon = "review.due_soon"
)

// ReviewCardSummary represents a learner's review card state.
type ReviewCardSummary struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	ContentVersionID uuid.UUID  `json:"content_version_id"`
	Skill            string     `json:"skill"`
	Stability        float64    `json:"stability"`
	Difficulty       float64    `json:"difficulty"`
	DueAt            time.Time  `json:"due_at"`
	Reps             int        `json:"reps"`
	Lapses           int        `json:"lapses"`
	State            string     `json:"state"`
	SuspendedAt      *time.Time `json:"suspended_at,omitempty"`
}

// CardWriter allows upstream modules to create review cards and to take content
// out of a learner's rotation.
//
// The exercise engine calls UpsertCards after grading. Skill modules call
// SetCardsSuspended when a learner declares they already know a piece of content
// — that is the only supported way to stop scheduling it, because
// learn.review_cards belongs to srs and to no one else.
type CardWriter interface {
	UpsertCards(ctx context.Context, userID uuid.UUID, items []learningcontract.ReviewItem) error
	SetCardsSuspended(ctx context.Context, userID uuid.UUID, contentVersionIDs []uuid.UUID, suspended bool) error
}

// QueueReader provides read-only access to due review cards and pending counts.
type QueueReader interface {
	DueCount(ctx context.Context, userID uuid.UUID) (int, error)
	DueCards(ctx context.Context, userID uuid.UUID, limit int32) ([]ReviewCardSummary, error)
}

// CardAnswered payload for review.card_answered event.
type CardAnswered struct {
	UserID       uuid.UUID `json:"user_id"`
	CardID       uuid.UUID `json:"card_id"`
	Grade        string    `json:"grade"`
	IntervalDays int       `json:"interval_days"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// SessionCompleted payload for review.session_completed event.
type SessionCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	Reviewed   int       `json:"reviewed"`
	Correct    int       `json:"correct"`
	Minutes    int       `json:"minutes"`
	OccurredAt time.Time `json:"occurred_at"`
}

// DueSoon payload for review.due_soon event.
type DueSoon struct {
	UserID     uuid.UUID `json:"user_id"`
	DueCount   int       `json:"due_count"`
	OccurredAt time.Time `json:"occurred_at"`
}
