// Package service implements the business logic and graders for the reading module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/reading/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// ContentReader narrows contentcontract.Reader to what Grader needs.
type ContentReader interface {
	GetVersion(ctx context.Context, id uuid.UUID) (*contentcontract.Version, error)
}

const (
	gradeAgain = "again"
	gradeGood  = "good"

	skillReading    = "reading"
	maxReadingScore = 100
)

type readingQuizBody struct {
	PassageTitle    string                              `json:"passage_title,omitempty"`
	Passage         string                              `json:"passage"`
	Prompt          string                              `json:"prompt"`
	CorrectAnswer   string                              `json:"correct_answer,omitempty"`
	CorrectOptionID string                              `json:"correct_option_id,omitempty"`
	Acceptable      []string                            `json:"acceptable,omitempty"`
	Explanation     *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

type readingResponse struct {
	SelectedOptionID string `json:"selected_option_id,omitempty"`
	TextAnswer       string `json:"text_answer,omitempty"`
	Answer           string `json:"answer,omitempty"`
}

// Grader implements learningcontract.ExerciseGrader and contract.Grader.
type Grader struct {
	content ContentReader
}

// NewGrader constructs a Grader.
func NewGrader(content ContentReader) *Grader {
	return &Grader{content: content}
}

func (g *Grader) loadBody(ctx context.Context, versionID uuid.UUID) (readingQuizBody, error) {
	if g.content == nil {
		return readingQuizBody{}, apperr.New(
			apperr.Internal,
			"CONTENT_READER_REQUIRED",
			"reading grader requires a content reader",
		)
	}

	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return readingQuizBody{}, fmt.Errorf("load reading content version: %w", err)
	}
	if version == nil {
		return readingQuizBody{}, apperr.New(
			apperr.NotFound,
			"CONTENT_VERSION_NOT_FOUND",
			"reading content version not found",
		)
	}

	var body readingQuizBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return readingQuizBody{}, fmt.Errorf("unmarshal reading content body: %w", err)
		}
	}
	return body, nil
}

// Grade implements learningcontract.ExerciseGrader for reading comprehension activities.
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

func grade(response json.RawMessage, body readingQuizBody) (int, bool) {
	submitted := submittedAnswer(response)
	if matches(submitted, body) {
		return maxReadingScore, true
	}
	return 0, false
}

func submittedAnswer(response json.RawMessage) string {
	if len(response) == 0 {
		return ""
	}

	var resp readingResponse
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

func matches(submitted string, body readingQuizBody) bool {
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
	contentVersionID uuid.UUID, score int, correct bool, body readingQuizBody,
) learningcontract.GradeResult {
	initialGrade := gradeAgain
	feedback := "Incorrect. Review the passage carefully to find the answer."
	if correct {
		initialGrade = gradeGood
		feedback = "Correct! Great reading comprehension."
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
				Skill:            skillReading,
				InitialGrade:     initialGrade,
			},
		}
	}

	return learningcontract.GradeResult{
		Score:         score,
		MaxScore:      maxReadingScore,
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
