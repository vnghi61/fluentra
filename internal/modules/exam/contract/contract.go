// Package contract defines the public contract interface for the exam module.
package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Reader provides read access to exam attempts and history for other modules.
type Reader interface {
	AttemptHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]AttemptSummary, int, error)
	LatestBand(ctx context.Context, userID uuid.UUID) (*string, error)
	IsExamPoolActivity(ctx context.Context, activityID uuid.UUID) (bool, error)
}

// AttemptSummary summarizes a completed or historical sitting.
type AttemptSummary struct {
	ID          uuid.UUID  `json:"id"`
	ExamID      uuid.UUID  `json:"exam_id"`
	ExamSlug    string     `json:"exam_slug"`
	ExamTitle   string     `json:"exam_title"`
	Mode        string     `json:"mode"`
	StartedAt   time.Time  `json:"started_at"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	Status      string     `json:"status"`
	Score       *float64   `json:"score,omitempty"`
	Band        *string    `json:"band,omitempty"`
}

// AttemptStartedEvent is published when an exam sitting starts.
type AttemptStartedEvent struct {
	UserID    uuid.UUID `json:"user_id"`
	ExamID    uuid.UUID `json:"exam_id"`
	AttemptID uuid.UUID `json:"attempt_id"`
}

// AttemptFinishedEvent is published when an exam sitting finishes and reports are ready.
type AttemptFinishedEvent struct {
	UserID     uuid.UUID      `json:"user_id"`
	AttemptID  uuid.UUID      `json:"attempt_id"`
	Band       string         `json:"band"`
	PerSection map[string]any `json:"per_section"`
}
