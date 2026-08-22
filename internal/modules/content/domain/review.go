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
