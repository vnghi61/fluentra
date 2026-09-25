package contract

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Supported activity kinds in question bank.
const (
	KindPhotoDescription = "photo_description"
	KindQuestionResponse = "question_response"
	KindMcqGap           = "mcq_gap"
	KindTextCompletion   = "text_completion"
)

// Question lifecycle statuses.
const (
	StatusDraft     = "draft"
	StatusInReview  = "in_review"
	StatusPublished = "published"
	StatusRetired   = "retired"
)

// Event names published by questionbank.
const (
	EventItemPublished = "questionbank.item_published"
	Aggregate          = "questionbank"
)

// Question is an assessment item in the bank.
type Question struct {
	ID            uuid.UUID      `json:"id"`
	ContentItemID uuid.UUID      `json:"content_item_id"`
	ActivityID    *uuid.UUID     `json:"activity_id,omitempty"`
	ExamPartID    *uuid.UUID     `json:"exam_part_id,omitempty"`
	Kind          string         `json:"kind"`
	Skill         string         `json:"skill"`
	CEFRLevel     string         `json:"cefr_level"`
	Difficulty    *float64       `json:"difficulty,omitempty"`
	QuestionCount int            `json:"question_count"`
	Fingerprint   string         `json:"fingerprint"`
	Provenance    map[string]any `json:"provenance"`
	Status        string         `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// QuestionStats holds empirical difficulty and discrimination statistics.
type QuestionStats struct {
	QuestionID     uuid.UUID `json:"question_id"`
	Attempts       int       `json:"attempts"`
	PValue         *float64  `json:"p_value,omitempty"`
	Discrimination *float64  `json:"discrimination,omitempty"`
	AvgTimeMs      int       `json:"avg_time_ms"`
	LastComputedAt time.Time `json:"last_computed_at"`
}

// Filter defines search criteria for listing questions.
type Filter struct {
	ExamVersion *string
	ExamPartID  *uuid.UUID
	Kind        *string
	Skill       *string
	CEFRLevel   *string
	NodeCode    *string
	Status      *string
	Limit       int
	Offset      int
}

// SampleCriteria defines criteria for drawing random published questions.
type SampleCriteria struct {
	Kind       *string
	Skill      *string
	CEFRLevel  *string
	ExamPartID *uuid.UUID
	Limit      int
}

// GenerateRequest describes an AI generation job for questions.
type GenerateRequest struct {
	ExamVersion *string    `json:"exam_version,omitempty"`
	ExamPartID  *uuid.UUID `json:"exam_part_id,omitempty"`
	Kind        string     `json:"kind"`
	Skill       string     `json:"skill"`
	CEFRLevel   string     `json:"cefr_level"`
	NodeCodes   []string   `json:"node_codes,omitempty"`
	Count       int        `json:"count"`
	// Photo is the photograph a TOEIC Part 1 question is written from
	// (D22-21); each request with one generates one question.
	Photo *Photo `json:"photo,omitempty"`
}

// Photo is an openly licensed photograph with its credit and a person's
// description of it.
type Photo struct {
	URL         string `json:"url"`
	CreditPage  string `json:"credit_page"`
	Licence     string `json:"licence"`
	Description string `json:"description"`
}

// Reader provides read-only access to question bank items.
type Reader interface {
	GetQuestion(ctx context.Context, id uuid.UUID) (*Question, error)
	ListQuestions(ctx context.Context, filter Filter) ([]*Question, int, error)
	SampleQuestions(ctx context.Context, criteria SampleCriteria) ([]*Question, error)
	GetQuestionStats(ctx context.Context, id uuid.UUID) (*QuestionStats, error)
	// DrawableForPart is the system read an exam composes from: the published
	// questions of one part that have an activity, in a stable order. It carries
	// no permission check — a learner composing a mock test holds no
	// questionbank permission, and nothing unpublished can come back.
	DrawableForPart(ctx context.Context, examPartID uuid.UUID) ([]*Question, error)
}

// Author provides question bank modification and generation operations.
type Author interface {
	CreateQuestion(ctx context.Context, q *Question) (*Question, error)
	GenerateQuestions(ctx context.Context, req GenerateRequest) ([]*Question, error)
	PublishQuestion(ctx context.Context, id uuid.UUID) (*Question, error)
	RetireQuestion(ctx context.Context, id uuid.UUID) (*Question, error)
}

// BatchPrefix is the start of every generation batch id for one exam version:
// the review queue groups a run's doubts by batch, and the daily job reads how
// many of an exam's batches wait for a person by this prefix (WO 22 Stage O).
func BatchPrefix(examVersion string) string {
	if examVersion == "" {
		examVersion = "any"
	}
	return "bank:" + examVersion + ":"
}
