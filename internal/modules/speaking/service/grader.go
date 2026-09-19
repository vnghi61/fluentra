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
		return speakingTaskBody{}, apperr.New(
			apperr.Internal, "CONTENT_READER_REQUIRED", "speaking grader requires a content reader",
		)
	}
	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return speakingTaskBody{}, fmt.Errorf("load speaking version: %w", err)
	}
	if version == nil {
		return speakingTaskBody{}, apperr.New(
			apperr.NotFound, "CONTENT_VERSION_NOT_FOUND", "speaking content version not found",
		)
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

	// The attempt's response is only a key the client wrote; the object behind it
	// must exist before the attempt is accepted (work order 12 §3.5).
	if g.storage == nil {
		return learningcontract.GradeResult{}, domain.ErrSpeakingQueueUnavailable
	}
	if _, err := g.storage.Stat(ctx, g.bucket, recordingKey); err != nil {
		return learningcontract.GradeResult{}, domain.ErrRecordingNotFound
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

	fb, gradeResult, err := g.evaluate(ctx, attempt)
	if err != nil {
		g.failIfFinal(ctx, attemptID, finalAttempt, err)
		return err
	}

	if g.feedback != nil {
		if err := g.feedback.InsertFeedback(ctx, fb); err != nil {
			return fmt.Errorf("insert speaking feedback: %w", err)
		}
	}

	if _, err := g.completer.CompleteAsyncGrading(ctx, attemptID, gradeResult); err != nil {
		return fmt.Errorf("complete async grading: %w", err)
	}
	return nil
}

// evaluate transcribes the recording, scores the transcript, and returns the
// feedback to store and the grade to complete the attempt with.
func (g *Grader) evaluate(
	ctx context.Context, attempt *learningcontract.AttemptDetail,
) (*contract.SpeakingFeedback, learningcontract.GradeResult, error) {
	body, err := g.loadBody(ctx, attempt.ContentVersionID)
	if err != nil {
		return nil, learningcontract.GradeResult{}, fmt.Errorf("load speaking body for attempt %s: %w", attempt.ID, err)
	}

	recordingKey := extractRecordingKey(attempt.Response)
	if recordingKey == "" {
		return nil, learningcontract.GradeResult{}, fmt.Errorf("empty recording key in attempt %s", attempt.ID)
	}

	transcript, err := g.transcribe(ctx, recordingKey)
	if err != nil {
		return nil, learningcontract.GradeResult{}, err
	}

	readAloudAcc, wordsPerMinute := speechMetrics(body, transcript)

	out, modelName, err := g.judge(ctx, body, transcript.Text)
	if err != nil {
		return nil, learningcontract.GradeResult{}, err
	}

	// For read-aloud, the word accuracy computed in Go carries most of the score.
	finalScore := out.Score
	if readAloudAcc != nil {
		finalScore = int(float64(out.Score)*0.3 + (*readAloudAcc)*0.7)
	}
	isCorrect := finalScore >= 60

	criteria := make([]contract.SpeakingCriterion, 0, len(out.Criteria))
	for _, c := range out.Criteria {
		criteria = append(criteria, contract.SpeakingCriterion(c))
	}

	fb := &contract.SpeakingFeedback{
		AttemptID:         attempt.ID,
		UserID:            attempt.UserID,
		RecordingKey:      recordingKey,
		Transcript:        transcript.Text,
		Criteria:          criteria,
		ReadAloudAccuracy: readAloudAcc,
		WordsPerMinute:    wordsPerMinute,
		FeedbackEn:        out.FeedbackEn,
		FeedbackVi:        out.FeedbackVi,
		PromptVersion:     "speaking_grade.v1",
		Model:             modelName,
		ASRModel:          g.asrModel,
		// Kept, not left to be reconstructed later. The history screen reads
		// these; deriving a replacement from the criteria gave a different
		// number for the same attempt.
		OverallBand: &out.OverallBand,
		Score:       &finalScore,
		TaskType:    taskTypeOf(body),
	}

	initialGrade := "again"
	if isCorrect {
		initialGrade = "good"
	}
	result := learningcontract.GradeResult{
		Score:    finalScore,
		MaxScore: 100,
		Correct:  isCorrect,
		Feedback: out.FeedbackEn,
		Explanation: &learningcontract.AnswerExplanation{
			Text:   out.FeedbackEn,
			TextVi: out.FeedbackVi,
		},
		ReviewItems: []learningcontract.ReviewItem{
			{ContentVersionID: attempt.ContentVersionID, Skill: "speaking", InitialGrade: initialGrade},
		},
	}
	return fb, result, nil
}

// taskTypeOf reports the authored task type, defaulting the way judge() does.
func taskTypeOf(body speakingTaskBody) string {
	if body.TaskType != "" {
		return body.TaskType
	}
	return contract.TypeRespond
}

// transcribe fetches the recording from storage and sends it to the transcriber.
func (g *Grader) transcribe(ctx context.Context, recordingKey string) (*media.TranscribeResult, error) {
	if g.storage == nil {
		return nil, fmt.Errorf("storage is required to fetch recording")
	}
	if g.transcriber == nil {
		return nil, fmt.Errorf("transcriber is required for speaking evaluation")
	}
	audioReader, err := g.storage.Get(ctx, g.bucket, recordingKey)
	if err != nil {
		return nil, fmt.Errorf("fetch recording %s: %w", recordingKey, err)
	}
	defer func() { _ = audioReader.Close() }()

	result, err := g.transcriber.Transcribe(ctx, audioReader, path.Base(recordingKey))
	if err != nil {
		return nil, fmt.Errorf("transcribe recording %s: %w", recordingKey, err)
	}
	return result, nil
}

// speechMetrics computes read-aloud accuracy and speaking rate in Go, not by asking the model.
func speechMetrics(body speakingTaskBody, transcript *media.TranscribeResult) (*float64, *int) {
	var readAloudAcc *float64
	if body.TaskType == contract.TypeReadAloud || body.ReferenceText != "" {
		acc := domain.ComputeReadAloudAccuracy(body.ReferenceText, transcript.Text)
		readAloudAcc = &acc
	}
	var wordsPerMinute *int
	if transcript.Duration > 0 {
		wpm := domain.ComputeWordsPerMinute(len(domain.TokenizeWords(transcript.Text)), transcript.Duration)
		wordsPerMinute = &wpm
	}
	return readAloudAcc, wordsPerMinute
}

// judge asks the model to score the transcript. The model never receives audio.
func (g *Grader) judge(
	ctx context.Context, body speakingTaskBody, transcript string,
) (aiSpeakingGradeOutput, string, error) {
	promptText := body.Prompt
	if promptText == "" && body.ReferenceText != "" {
		promptText = "Read aloud: " + body.ReferenceText
	}
	taskType := body.TaskType
	if taskType == "" {
		taskType = contract.TypeRespond
	}

	var out aiSpeakingGradeOutput
	if g.ai == nil {
		// Fallback for tests without an AI client.
		out = aiSpeakingGradeOutput{
			OverallBand: 6.0,
			Score:       70,
			Correct:     true,
			FeedbackEn:  "Speaking submission evaluated.",
			FeedbackVi:  "Bài nói đã được đánh giá.",
		}
		return out, "", nil
	}

	resp, err := ai.CompleteJSONWithResponse(ctx, g.ai, ai.Request{
		Task: ai.TaskGradeSpeaking,
		Vars: map[string]any{"TaskType": taskType, "Prompt": promptText, "Transcript": transcript},
	}, &out)
	if err != nil {
		return out, "", fmt.Errorf("ai evaluate speaking: %w", err)
	}
	if out.FeedbackEn == "" {
		out.FeedbackEn = out.Feedback
	}
	return out, resp.Model, nil
}

func (g *Grader) failIfFinal(ctx context.Context, attemptID uuid.UUID, finalAttempt bool, err error) {
	if !finalAttempt || g.completer == nil {
		return
	}
	_, _ = g.completer.FailAsyncGrading(ctx, attemptID, err.Error())
}
