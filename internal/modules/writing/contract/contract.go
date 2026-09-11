package contract

import (
	"context"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// KindWritingPrompt is an open-ended writing exercise graded by rubric and AI.
	KindWritingPrompt = "writing_prompt"
)

// GradedKinds are the activity kinds this module's grader can score.
func GradedKinds() []string {
	return []string{
		KindWritingPrompt,
	}
}

// Grader defines the exercise grading contract implemented by the writing module.
type Grader interface {
	learningcontract.ExerciseGrader
	learningcontract.MeteredGrader
}

// WritingCriterion is one of the four IELTS-style rubric criteria.
type WritingCriterion struct {
	Name      string  `json:"name"`
	Band      float64 `json:"band"`
	CommentEn string  `json:"comment_en"`
	CommentVi string  `json:"comment_vi"`
}

// WritingAnnotation is a located span in the essay with bilingual comments.
// Offsets are rune-based (matching JavaScript string indices).
type WritingAnnotation struct {
	QuotedText  string `json:"quoted_text"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	CommentEn   string `json:"comment_en"`
	CommentVi   string `json:"comment_vi"`
}

// WritingFeedback contains the full structured feedback for a graded writing attempt.
type WritingFeedback struct {
	AttemptID     uuid.UUID           `json:"attempt_id"`
	UserID        uuid.UUID           `json:"user_id"`
	OverallBand   float64             `json:"overall_band"`
	Score         int                 `json:"score"`
	Criteria      []WritingCriterion  `json:"criteria"`
	Annotations   []WritingAnnotation `json:"annotations"`
	FeedbackEn    string              `json:"feedback_en"`
	FeedbackVi    string              `json:"feedback_vi"`
	PromptVersion string              `json:"prompt_version"`
	Model         string              `json:"model"`
	CreatedAt     time.Time           `json:"created_at"`
}

// FeedbackReader provides read access to writing feedback for the HTTP handler.
type FeedbackReader interface {
	GetWritingFeedback(ctx context.Context, attemptID, userID uuid.UUID) (*WritingFeedback, error)
}
