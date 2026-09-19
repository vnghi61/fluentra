package contract

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Aggregate is the outbox aggregate name every studio event is written under.
const Aggregate = "studio"

// Studio event topics.
const (
	EventCoursePublished   = "studio.course_published"
	EventSubmissionCreated = "studio.submission_created"
	EventSubmissionDecided = "studio.submission_decided"
)

// CreatorProfile represents a creator's public profile in the studio.
type CreatorProfile struct {
	UserID         uuid.UUID `json:"user_id"`
	Bio            string    `json:"bio"`
	Headline       string    `json:"headline"`
	PayoutEligible bool      `json:"payout_eligible"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PayoutAccount represents bank account information for creator disbursements.
// BR-STUDIO-09: A creator's payout account is never returned in a list response and never logged.
type PayoutAccount struct {
	ID                uuid.UUID `json:"id"`
	CreatorID         uuid.UUID `json:"creator_id"`
	BankCode          string    `json:"bank_code"`
	AccountNumber     string    `json:"account_number"`
	AccountHolderName string    `json:"account_holder_name"`
	IsDefault         bool      `json:"is_default"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CourseDraft represents a creator course draft being authored.
type CourseDraft struct {
	ID              uuid.UUID       `json:"id"`
	OwnerID         uuid.UUID       `json:"owner_id"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	Description     string          `json:"description"`
	CEFRLevel       string          `json:"cefr_level"`
	TopicTaxonomyID *uuid.UUID      `json:"topic_taxonomy_id,omitempty"`
	PriceVND        int64           `json:"price_vnd"`
	Status          string          `json:"status"`
	Structure       json.RawMessage `json:"structure"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Submission represents a review submission attempt for a course draft.
type Submission struct {
	ID                 uuid.UUID       `json:"id"`
	DraftID            uuid.UUID       `json:"draft_id"`
	Version            int             `json:"version"`
	Status             string          `json:"status"`
	SubmittedBy        uuid.UUID       `json:"submitted_by"`
	ReviewerID         *uuid.UUID      `json:"reviewer_id,omitempty"`
	Feedback           *string         `json:"feedback,omitempty"`
	VerificationReport json.RawMessage `json:"verification_report,omitempty"`
	SubmittedAt        time.Time       `json:"submitted_at"`
	ReviewedAt         *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ModerationQueueItem combines a submission with its parent course draft for reviewers.
type ModerationQueueItem struct {
	Submission Submission  `json:"submission"`
	Draft      CourseDraft `json:"draft"`
}

// AccessReader evaluates course opening permission.
// BR-STUDIO-05: The paywall is one function: MayOpen.
type AccessReader interface {
	MayOpen(ctx context.Context, userID *uuid.UUID, courseID uuid.UUID) (bool, error)
}
