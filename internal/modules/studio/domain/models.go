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

// Pricing models.
const (
	PricingModelFree    = "free"
	PricingModelOneTime = "one_time"
)

// Listing statuses.
const (
	ListingStatusActive    = "active"
	ListingStatusUnlisted  = "unlisted"
	ListingStatusTakenDown = "taken_down"
)

// Creator ledger kinds.
const (
	LedgerKindSale          = "sale"
	LedgerKindRefund        = "refund"
	LedgerKindPayout        = "payout"
	LedgerKindAdjustment    = "adjustment"
	LedgerKindPlatformShare = "platform_share"
)

// Price defaults and bounds (BR-STUDIO-01).
const (
	DefaultMinPriceVND     int64 = 49_000
	DefaultMaxPriceVND     int64 = 5_000_000
	DefaultRevenueShareBPS int   = 7000
)

// Listing domain model.
type Listing struct {
	CourseID        uuid.UUID
	CreatorID       uuid.UUID
	PricingModel    string
	PriceVND        int64
	RevenueShareBPS int
	Status          string
	PublishedAt     time.Time
	UpdatedAt       time.Time
}

// Purchase domain model.
type Purchase struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	CourseID     uuid.UUID
	OrderID      *uuid.UUID
	PricePaidVND int64
	GrantedAt    time.Time
	RevokedAt    *time.Time
	RevokeReason *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreatorLedgerEntry domain model.
type CreatorLedgerEntry struct {
	ID             uuid.UUID
	CreatorID      uuid.UUID
	Kind           string
	AmountVND      int64
	GrossAmountVND int64
	FeeAmountVND   int64
	PurchaseID     *uuid.UUID
	PayoutID       *uuid.UUID
	Note           string
	CreatedAt      time.Time
}

// ValidatePrice validates pricing bounds according to BR-STUDIO-01.
func ValidatePrice(pricingModel string, priceVND, minVND, maxVND int64) error {
	if pricingModel == PricingModelFree {
		if priceVND != 0 {
			return ErrPriceOutOfBounds
		}
		return nil
	}
	if pricingModel == PricingModelOneTime {
		if priceVND < minVND || priceVND > maxVND {
			return ErrPriceOutOfBounds
		}
		return nil
	}
	return ErrPriceOutOfBounds
}

