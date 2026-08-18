package domain

import (
	"time"

	"github.com/google/uuid"
)

// DeletionGracePeriod is the 30-day window during which an account deletion can be cancelled.
const DeletionGracePeriod = 30 * 24 * time.Hour

// DeletionStatus represents the lifecycle of an account deletion request.
type DeletionStatus string

const (
	// DeletionStatusPending means the deletion was requested and is waiting out the 30-day grace period.
	DeletionStatusPending DeletionStatus = "pending"
	// DeletionStatusProcessing means the deletion job has picked up the request
	// and is executing data erasure, or has finished erasing and is waiting for
	// the completeness check to confirm every module purged its own data.
	DeletionStatusProcessing DeletionStatus = "processing"
	// DeletionStatusCompleted means the user has been fully anonymised and personal data purged across modules.
	DeletionStatusCompleted DeletionStatus = "completed"
	// DeletionStatusFailed means an unexpected error occurred during execution.
	DeletionStatusFailed DeletionStatus = "failed"
	// DeletionStatusCancelled means the learner cancelled the request before the 30-day deadline expired.
	DeletionStatusCancelled DeletionStatus = "cancelled"
)

// DeletionRequest represents an account deletion request in core.user_deletions.
type DeletionRequest struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Status       DeletionStatus
	RequestedAt  time.Time
	ExecuteAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CancelledAt  *time.Time
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsCancellable reports whether the deletion request can still be cancelled.
func (d DeletionRequest) IsCancellable() bool {
	return d.Status == DeletionStatusPending
}
