package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/domain"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// ContentReader narrows contentcontract.Reader.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

// AttemptReader loads attempt details across modules.
type AttemptReader interface {
	GetAttemptForGrading(ctx context.Context, attemptID uuid.UUID) (*learningcontract.AttemptDetail, error)
}

// AsyncGradingCompleter completes or fails an attempt asynchronously in learning.
type AsyncGradingCompleter interface {
	CompleteAsyncGrading(ctx context.Context, attemptID uuid.UUID, result learningcontract.GradeResult) (bool, error)
	FailAsyncGrading(ctx context.Context, attemptID uuid.UUID, reason string) (bool, error)
}

// JobEnqueuer enqueues the speaking.grade_recording River job.
type JobEnqueuer interface {
	EnqueueGradeRecording(ctx context.Context, attemptID uuid.UUID) error
}

// WorkerNudger pings the worker to wake up after a job is enqueued.
type WorkerNudger interface {
	Nudge(ctx context.Context)
}

// FeedbackWriter persists speaking feedback records.
type FeedbackWriter interface {
	InsertFeedback(ctx context.Context, fb *contract.SpeakingFeedback) error
}

type speakingTaskBody struct {
	TaskType            string `json:"task_type"` // read_aloud or respond
	Prompt              string `json:"prompt"`
	ReferenceText       string `json:"reference_text,omitempty"`
	SpeakingTimeSeconds int    `json:"speaking_time_seconds,omitempty"`
}

type speakingSubmissionResponse struct {
	AudioObjectKey string `json:"audio_object_key"`
}

func extractRecordingKey(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var resp speakingSubmissionResponse
	if err := json.Unmarshal(raw, &resp); err == nil && resp.AudioObjectKey != "" {
		return strings.TrimSpace(resp.AudioObjectKey)
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.TrimSpace(str)
	}
	return ""
}

// GraderDeps contains all dependencies for constructing the Speaking Grader.
type GraderDeps struct {
	Content     ContentReader
	Attempts    AttemptReader
	Completer   AsyncGradingCompleter
	Storage     storage.Store
	Transcriber media.Transcriber
	AI          ai.Client
	Enqueuer    JobEnqueuer
	Feedback    FeedbackWriter
	Counter     AttemptCounter
	Nudger      WorkerNudger
	Clock       clock.Clock
	Bucket      string
	DailyLimit  int
	ASRModel    string
}

// Grader implements learningcontract.ExerciseGrader and contract.Grader.
type Grader struct {
	content     ContentReader
	attempts    AttemptReader
	completer   AsyncGradingCompleter
	storage     storage.Store
	transcriber media.Transcriber
	ai          ai.Client
	enqueuer    JobEnqueuer
	feedback    FeedbackWriter
	counter     AttemptCounter
	nudger      WorkerNudger
	clock       clock.Clock
	bucket      string
	dailyLimit  int
	asrModel    string
}

// NewGrader constructs a new speaking Grader.
func NewGrader(deps GraderDeps) *Grader {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	b := deps.Bucket
	if b == "" {
		b = storage.BucketMedia
	}
	limit := deps.DailyLimit
	if limit <= 0 {
		limit = 30
	}
	asr := deps.ASRModel
	if asr == "" {
		asr = "whisper-1"
	}

	return &Grader{
		content:     deps.Content,
		attempts:    deps.Attempts,
		completer:   deps.Completer,
		storage:     deps.Storage,
		transcriber: deps.Transcriber,
		ai:          deps.AI,
		enqueuer:    deps.Enqueuer,
		feedback:    deps.Feedback,
		counter:     deps.Counter,
		nudger:      deps.Nudger,
		clock:       timekeeper,
		bucket:      b,
		dailyLimit:  limit,
		asrModel:    asr,
	}
}

func (g *Grader) loadBody(ctx context.Context, versionID uuid.UUID) (speakingTaskBody, error) {
	if g.content == nil {
		return speakingTaskBody{}, apperr.New(apperr.Internal, "CONTENT_READER_REQUIRED", "speaking grader requires a content reader")
	}
	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return speakingTaskBody{}, fmt.Errorf("load speaking version: %w", err)
	}
	if version == nil {
		return speakingTaskBody{}, apperr.New(apperr.NotFound, "CONTENT_VERSION_NOT_FOUND", "speaking content version not found")
	}

	var body speakingTaskBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return speakingTaskBody{}, fmt.Errorf("unmarshal speaking body: %w", err)
		}
	}
	return body, nil
}

// Grade implements learningcontract.ExerciseGrader.
func (g *Grader) Grade(ctx context.Context, req learningcontract.GradeRequest) (learningcontract.GradeResult, error) {
	recordingKey := extractRecordingKey(req.Response)
	if recordingKey == "" {
		return learningcontract.GradeResult{
			Score:    0,
			MaxScore: 100,
			Correct:  false,
			Feedback: "No audio recording submitted.",
			ReviewItems: []learningcontract.ReviewItem{
				{
					ContentVersionID: req.ContentVersionID,
					Skill:            "speaking",
					InitialGrade:     "again",
				},
			},
		}, nil
	}

	// Validate recording key ownership and format
	if !domain.ValidateRecordingKey(recordingKey, req.UserID) {
		return learningcontract.GradeResult{}, domain.ErrInvalidRecordingKey
	}

	// Rate limit check
	if g.counter != nil && g.dailyLimit > 0 {
		startOfDay := startOfLearnerDay(g.clock.Now())
		used, err := g.counter.CountAttemptsTowardLimitSince(ctx, req.UserID, contract.KindSpeakingTask, startOfDay)
		if err != nil {
			return learningcontract.GradeResult{}, fmt.Errorf("count daily speaking attempts: %w", err)
		}
		if used >= g.dailyLimit {
			return learningcontract.GradeResult{}, domain.ErrDailyLimitReached
		}
	}

	if g.enqueuer == nil {
		return learningcontract.GradeResult{}, domain.ErrSpeakingQueueUnavailable
	}

	if err := g.enqueuer.EnqueueGradeRecording(ctx, req.AttemptID); err != nil {
		return learningcontract.GradeResult{}, fmt.Errorf("enqueue speaking grade recording: %w", err)
	}

	if g.nudger != nil {
		g.nudger.Nudge(ctx)
	}

	return learningcontract.GradeResult{
		Async: true,
	}, nil
}

type aiSpeakingCriterionOutput struct {
	Name      string  `json:"name"`
	Band      float64 `json:"band"`
	CommentEn string  `json:"comment_en"`
	CommentVi string  `json:"comment_vi"`
}

type aiSpeakingGradeOutput struct {
	OverallBand float64                     `json:"overall_band"`
	Score       int                         `json:"score"`
	Correct     bool                        `json:"correct"`
	Criteria    []aiSpeakingCriterionOutput `json:"criteria"`
	Feedback    string                      `json:"feedback,omitempty"`
	FeedbackEn  string                      `json:"feedback_en,omitempty"`
	FeedbackVi  string                      `json:"feedback_vi,omitempty"`
}

// GradeSubmission performs the background asynchronous transcription and AI evaluation.
func (g *Grader) GradeSubmission(ctx context.Context, attemptID uuid.UUID, finalAttempt bool) error {
	if g.attempts == nil {
		return fmt.Errorf("attempt reader is required for async speaking grading")
	}
	if g.completer == nil {
		return fmt.Errorf("async grading completer is required")
	}

	attempt, err := g.attempts.GetAttemptForGrading(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("get attempt %s: %w", attemptID, err)
	}

	if attempt.Status != "grading" {
		return nil
	}

	body, err := g.loadBody(ctx, attempt.ContentVersionID)
	if err != nil {
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return fmt.Errorf("load speaking body for attempt %s: %w", attemptID, err)
	}

	recordingKey := extractRecordingKey(attempt.Response)
	if recordingKey == "" {
		err := fmt.Errorf("empty recording key in attempt %s", attemptID)
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return err
	}

	// 1. Download audio from storage
	if g.storage == nil {
		err := fmt.Errorf("storage is required to fetch recording")
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return err
	}

	audioReader, err := g.storage.Get(ctx, g.bucket, recordingKey)
	if err != nil {
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return fmt.Errorf("fetch recording %s: %w", recordingKey, err)
	}
	defer audioReader.Close()

	// 2. Transcribe audio
	if g.transcriber == nil {
		err := fmt.Errorf("transcriber is required for speaking evaluation")
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return err
	}

	filename := path.Base(recordingKey)
	transcribeRes, err := g.transcriber.Transcribe(ctx, audioReader, filename)
	if err != nil {
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return fmt.Errorf("transcribe recording %s: %w", recordingKey, err)
	}

	// 3. Compute metrics in Go
	var readAloudAcc *float64
	if body.TaskType == contract.TypeReadAloud || body.ReferenceText != "" {
		acc := domain.ComputeReadAloudAccuracy(body.ReferenceText, transcribeRes.Text)
		readAloudAcc = &acc
	}

	var wordsPerMinute *int
	wordCount := len(domain.TokenizeWords(transcribeRes.Text))
	if transcribeRes.Duration > 0 {
		wpm := domain.ComputeWordsPerMinute(wordCount, transcribeRes.Duration)
		wordsPerMinute = &wpm
	}

	// 4. Evaluate with AI prompt (transcript only)
	promptText := body.Prompt
	if promptText == "" && body.ReferenceText != "" {
		promptText = "Read aloud: " + body.ReferenceText
	}

	taskType := body.TaskType
	if taskType == "" {
		taskType = contract.TypeRespond
	}

	vars := map[string]any{
		"TaskType":   taskType,
		"Prompt":     promptText,
		"Transcript": transcribeRes.Text,
	}

	var out aiSpeakingGradeOutput
	var modelName string
	if g.ai != nil {
		aiResp, evalErr := ai.CompleteJSONWithResponse(ctx, g.ai, ai.Request{
			Task: ai.TaskGradeSpeaking,
			Vars: vars,
		}, &out)
		if evalErr != nil {
			g.failIfFinal(ctx, attemptID, finalAttempt, evalErr)
			return fmt.Errorf("ai evaluate speaking: %w", evalErr)
		}
		modelName = aiResp.Model
	} else {
		// Fallback for mock/test without AI client
		out = aiSpeakingGradeOutput{
			OverallBand: 6.0,
			Score:       70,
			Correct:     true,
			FeedbackEn:  "Speaking submission evaluated.",
			FeedbackVi:  "Bài nói đã được đánh giá.",
		}
	}

	if out.FeedbackEn == "" {
		out.FeedbackEn = out.Feedback
	}

	// For read-aloud, combine AI score with word accuracy computed in Go
	finalScore := out.Score
	if readAloudAcc != nil {
		finalScore = int(float64(out.Score)*0.3 + (*readAloudAcc)*0.7)
	}
	isCorrect := finalScore >= 60

	// 5. Store speaking feedback
	criteria := make([]contract.SpeakingCriterion, 0, len(out.Criteria))
	for _, c := range out.Criteria {
		criteria = append(criteria, contract.SpeakingCriterion{
			Name:      c.Name,
			Band:      c.Band,
			CommentEn: c.CommentEn,
			CommentVi: c.CommentVi,
		})
	}

	fb := &contract.SpeakingFeedback{
		AttemptID:         attemptID,
		UserID:            attempt.UserID,
		RecordingKey:      recordingKey,
		Transcript:        transcribeRes.Text,
		Criteria:          criteria,
		ReadAloudAccuracy: readAloudAcc,
		WordsPerMinute:    wordsPerMinute,
		FeedbackEn:        out.FeedbackEn,
		FeedbackVi:        out.FeedbackVi,
		PromptVersion:     "speaking_grade.v1",
		Model:             modelName,
		ASRModel:          g.asrModel,
	}

	if g.feedback != nil {
		if err := g.feedback.InsertFeedback(ctx, fb); err != nil {
			return fmt.Errorf("insert speaking feedback: %w", err)
		}
	}

	initialGrade := "again"
	if isCorrect {
		initialGrade = "good"
	}

	gradeResult := learningcontract.GradeResult{
		Score:    finalScore,
		MaxScore: 100,
		Correct:  isCorrect,
		Feedback: out.FeedbackEn,
		Explanation: &learningcontract.AnswerExplanation{
			Text:   out.FeedbackEn,
			TextVi: out.FeedbackVi,
		},
		ReviewItems: []learningcontract.ReviewItem{
			{
				ContentVersionID: attempt.ContentVersionID,
				Skill:            "speaking",
				InitialGrade:     initialGrade,
			},
		},
	}

	if _, err := g.completer.CompleteAsyncGrading(ctx, attemptID, gradeResult); err != nil {
		return fmt.Errorf("complete async grading: %w", err)
	}

	return nil
}

func (g *Grader) failIfFinal(ctx context.Context, attemptID uuid.UUID, finalAttempt bool, err error) {
	if !finalAttempt || g.completer == nil {
		return
	}
	_, _ = g.completer.FailAsyncGrading(ctx, attemptID, err.Error())
}
