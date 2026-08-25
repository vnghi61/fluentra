// Package contract exposes the public interfaces, DTOs and event payloads
// of the srs (Spaced Repetition System) module.
package contract

import (
	"context"
	"encoding/json"
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

// ReviewCardContent is the authored material behind a review card.
//
// srs schedules a `content_version_id` and knows nothing about what it holds; a
// card carrying only a schedule cannot be rendered as a flashcard, which is what
// the review screen needs. The version is resolved through `c_content`, so the
// body arrives exactly as it was authored and this module stays ignorant of
// which skill module owns the material.
type ReviewCardContent struct {
	Kind      string          `json:"kind"`
	CEFRLevel string          `json:"cefr_level,omitempty"`
	Body      json.RawMessage `json:"body"`
}

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

	// Content is nil when the version behind the card could not be resolved —
	// archived, or not yet authored. The client renders that as an explicit
	// state; it must never be filled in with a placeholder.
	Content *ReviewCardContent `json:"content,omitempty"`
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
