// Package service implements the business logic and graders for the writing module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// ContentReader narrows contentcontract.Reader to what Grader needs.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

// AttemptCounter counts daily graded attempts for a user.
type AttemptCounter interface {
	CountGradedAttemptsSince(ctx context.Context, userID uuid.UUID, grader string, since time.Time) (int, error)
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

// JobEnqueuer enqueues the writing.grade_submission River job.
type JobEnqueuer interface {
	EnqueueGradeSubmission(ctx context.Context, attemptID uuid.UUID) error
}

// WorkerNudger pings the worker to wake up after a job is enqueued.
type WorkerNudger interface {
	Nudge(ctx context.Context)
}

const (
	gradeAgain = "again"
	gradeGood  = "good"

	statusGrading = "grading"

	skillWriting    = "writing"
	maxWritingScore = 100
)

type writingPromptBody struct {
	Prompt        string                              `json:"prompt"`
	Rubric        string                              `json:"rubric,omitempty"`
	MinWords      int                                 `json:"min_words,omitempty"`
	SampleAnswer  string                              `json:"sample_answer,omitempty"`
	CorrectAnswer string                              `json:"correct_answer,omitempty"`
	Explanation   *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

type writingResponse struct {
	TextAnswer string `json:"text_answer,omitempty"`
	Answer     string `json:"answer,omitempty"`
}

// FeedbackRepo stores writing feedback across modules.
type FeedbackRepo interface {
	InsertWritingFeedback(ctx context.Context, fb contract.WritingFeedback) error
}

type aiCriterionOutput struct {
	Name      string  `json:"name"`
	Band      float64 `json:"band"`
	CommentEn string  `json:"comment_en"`
	CommentVi string  `json:"comment_vi"`
}

type aiGradeV2Output struct {
	OverallBand float64             `json:"overall_band"`
	Score       int                 `json:"score"`
	Correct     bool                `json:"correct"`
	Criteria    []aiCriterionOutput `json:"criteria"`
	Annotations []rawAnnotation     `json:"annotations"`
	Feedback    string              `json:"feedback,omitempty"`
	FeedbackEn  string              `json:"feedback_en,omitempty"`
	FeedbackVi  string              `json:"feedback_vi,omitempty"`
}

// GraderDeps contains dependencies for constructing a Grader.
type GraderDeps struct {
	Content    ContentReader
	AI         ai.Client
	Counter    AttemptCounter
	Attempts   AttemptReader
	Completer  AsyncGradingCompleter
	Feedback   FeedbackRepo
	Enqueuer   JobEnqueuer
	Nudger     WorkerNudger
	Clock      clock.Clock
	DailyLimit int
}

// Grader implements learningcontract.ExerciseGrader and contract.Grader.
type Grader struct {
	content    ContentReader
	ai         ai.Client
	counter    AttemptCounter
	attempts   AttemptReader
	completer  AsyncGradingCompleter
	feedback   FeedbackRepo
	enqueuer   JobEnqueuer
	nudger     WorkerNudger
	clock      clock.Clock
	dailyLimit int
}

// NewGrader constructs a Grader with ContentReader and AI client.
func NewGrader(content ContentReader, aiClient ai.Client) *Grader {
	return NewGraderWithDeps(GraderDeps{
		Content: content,
		AI:      aiClient,
	})
}

// NewGraderWithDeps constructs a Grader with full dependencies.
func NewGraderWithDeps(deps GraderDeps) *Grader {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}
	return &Grader{
		content:    deps.Content,
		ai:         deps.AI,
		counter:    deps.Counter,
		attempts:   deps.Attempts,
		completer:  deps.Completer,
		feedback:   deps.Feedback,
		enqueuer:   deps.Enqueuer,
		nudger:     deps.Nudger,
		clock:      timekeeper,
		dailyLimit: deps.DailyLimit,
	}
}

func (g *Grader) loadBody(ctx context.Context, versionID uuid.UUID) (writingPromptBody, error) {
	if g.content == nil {
		return writingPromptBody{}, apperr.New(
			apperr.Internal,
			"CONTENT_READER_REQUIRED",
			"writing grader requires a content reader",
		)
	}

	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return writingPromptBody{}, fmt.Errorf("load writing content version: %w", err)
	}
	if version == nil {
		return writingPromptBody{}, apperr.New(
			apperr.NotFound,
			"CONTENT_VERSION_NOT_FOUND",
			"writing content version not found",
		)
	}

	var body writingPromptBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return writingPromptBody{}, fmt.Errorf("unmarshal writing content body: %w", err)
		}
	}
	return body, nil
}

// Grade implements learningcontract.ExerciseGrader for writing prompt activities.
func (g *Grader) Grade(
	ctx context.Context, req learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	body, err := g.loadBody(ctx, req.ContentVersionID)
	if err != nil {
		return learningcontract.GradeResult{}, err
	}

	submitted := submittedText(req.Response)
	trimmed := strings.TrimSpace(submitted)
	words := len(strings.Fields(trimmed))

	// 1. Empty submission -> scored 0 immediately, no model call
	if words == 0 {
		return buildResult(
			req.ContentVersionID,
			0,
			false,
			"No response provided. Please write an answer to the prompt.",
			body,
			body.Explanation,
		), nil
	}

	// 2. Under min_words -> proportional score immediately, no model call
	if body.MinWords > 0 && words < body.MinWords {
		score := (words * 50) / body.MinWords
		short := fmt.Sprintf(
			"Your response is %d words, but the prompt asks for at least %d words.",
			words, body.MinWords,
		)
		return buildResult(req.ContentVersionID, score, false, short, body, body.Explanation), nil
	}

	// 3. Over daily limit -> 429 WRITING_DAILY_LIMIT_REACHED; attempt stays resubmittable
	if g.counter != nil && g.dailyLimit > 0 {
		now := g.clock.Now().UTC()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		count, err := g.counter.CountGradedAttemptsSince(ctx, req.UserID, "writing_prompt", startOfDay)
		if err != nil {
			return learningcontract.GradeResult{}, fmt.Errorf("count daily graded attempts: %w", err)
		}
		if count >= g.dailyLimit {
			return learningcontract.GradeResult{}, apperr.New(
				apperr.RateLimited,
				"WRITING_DAILY_LIMIT_REACHED",
				"daily limit of graded writing submissions reached",
			)
		}
	}

	// 4. Otherwise: enqueue the job and return Async: true
	if g.enqueuer == nil {
		out, _, evalErr := g.evaluateAIv2(ctx, submitted, body)
		if evalErr != nil {
			return learningcontract.GradeResult{}, evalErr
		}
		feedback := out.FeedbackEn
		if feedback == "" {
			feedback = out.Feedback
		}
		var exp *learningcontract.AnswerExplanation
		if out.FeedbackVi != "" {
			exp = &learningcontract.AnswerExplanation{
				Text:   feedback,
				TextVi: out.FeedbackVi,
			}
		} else if body.Explanation != nil {
			exp = body.Explanation
		}
		return buildResult(req.ContentVersionID, out.Score, out.Correct, feedback, body, exp), nil
	}

	if err := g.enqueuer.EnqueueGradeSubmission(ctx, req.AttemptID); err != nil {
		return learningcontract.GradeResult{}, fmt.Errorf("enqueue writing grade submission: %w", err)
	}

	if g.nudger != nil {
		g.nudger.Nudge(ctx)
	}

	return learningcontract.GradeResult{
		Async: true,
	}, nil
}

// GradeSubmission processes an enqueued writing submission in the background worker.
func (g *Grader) GradeSubmission(ctx context.Context, attemptID uuid.UUID) error {
	if g.attempts == nil {
		return fmt.Errorf("attempt reader is required for async grading")
	}
	if g.completer == nil {
		return fmt.Errorf("async grading completer is required")
	}

	attempt, err := g.attempts.GetAttemptForGrading(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("get attempt %s: %w", attemptID, err)
	}

	// A job run against an attempt the sweep already failed changes nothing.
	if attempt.Status != statusGrading {
		return nil
	}

	body, err := g.loadBody(ctx, attempt.ContentVersionID)
	if err != nil {
		_, _ = g.completer.FailAsyncGrading(ctx, attemptID, err.Error())
		return fmt.Errorf("load writing body for attempt %s: %w", attemptID, err)
	}

	submitted := submittedText(attempt.Response)
	out, modelName, evalErr := g.evaluateAIv2(ctx, submitted, body)
	if evalErr != nil {
		// A provider that errors leaves the attempt failed: no score, no progress row,
		// no review card, nothing counted against the limit.
		_, _ = g.completer.FailAsyncGrading(ctx, attemptID, evalErr.Error())
		return evalErr
	}

	feedback := out.FeedbackEn
	if feedback == "" {
		feedback = out.Feedback
	}

	locatedAnnotations := locateAnnotations(submitted, out.Annotations)

	var criteria []contract.WritingCriterion
	for _, c := range out.Criteria {
		criteria = append(criteria, contract.WritingCriterion{
			Name:      c.Name,
			Band:      c.Band,
			CommentEn: c.CommentEn,
			CommentVi: c.CommentVi,
		})
	}

	if g.feedback != nil {
		fb := contract.WritingFeedback{
			AttemptID:     attemptID,
			UserID:        attempt.UserID,
			OverallBand:   out.OverallBand,
			Score:         out.Score,
			Criteria:      criteria,
			Annotations:   locatedAnnotations,
			FeedbackEn:    feedback,
			FeedbackVi:    out.FeedbackVi,
			PromptVersion: "writing_grade.v2",
			Model:         modelName,
		}
		if err := g.feedback.InsertWritingFeedback(ctx, fb); err != nil {
			return fmt.Errorf("insert writing feedback for attempt %s: %w", attemptID, err)
		}
	}

	var explanation *learningcontract.AnswerExplanation
	if out.FeedbackVi != "" {
		explanation = &learningcontract.AnswerExplanation{
			Text:   feedback,
			TextVi: out.FeedbackVi,
		}
	} else if body.Explanation != nil {
		explanation = body.Explanation
	}

	result := buildResult(attempt.ContentVersionID, out.Score, out.Correct, feedback, body, explanation)
	_, compErr := g.completer.CompleteAsyncGrading(ctx, attemptID, result)
	if compErr != nil {
		return fmt.Errorf("complete async grading for attempt %s: %w", attemptID, compErr)
	}

	return nil
}

func submittedText(response json.RawMessage) string {
	if len(response) == 0 {
		return ""
	}

	var resp writingResponse
	if err := json.Unmarshal(response, &resp); err == nil {
		if resp.TextAnswer != "" {
			return resp.TextAnswer
		}
		if resp.Answer != "" {
			return resp.Answer
		}
	}

	var raw string
	if err := json.Unmarshal(response, &raw); err == nil {
		return raw
	}

	return ""
}

func (g *Grader) evaluateAIv2(
	ctx context.Context,
	submitted string,
	body writingPromptBody,
) (aiGradeV2Output, string, error) {
	trimmed := strings.TrimSpace(submitted)
	words := len(strings.Fields(trimmed))

	if words == 0 {
		return aiGradeV2Output{
			OverallBand: 0,
			Score:       0,
			Correct:     false,
			FeedbackEn:  "No response provided. Please write an answer to the prompt.",
			Feedback:    "No response provided. Please write an answer to the prompt.",
		}, "", nil
	}

	if g.ai != nil {
		vars := map[string]any{
			"Prompt":     body.Prompt,
			"Submission": trimmed,
		}
		if body.Rubric != "" {
			vars["Rubric"] = body.Rubric
		}
		if body.MinWords > 0 {
			vars["MinWords"] = body.MinWords
		}

		var out aiGradeV2Output
		resp, err := ai.CompleteJSONWithResponse(ctx, g.ai, ai.Request{
			Task: ai.TaskGradeWriting,
			Vars: vars,
		}, &out)
		if err != nil {
			return aiGradeV2Output{}, "", fmt.Errorf("ai grade writing: %w", err)
		}

		if out.FeedbackEn == "" && out.Feedback != "" {
			out.FeedbackEn = out.Feedback
		}
		if out.Feedback == "" && out.FeedbackEn != "" {
			out.Feedback = out.FeedbackEn
		}

		if out.OverallBand == 0 && out.Score > 0 {
			out.OverallBand = float64(out.Score) / 10.0
		}

		return out, resp.Model, nil
	}

	// Fallback heuristic when AI is unavailable or offline
	if body.MinWords > 0 && words < body.MinWords {
		score := (words * 50) / body.MinWords
		short := fmt.Sprintf(
			"Your response is %d words, but the prompt asks for at least %d words.",
			words, body.MinWords,
		)
		return aiGradeV2Output{
			OverallBand: float64(score) / 10.0,
			Score:       score,
			Correct:     false,
			FeedbackEn:  short,
			Feedback:    short,
		}, "", nil
	}

	return aiGradeV2Output{
		OverallBand: 6.5,
		Score:       75,
		Correct:     true,
		FeedbackEn:  "Your writing has been received and reviewed.",
		Feedback:    "Your writing has been received and reviewed.",
	}, "", nil
}

func buildResult(
	contentVersionID uuid.UUID,
	score int,
	correct bool,
	feedback string,
	body writingPromptBody,
	explanation *learningcontract.AnswerExplanation,
) learningcontract.GradeResult {
	initialGrade := gradeAgain
	if correct {
		initialGrade = gradeGood
	}

	var items []learningcontract.ReviewItem
	if contentVersionID != uuid.Nil {
		items = []learningcontract.ReviewItem{
			{
				ContentVersionID: contentVersionID,
				Skill:            skillWriting,
				InitialGrade:     initialGrade,
			},
		}
	}

	sample := body.SampleAnswer
	if sample == "" {
		sample = body.CorrectAnswer
	}

	return learningcontract.GradeResult{
		Score:         score,
		MaxScore:      maxWritingScore,
		Correct:       correct,
		Feedback:      feedback,
		CorrectAnswer: sample,
		Async:         false,
		ReviewItems:   items,
		Explanation:   explanation,
	}
}

// SpendsMoney implements learningcontract.MeteredGrader. Writing grading invokes
// metered AI model calls, so unauthenticated visitors may not run it via preview.
func (g *Grader) SpendsMoney() bool {
	return true
}

var _ contract.Grader = (*Grader)(nil)
var _ learningcontract.ExerciseGrader = (*Grader)(nil)
var _ learningcontract.MeteredGrader = (*Grader)(nil)
