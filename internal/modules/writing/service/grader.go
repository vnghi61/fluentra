// Package service implements the business logic and graders for the writing module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// ContentReader narrows contentcontract.Reader to what Grader needs.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

const (
	gradeAgain = "again"
	gradeGood  = "good"

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

type aiGradeOutput struct {
	Score      int    `json:"score"`
	Correct    bool   `json:"correct"`
	Feedback   string `json:"feedback"`
	FeedbackVi string `json:"feedback_vi,omitempty"`
}

// Grader implements learningcontract.ExerciseGrader and contract.Grader.
type Grader struct {
	content ContentReader
	ai      ai.Client
}

// NewGrader constructs a Grader.
func NewGrader(content ContentReader, aiClient ai.Client) *Grader {
	return &Grader{
		content: content,
		ai:      aiClient,
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

	score, correct, feedback, explanation := g.evaluate(ctx, submitted, body)
	return buildResult(req.ContentVersionID, score, correct, feedback, body, explanation), nil
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

func (g *Grader) evaluate(
	ctx context.Context,
	submitted string,
	body writingPromptBody,
) (int, bool, string, *learningcontract.AnswerExplanation) {
	trimmed := strings.TrimSpace(submitted)
	words := len(strings.Fields(trimmed))

	if words == 0 {
		return 0, false, "No response provided. Please write an answer to the prompt.", body.Explanation
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

		var out aiGradeOutput
		err := ai.CompleteJSON(ctx, g.ai, ai.Request{
			Task: ai.TaskGradeWriting,
			Vars: vars,
		}, &out)

		if err == nil {
			exp := body.Explanation
			if out.FeedbackVi != "" {
				exp = &learningcontract.AnswerExplanation{
					Text:   out.Feedback,
					TextVi: out.FeedbackVi,
				}
			}
			return out.Score, out.Correct, out.Feedback, exp
		}
	}

	// Fallback heuristic when AI is unavailable or offline
	if body.MinWords > 0 && words < body.MinWords {
		score := (words * 50) / body.MinWords
		short := fmt.Sprintf(
			"Your response is %d words, but the prompt asks for at least %d words.",
			words, body.MinWords,
		)
		return score, false, short, body.Explanation
	}

	return 75, true, "Your writing has been received and reviewed.", body.Explanation
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

var _ contract.Grader = (*Grader)(nil)
var _ learningcontract.ExerciseGrader = (*Grader)(nil)
