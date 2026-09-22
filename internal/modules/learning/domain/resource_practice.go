package domain

import (
	"time"

	"github.com/google/uuid"
)

// The states of a resource practice set.
const (
	ResourcePracticeGenerating = "generating"
	ResourcePracticeReady      = "ready"
	ResourcePracticeFailed     = "failed"
)

// ResourcePracticeSet is the private practice generated from one resource.
//
// One set per resource (WO 21 D21-2), regenerable once a day: the row is
// upserted back to generating and its activity list replaced. It is private to
// its owner (BR-RESOURCE-12); every read filters on the user.
type ResourcePracticeSet struct {
	ResourceID    uuid.UUID
	UserID        uuid.UUID
	LessonID      *uuid.UUID
	ActivityIDs   []uuid.UUID
	Status        string
	FailureReason string
	GeneratedOn   time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ResourcePracticeSetDTO is the learner-facing view of a set.
//
// The activities are assembled exactly as the daily set's are — resolved
// through lesson's contract and redacted — so the existing runner renders them
// without a second code path.
type ResourcePracticeSetDTO struct {
	ResourceID    uuid.UUID             `json:"resource_id"`
	Status        string                `json:"status"`
	FailureReason string                `json:"failure_reason,omitempty"`
	GeneratedOn   string                `json:"generated_on"`
	Activities    []DailySetActivityDTO `json:"activities"`
}
