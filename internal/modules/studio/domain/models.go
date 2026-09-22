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

// KindLessonMaterial is an authored activity that is a document or a video to
// read or watch, not a graded exercise (WO 20, D20-1).
const KindLessonMaterial = "lesson_material"

// AllowedActivityKinds contains the twelve activity kinds a creator may author
// (BR-STUDIO-10). Listening comprehension is excluded in v1. The twelfth kind,
// lesson_material, is the runner's non-graded document/video kind.
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
	KindLessonMaterial:           {},
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
	// TrustedAt is when this creator stopped needing a human for free courses.
	// Nil means every submission of theirs is reviewed.
	TrustedAt           *time.Time
	ApprovedCourseCount int
	// UpheldReportCount counts reports a moderator agreed with. One clears
	// trust: being wrong once puts a creator back in front of a human.
	UpheldReportCount int
	SuspendedAt       *time.Time
	SuspendedReason   *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Trusted reports whether this creator's free courses may publish on the
// automated gate alone.
func (p *CreatorProfile) Trusted() bool {
	return p != nil && p.TrustedAt != nil && p.SuspendedAt == nil
}

// Suspended reports whether this creator may submit or sell at all.
func (p *CreatorProfile) Suspended() bool {
	return p != nil && p.SuspendedAt != nil
}

// TrustThreshold is how many approved courses earn trust (WO 15 §8).
const TrustThreshold = 3

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
	// Gate2Required is decided when the submission is made and kept, not
	// recomputed at review time: a creator who becomes trusted while their
	// submission sits in the queue should not have it silently skip the human
	// who was about to read it.
	Gate2Required bool
	Gate2Reason   *string
	SubmittedAt   time.Time
	ReviewedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
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

// DefaultPayoutThresholdVND is the minimum balance required before a creator can request a payout.
const DefaultPayoutThresholdVND int64 = 500000

// EarningsSummary represents a creator's financial position and ledger overview.
type EarningsSummary struct {
	AvailableBalanceVND     int64
	LifetimeEarningsVND     int64
	PendingPayoutVND        int64
	TotalPaidOutVND         int64
	PayoutThresholdVND      int64
	CanRequestPayout        bool
	PayoutAccountConfigured bool
	PayoutBankCode          *string
	PayoutAccountHolder     *string
	PayoutMaskedAccount     *string
	RecentLedger            []*CreatorLedgerEntry
}

// Takedown is a course removed from sale, and why.
//
// Kept as a record rather than only flipping the listing's status, because
// "when, by whom and on what grounds" is what anybody asks about a takedown
// afterwards, and a status column answers none of it.
type Takedown struct {
	ID           uuid.UUID
	CourseID     uuid.UUID
	ActorID      uuid.UUID
	Reason       string
	ReinstatedAt *time.Time
	ReinstatedBy *uuid.UUID
	CreatedAt    time.Time
}
