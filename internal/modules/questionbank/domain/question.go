package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Domain errors for question bank.
var (
	// ErrInvalidStatus is returned when question status is not one of the allowed values.
	ErrInvalidStatus = errors.New("invalid question status")
	// ErrInvalidCount is returned when question count is out of bounds [1, 20].
	ErrInvalidCount = errors.New("question count must be between 1 and 20")
	// ErrEmptyFingerprint is returned when fingerprint is empty.
	ErrEmptyFingerprint = errors.New("fingerprint cannot be empty")
	// ErrQuestionNotFound is returned when a requested question does not exist.
	ErrQuestionNotFound = errors.New("question not found")
	// ErrInvalidDifficulty is returned when difficulty is not in [0.0, 1.0].
	ErrInvalidDifficulty = errors.New("difficulty must be between 0.0 and 1.0")
)

// Valid question statuses.
const (
	StatusDraft     = "draft"
	StatusInReview  = "in_review"
	StatusPublished = "published"
	StatusRetired   = "retired"
)

// Question is the domain model of an assessment question.
type Question struct {
	ID            uuid.UUID
	ContentItemID uuid.UUID
	ActivityID    *uuid.UUID
	ExamPartID    *uuid.UUID
	Kind          string
	Skill         string
	CEFRLevel     string
	Difficulty    *float64
	QuestionCount int
	Fingerprint   string
	Provenance    map[string]any
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Validate checks question business invariants.
func (q *Question) Validate() error {
	if q.QuestionCount < 1 || q.QuestionCount > 20 {
		return ErrInvalidCount
	}
	if q.Fingerprint == "" {
		return ErrEmptyFingerprint
	}
	switch q.Status {
	case StatusDraft, StatusInReview, StatusPublished, StatusRetired:
	default:
		return ErrInvalidStatus
	}
	if q.Difficulty != nil && (*q.Difficulty < 0.0 || *q.Difficulty > 1.0) {
		return ErrInvalidDifficulty
	}
	return nil
}

// QuestionStats holds empirical statistics for a question.
type QuestionStats struct {
	QuestionID     uuid.UUID
	Attempts       int
	PValue         *float64
	Discrimination *float64
	AvgTimeMs      int
	LastComputedAt time.Time
}
