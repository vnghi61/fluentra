// Package service implements the business logic and graders for the grammar module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/grammar/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// ContentReader narrows contentcontract.Reader to what Grader needs.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

const (
	gradeAgain = "again"
	gradeGood  = "good"

	skillGrammar    = "grammar"
	maxGrammarScore = 100
)

type grammarQuizBody struct {
	Prompt          string                              `json:"prompt"`
	CorrectAnswer   string                              `json:"correct_answer"`
	CorrectOptionID string                              `json:"correct_option_id"`
	Acceptable      []string                            `json:"acceptable"`
	Explanation     *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

type grammarResponse struct {
	SelectedOptionID string `json:"selected_option_id"`
	TextAnswer       string `json:"text_answer"`
	Answer           string `json:"answer"`
}

// Grader implements learningcontract.ExerciseGrader and contract.Grader.
type Grader struct {
	content ContentReader
}

// NewGrader constructs a Grader.
func NewGrader(content ContentReader) *Grader {
	return &Grader{content: content}
}

func (g *Grader) loadBody(ctx context.Context, versionID uuid.UUID) (grammarQuizBody, error) {
	if g.content == nil {
		return grammarQuizBody{}, apperr.New(
			apperr.Internal,
			"CONTENT_READER_REQUIRED",
			"grammar grader requires a content reader",
		)
	}

	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return grammarQuizBody{}, fmt.Errorf("load grammar content version: %w", err)
	}
	if version == nil {
		return grammarQuizBody{}, apperr.New(
			apperr.NotFound,
			"CONTENT_VERSION_NOT_FOUND",
			"grammar content version not found",
		)
	}

	var body grammarQuizBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return grammarQuizBody{}, fmt.Errorf("unmarshal grammar content body: %w", err)
		}
	}
	return body, nil
}

// Grade implements learningcontract.ExerciseGrader for grammar activity kinds.
func (g *Grader) Grade(
	ctx context.Context, req learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	body, err := g.loadBody(ctx, req.ContentVersionID)
	if err != nil {
		return learningcontract.GradeResult{}, err
	}

	score, correct := grade(req.Response, body)
	return buildResult(req.ContentVersionID, score, correct, body), nil
}

func grade(response json.RawMessage, body grammarQuizBody) (int, bool) {
	submitted := submittedAnswer(response)
	if matches(submitted, body) {
		return maxGrammarScore, true
	}
	return 0, false
}

func submittedAnswer(response json.RawMessage) string {
	if len(response) == 0 {
		return ""
	}

	var resp grammarResponse
	if err := json.Unmarshal(response, &resp); err == nil {
		if resp.SelectedOptionID != "" {
			return resp.SelectedOptionID
		}
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

func matches(submitted string, body grammarQuizBody) bool {
	if submitted == "" {
		return false
	}

	normSubmitted := normalise(submitted)

	if body.CorrectOptionID != "" && normSubmitted == normalise(body.CorrectOptionID) {
		return true
	}

	if body.CorrectAnswer != "" &&
		(normSubmitted == normalise(body.CorrectAnswer) || sentence(submitted) == sentence(body.CorrectAnswer)) {
		return true
	}

	for _, alt := range body.Acceptable {
		if normSubmitted == normalise(alt) || sentence(submitted) == sentence(alt) {
			return true
		}
	}

	return false
}

func normalise(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func sentence(s string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return strings.Join(fields, " ")
}

func buildResult(
	contentVersionID uuid.UUID, score int, correct bool, body grammarQuizBody,
) learningcontract.GradeResult {
	initialGrade := gradeAgain
	feedback := "Incorrect grammar form. Review this rule again."
	if correct {
		initialGrade = gradeGood
		feedback = "Correct! Well done."
	}

	answer := body.CorrectOptionID
	if answer == "" {
		answer = body.CorrectAnswer
	}

	var items []learningcontract.ReviewItem
	if contentVersionID != uuid.Nil {
		items = []learningcontract.ReviewItem{
			{
				ContentVersionID: contentVersionID,
				Skill:            skillGrammar,
				InitialGrade:     initialGrade,
			},
		}
	}

	return learningcontract.GradeResult{
		Score:         score,
		MaxScore:      maxGrammarScore,
		Correct:       correct,
		Feedback:      feedback,
		CorrectAnswer: answer,
		Async:         false,
		ReviewItems:   items,
		Explanation:   body.Explanation,
	}
}

var _ contract.Grader = (*Grader)(nil)
var _ learningcontract.ExerciseGrader = (*Grader)(nil)
