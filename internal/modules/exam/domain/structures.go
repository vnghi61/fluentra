// Package domain holds the exam module's rules: the clock, sections and scoring.
package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Exam families supported in the platform.
const (
	FamilyVstep         = "vstep"
	FamilyToeicLR       = "toeic_lr"
	FamilyIeltsAcademic = "ielts_academic"
	FamilyToeflIbt      = "toefl_ibt"
	FamilyCambridgeB2   = "cambridge_b2_first"
	FamilyVnThpt        = "vn_thpt"
)

// Exam sections.
const (
	SectionUseOfEnglish = "use_of_english"
)

// Domain errors for exam structures.
var (
	ErrInvalidSourceURL = apperr.New(
		apperr.Validation, "INVALID_SOURCE_URL", "exam source url must be a valid https link",
	)
	ErrInvalidExamFamily = apperr.New(
		apperr.Validation, "INVALID_EXAM_FAMILY", "unsupported exam family",
	)
	ErrInvalidGroupSize = apperr.New(
		apperr.Validation, "INVALID_GROUP_SIZE", "question count must be divisible by group size",
	)
	ErrInvalidDistribution = apperr.New(
		apperr.Validation, "INVALID_DISTRIBUTION", "blueprint distribution must not be empty",
	)
)

// ExamVersion represents a verified specification of an exam standard.
type ExamVersion struct {
	ID           uuid.UUID       `json:"id"`
	ExamFamily   string          `json:"exam_family"`
	Code         string          `json:"code"`
	Title        string          `json:"title"`
	TotalMinutes int             `json:"total_minutes"`
	Scoring      json.RawMessage `json:"scoring"`
	SourceURL    string          `json:"source_url"`
	VerifiedAt   time.Time       `json:"verified_at"`
	IsCurrent    bool            `json:"is_current"`
	Notes        string          `json:"notes"`
}

// Validate checks constraints on the ExamVersion.
func (v *ExamVersion) Validate() error {
	if !strings.HasPrefix(v.SourceURL, "https://") {
		return ErrInvalidSourceURL
	}
	switch v.ExamFamily {
	case FamilyVstep, FamilyToeicLR, FamilyIeltsAcademic, FamilyToeflIbt, FamilyCambridgeB2, FamilyVnThpt:
		return nil
	default:
		return ErrInvalidExamFamily
	}
}

// ExamPart defines a constituent part of an exam version.
type ExamPart struct {
	ID              uuid.UUID       `json:"id"`
	VersionID       uuid.UUID       `json:"version_id"`
	Section         string          `json:"section"`
	PartNumber      int             `json:"part_number"`
	Kind            string          `json:"kind"`
	QuestionCount   int             `json:"question_count"`
	GroupSize       int             `json:"group_size"`
	DurationMinutes *int            `json:"duration_minutes,omitempty"`
	Constraints     json.RawMessage `json:"constraints"`
}

// Validate ensures questions divide cleanly into groups.
func (p *ExamPart) Validate() error {
	if p.GroupSize <= 0 || p.QuestionCount <= 0 {
		return ErrInvalidGroupSize
	}
	if p.QuestionCount%p.GroupSize != 0 {
		return ErrInvalidGroupSize
	}
	return nil
}

// Blueprint defines composition rules and distributions for a mock test.
type Blueprint struct {
	ID               uuid.UUID       `json:"id"`
	VersionID        uuid.UUID       `json:"version_id"`
	Name             string          `json:"name"`
	CefrDistribution json.RawMessage `json:"cefr_distribution"`
	NodeDistribution json.RawMessage `json:"node_distribution"`
}

// Validate checks that the blueprint has valid distribution data.
func (b *Blueprint) Validate() error {
	if len(b.CefrDistribution) == 0 {
		return ErrInvalidDistribution
	}
	return nil
}

// Mock test modes.
const (
	MockModeFixed     = "fixed"
	MockModeRandom    = "random"
	MockModeWeakTopic = "weak_topic"
	MockModeFull      = "full"
	MockModeCustom    = "custom"
)

// Mock test errors.
var (
	// ErrInvalidMockMode indicates the requested mock test mode is not supported.
	ErrInvalidMockMode = apperr.New(apperr.Validation, "INVALID_MOCK_MODE", "unsupported mock test mode")
	// ErrMockTestNotFound indicates the requested mock test does not exist.
	ErrMockTestNotFound = apperr.New(apperr.NotFound, "MOCK_TEST_NOT_FOUND", "mock test not found")
	// ErrInsufficientQuestionsForPart indicates not enough questions exist to compose a part.
	ErrInsufficientQuestionsForPart = apperr.New(
		apperr.Conflict, "INSUFFICIENT_ITEMS", "insufficient published questions to compose part",
	)
)

// MockTestPartComposition lists activities drawn for a single exam part.
type MockTestPartComposition struct {
	PartID      uuid.UUID   `json:"part_id"`
	ActivityIDs []uuid.UUID `json:"activity_ids"`
}

// MockTest models a composed mock test.
type MockTest struct {
	ID          uuid.UUID `json:"id"`
	BlueprintID uuid.UUID `json:"blueprint_id"`
	Mode        string    `json:"mode"`
	// Number is the fixed test's number ("Đề 3"); nil for every other mode.
	Number      *int                      `json:"number,omitempty"`
	Seed        int64                     `json:"seed"`
	Composition []MockTestPartComposition `json:"composition"`
	OwnerID     *uuid.UUID                `json:"owner_id,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
}

// Validate checks mock test constraints.
func (m *MockTest) Validate() error {
	switch m.Mode {
	case MockModeFixed, MockModeRandom, MockModeWeakTopic, MockModeFull, MockModeCustom:
		return nil
	default:
		return ErrInvalidMockMode
	}
}
