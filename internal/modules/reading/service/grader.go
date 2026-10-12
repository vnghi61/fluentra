// Package service implements the business logic and graders for the reading module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

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

	// Maximum valid reading duration (1 hour in milliseconds)
	maxReadingDurationMs = 3600 * 1000
)

type (
	readingOption   = contentcontract.QuestionOption
	readingQuestion = contentcontract.QuestionItem
)

type readingQuizBody struct {
	PassageTitle    string   `json:"passage_title,omitempty"`
	Passage         string   `json:"passage"`
	Sentence        string   `json:"sentence,omitempty"`
	Prompt          string   `json:"prompt,omitempty"`
	CorrectAnswer   string   `json:"correct_answer,omitempty"`
	CorrectOptionID string   `json:"correct_option_id,omitempty"`
	Key             string   `json:"key,omitempty"`
	Acceptable      []string `json:"acceptable,omitempty"`
	// MaxWords is a typed completion's word limit ("NO MORE THAN TWO WORDS").
	// Zero means no limit. An answer over it is wrong even if the words match
	// (WO 22 D22-25).
	MaxWords    int                                 `json:"max_words,omitempty"`
	Explanation *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
	Questions   []contentcontract.QuestionItem      `json:"questions,omitempty"`
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

	resp := contentcontract.ParseComprehensionResponse(req.Response)
	wpm := calculateWPM(body.Passage, resp.ReadingMs)

	// Multi-question comprehension set
	if len(body.Questions) > 0 {
		return gradeQuestionSet(req.ContentVersionID, body, resp, wpm), nil
	}

	// Legacy single-question format (BR-CONTENT-01)
	score, correct := gradeSingle(resp, body)
	return buildSingleResult(req.ContentVersionID, score, correct, body, wpm), nil
}

func calculateWPM(passage string, readingMs *int64) int {
	if readingMs == nil {
		return 0
	}
	ms := *readingMs
	if ms <= 0 || ms > maxReadingDurationMs {
		return 0
	}
	words := len(strings.Fields(passage))
	if words == 0 {
		return 0
	}
	minutes := float64(ms) / 60000.0
	return int(math.Round(float64(words) / minutes))
}

func gradeQuestionSet(
	contentVersionID uuid.UUID,
	body readingQuizBody,
	resp contentcontract.ComprehensionResponse,
	wpm int,
) learningcontract.GradeResult {
	questions := contentcontract.WithGroupWordLimit(body.Questions, body.MaxWords)
	qResult := contentcontract.GradeQuestionSet(questions, resp.Answers, maxReadingScore)

	itemResults := make([]learningcontract.ItemResult, len(qResult.ItemResults))
	for i, r := range qResult.ItemResults {
		itemResults[i] = learningcontract.ItemResult{
			ID:            r.ID,
			Correct:       r.Correct,
			CorrectAnswer: r.CorrectAnswer,
			Explanation:   learningcontract.ExplanationFrom(r.Explanation),
		}
	}

	initialGrade := gradeAgain
	if qResult.AllCorrect {
		initialGrade = gradeGood
	}

	feedback := fmt.Sprintf(
		"You scored %d%% (%d/%d questions correct).", qResult.Score, qResult.CorrectCount, qResult.TotalCount,
	)
	if qResult.AllCorrect {
		feedback = "All answers correct! Excellent reading comprehension."
	}
	if wpm > 0 {
		feedback = fmt.Sprintf("%s Reading speed: %d WPM.", feedback, wpm)
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
		Score:       qResult.Score,
		MaxScore:    maxReadingScore,
		Correct:     qResult.AllCorrect,
		Feedback:    feedback,
		Async:       false,
		ReviewItems: items,
		ItemResults: itemResults,
		Explanation: body.Explanation,
	}
}

func gradeSingle(resp contentcontract.ComprehensionResponse, body readingQuizBody) (int, bool) {
	submitted := contentcontract.SingleSubmittedAnswer(resp)
	if exceedsWordLimit(submitted, body.MaxWords) {
		return 0, false
	}
	key := body.CorrectOptionID
	if key == "" {
		key = body.Key
	}
	if contentcontract.MatchesSingleAnswer(submitted, key, body.CorrectAnswer, body.Acceptable) {
		return maxReadingScore, true
	}
	return 0, false
}

// exceedsWordLimit reports whether a typed answer breaks the item's word limit
// ("NO MORE THAN TWO WORDS"). Zero means no limit.
func exceedsWordLimit(submitted string, maxWords int) bool {
	if maxWords <= 0 || strings.TrimSpace(submitted) == "" {
		return false
	}
	return len(strings.Fields(submitted)) > maxWords
}

func buildSingleResult(
	contentVersionID uuid.UUID,
	score int,
	correct bool,
	body readingQuizBody,
	wpm int,
) learningcontract.GradeResult {
	initialGrade := gradeAgain
	feedback := "Incorrect. Review the passage carefully to find the answer."
	if correct {
		initialGrade = gradeGood
		feedback = "Correct! Great reading comprehension."
	}
	if wpm > 0 {
		feedback = fmt.Sprintf("%s Reading speed: %d WPM.", feedback, wpm)
	}

	answer := body.CorrectOptionID
	if answer == "" {
		answer = body.Key
	}
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
