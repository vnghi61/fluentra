package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Draft statuses.
const (
	DraftStatusDraft            = "draft"
	DraftStatusSubmitted        = "submitted"
	DraftStatusVerifying        = "verifying"
	DraftStatusInReview         = "in_review"
	DraftStatusApproved         = "approved"
	DraftStatusPublished        = "published"
	DraftStatusRejected         = "rejected"
	DraftStatusChangesRequested = "changes_requested"
)

// Submission statuses.
const (
	SubmissionStatusSubmitted        = "submitted"
	SubmissionStatusVerifying        = "verifying"
	SubmissionStatusInReview         = "in_review"
	SubmissionStatusApproved         = "approved"
	SubmissionStatusRejected         = "rejected"
	SubmissionStatusChangesRequested = "changes_requested"
)

// The eleven activity kinds a creator may author (BR-STUDIO-10).
// Listening comprehension is excluded in v1.
var AllowedActivityKinds = map[string]struct{}{
	"vocab_multiple_choice":      {},
	"vocab_gap_fill":             {},
	"vocab_flashcard":            {},
	"vocab_listen_type":          {},
	"vocab_match":                {},
	"vocab_reorder":              {},
	"vocab_context_choice":       {},
	"grammar_tense_choice":       {},
	"grammar_sentence_transform": {},
	"reading_comprehension":      {},
	"writing_prompt":             {},
	"speaking_task":              {},
}

// IsAllowedActivityKind returns true if kind is supported by the lesson runner.
func IsAllowedActivityKind(kind string) bool {
	_, ok := AllowedActivityKinds[kind]
	return ok
}

// CreatorProfile domain model.
type CreatorProfile struct {
	UserID         uuid.UUID
	Bio            string
	Headline       string
	PayoutEligible bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PayoutAccount domain model.
type PayoutAccount struct {
	ID                uuid.UUID
	CreatorID         uuid.UUID
	BankCode          string
	AccountNumber     string
	AccountHolderName string
	IsDefault         bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CourseDraft domain model.
type CourseDraft struct {
	ID              uuid.UUID
	OwnerID         uuid.UUID
	Title           string
	Slug            string
	Description     string
	CEFRLevel       string
	TopicTaxonomyID *uuid.UUID
	PriceVND        int64
	Status          string
	Structure       json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Submission domain model.
type Submission struct {
	ID                 uuid.UUID
	DraftID            uuid.UUID
	Version            int
	Status             string
	SubmittedBy        uuid.UUID
	ReviewerID         *uuid.UUID
	Feedback           *string
	VerificationReport json.RawMessage
	SubmittedAt        time.Time
	ReviewedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
