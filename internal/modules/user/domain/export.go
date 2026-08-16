package domain

import (
	"time"

	"github.com/google/uuid"
)

// ExportStatus represents the lifecycle state of a user data export request.
type ExportStatus string

// Export status values.
const (
	ExportStatusPending    ExportStatus = "pending"
	ExportStatusProcessing ExportStatus = "processing"
	ExportStatusCompleted  ExportStatus = "completed"
	ExportStatusFailed     ExportStatus = "failed"
)

// ExportRequest is the domain representation of an export operation and its artefact metadata.
type ExportRequest struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Status       ExportStatus
	ObjectKey    *string
	RequestedAt  time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ExpiresAt    *time.Time
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
