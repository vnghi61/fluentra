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
	Batch     *string
	Limit     int
	Offset    int
}

// ReviewBatch groups the drafts of one generation run awaiting review, so a
// person can approve the doubts of a run together (WO 22 Stage A.4).
type ReviewBatch struct {
	Batch     string
	ItemCount int64
	Kinds     []string
	CreatedAt time.Time
}

// ReviewSample is one auto-published version drawn for a person to spot-check
// (WO 22 Stage A.5).
type ReviewSample struct {
	VersionID uuid.UUID
	Batch     string
	Kind      string
	CEFRLevel string
	SampledOn time.Time
	CreatedAt time.Time
}

// SampleDecision is what a person decided about a sampled item.
type SampleDecision string

// The two sample decisions.
const (
	// SampleKept means the item stands.
	SampleKept SampleDecision = "kept"
	// SampleRejected means the item is unpublished and stops being drawn.
	SampleRejected SampleDecision = "rejected"
)

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
