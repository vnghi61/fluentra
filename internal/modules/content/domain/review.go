package domain

import (
	"time"

	"github.com/google/uuid"
)

// Review represents an editorial review audit record.
type Review struct {
	ID         uuid.UUID
	VersionID  uuid.UUID
	ReviewerID uuid.UUID
	Decision   ReviewDecision
	Comments   *string
	CreatedAt  time.Time
}

// ReviewQueueFilter holds query parameters for filtering the review queue.
type ReviewQueueFilter struct {
	Purpose   *string
	Kind      *string
	CEFRLevel *string
	NodeCode  *string
	Limit     int
	Offset    int
}

// ReviewQueueItem represents a draft item in the editorial review queue.
type ReviewQueueItem struct {
	ID        uuid.UUID
	ItemID    uuid.UUID
	Slug      string
	Kind      string
	CEFRLevel string
	Status    AuthoringStatus
	Body      []byte
	CreatedAt time.Time
	NodeCodes []string
}
