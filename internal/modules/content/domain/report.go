package domain

import (
	"time"

	"github.com/google/uuid"
)

// ReportReason represents why an item is reported.
type ReportReason string

const (
	ReportReasonWrongAnswer      ReportReason = "wrong_answer"
	ReportReasonUnclear          ReportReason = "unclear"
	ReportReasonTypo             ReportReason = "typo"
	ReportReasonMyAnswerWasRight ReportReason = "my_answer_was_right"
	ReportReasonOther            ReportReason = "other"
)

// IsValidReportReason validates if the reason is recognized.
func IsValidReportReason(r string) bool {
	switch ReportReason(r) {
	case ReportReasonWrongAnswer, ReportReasonUnclear, ReportReasonTypo, ReportReasonMyAnswerWasRight, ReportReasonOther:
		return true
	default:
		return false
	}
}

// ItemReport represents a learner's report against a content version.
type ItemReport struct {
	ID               uuid.UUID
	ContentVersionID uuid.UUID
	UserID           uuid.UUID
	Reason           ReportReason
	Note             *string
	CreatedAt        time.Time
}

// ReportedVersionSummary aggregates reports for admin overview.
type ReportedVersionSummary struct {
	ContentVersionID uuid.UUID
	ItemID           uuid.UUID
	Slug             string
	Kind             string
	CEFRLevel        string
	ItemStatus       string
	ReportCount      int
	LastReportedAt   time.Time
}
