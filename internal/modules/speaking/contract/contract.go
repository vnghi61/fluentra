// Package contract defines the public interfaces and types for the speaking module.
package contract

import (
	"context"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// Exercise kinds and subtypes for spoken practice.
const (
	KindSpeakingTask = "speaking_task"

	TypeReadAloud = "read_aloud"
	TypeRespond   = "respond"
)

// GradedKinds returns the activity kinds graded by the speaking module.
func GradedKinds() []string {
	return []string{KindSpeakingTask}
}

// SpeakingCriterion represents an evaluated criterion for a speaking attempt.
type SpeakingCriterion struct {
	Name      string  `json:"name"`
	Band      float64 `json:"band"`
	CommentEn string  `json:"comment_en"`
	CommentVi string  `json:"comment_vi"`
}

// SpeakingFeedback is the structured result stored for a graded speaking attempt.
type SpeakingFeedback struct {
	ID                 uuid.UUID           `json:"id"`
	AttemptID          uuid.UUID           `json:"attempt_id"`
	UserID             uuid.UUID           `json:"user_id"`
	RecordingKey       string              `json:"recording_key"`
	RecordingDeletedAt *time.Time          `json:"recording_deleted_at,omitempty"`
	Transcript         string              `json:"transcript"`
	Criteria           []SpeakingCriterion `json:"criteria"`
	ReadAloudAccuracy  *float64            `json:"read_aloud_accuracy,omitempty"`
	WordsPerMinute     *int                `json:"words_per_minute,omitempty"`
	FeedbackEn         string              `json:"feedback_en"`
	FeedbackVi         string              `json:"feedback_vi"`
	PromptVersion      string              `json:"prompt_version"`
	Model              string              `json:"model"`
	ASRModel           string              `json:"asr_model"`
	AudioURL           string              `json:"audio_url,omitempty"`
	// The grade as it was given. Pointers because a row written before these
	// columns existed has no score, and reporting "none" is honest where
	// reconstructing one is not.
	OverallBand *float64  `json:"overall_band,omitempty"`
	Score       *int      `json:"score,omitempty"`
	TaskType    string    `json:"task_type,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SpeakingSubmissionSummary is a lightweight overview of a learner's speaking submission.
type SpeakingSubmissionSummary struct {
	AttemptID uuid.UUID `json:"attempt_id"`
	Status    string    `json:"status"`
	// Null for an attempt graded before the grade was persisted; the screen says
	// so rather than printing a number that was never awarded.
	OverallBand  *float64  `json:"overall_band"`
	Score        *int      `json:"score"`
	HasRecording bool      `json:"has_recording"`
	TaskType     string    `json:"task_type"`
	FeedbackEn   string    `json:"feedback_en,omitempty"`
	FeedbackVi   string    `json:"feedback_vi,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SpeakingSubmissionList is a paginated collection of speaking submissions.
type SpeakingSubmissionList struct {
	Items    []SpeakingSubmissionSummary `json:"items"`
	Total    int                         `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
}

// SpeakingConsent is a learner's recorded agreement to be recorded.
//
// BR-SPEAKING-03 requires the timestamp, not merely the fact: voice is
// biometric-adjacent personal data, and "when" is half the record.
type SpeakingConsent struct {
	Consented   bool       `json:"consented"`
	ConsentedAt *time.Time `json:"consented_at"`
}

// UploadIntentResult represents the presigned PUT URL and quota metadata for uploading audio.
type UploadIntentResult struct {
	UploadURL            string    `json:"upload_url"`
	ObjectKey            string    `json:"object_key"`
	ExpiresAt            time.Time `json:"expires_at"`
	DailyRecordingsUsed  int       `json:"daily_recordings_used"`
	DailyRecordingsLimit int       `json:"daily_recordings_limit"`
}

// Grader evaluates speaking exercises.
type Grader interface {
	learningcontract.ExerciseGrader
}

// ConsentKeeper reads and records voice-recording consent.
type ConsentKeeper interface {
	GetConsent(ctx context.Context, userID uuid.UUID) (*SpeakingConsent, error)
	RecordConsent(ctx context.Context, userID uuid.UUID) (*SpeakingConsent, error)
}

// FeedbackReader retrieves speaking feedback and submission lists.
type FeedbackReader interface {
	GetSpeakingFeedback(ctx context.Context, attemptID, userID uuid.UUID) (*SpeakingFeedback, error)
	ListSpeakingSubmissions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*SpeakingSubmissionList, error)
}

// RecordingCleaner purges audio recordings for GDPR / account erasure.
type RecordingCleaner interface {
	DeleteUserRecordings(ctx context.Context, userID uuid.UUID) error
}
