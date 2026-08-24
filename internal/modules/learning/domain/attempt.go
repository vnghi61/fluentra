// Package domain contains business entities, state invariants, grader registries,
// and domain errors for the learning module.
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Attempt statuses. These four and no others: ck_attempts_status in migration
// 1700000210 accepts exactly this set, and AttemptDetail.status in
// components/learning.yaml declares exactly this set. A fifth constant here is
// a state the database rejects and no response can carry.
const (
	StatusInProgress = "in_progress"
	StatusGrading    = "grading"
	StatusGraded     = "graded"
	StatusFailed     = "failed"
)

// Progress statuses. A different set from the attempt statuses above, and kept
// apart from them on purpose: ck_progress_status accepts these three, and
// "completed" is a progress state that no attempt is ever in.
const (
	ProgressNotStarted = "not_started"
	ProgressInProgress = "in_progress"
	ProgressCompleted  = "completed"
)

// Attempt models a learner's attempt at an interactive activity.
type Attempt struct {
	ID             uuid.UUID       `json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	UserID         uuid.UUID       `json:"user_id"`
	ActivityID     uuid.UUID       `json:"activity_id"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	Response       json.RawMessage `json:"response,omitempty"`
	Score          *int            `json:"score,omitempty"`
	MaxScore       int             `json:"max_score"`
	Grader         *string         `json:"grader,omitempty"`
	DurationMs     *int64          `json:"duration_ms,omitempty"`
	Status         string          `json:"status"`
}

// CanSubmit returns true if the attempt is in a valid state to be submitted.
func (a *Attempt) CanSubmit() bool {
	return a.Status == StatusInProgress
}

// IsGraded returns true if the attempt has completed grading.
func (a *Attempt) IsGraded() bool {
	return a.Status == StatusGraded
}

// IsGrading returns true if the attempt is currently undergoing asynchronous grading.
func (a *Attempt) IsGrading() bool {
	return a.Status == StatusGrading
}

// ValidStatus checks if the given attempt status string is known and valid.
func ValidStatus(status string) bool {
	switch status {
	case StatusInProgress, StatusGrading, StatusGraded, StatusFailed:
		return true
	default:
		return false
	}
}
