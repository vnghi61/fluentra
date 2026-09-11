package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

	// Maximum valid reading duration (1 hour in milliseconds)
	maxReadingDurationMs = 3600 * 1000
)

type readingOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type readingQuestion struct {
	ID              string                              `json:"id"`
	Type            string                              `json:"type"` // multiple_choice, true_false_not_given, gap_fill
	Prompt          string                              `json:"prompt"`
	Options         []readingOption                     `json:"options,omitempty"`
	Answer          string                              `json:"answer,omitempty"`
	CorrectAnswer   string                              `json:"correct_answer,omitempty"`
	CorrectOptionID string                              `json:"correct_option_id,omitempty"`
	Acceptable      []string                            `json:"acceptable,omitempty"`
	Explanation     *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
}

type readingQuizBody struct {
	PassageTitle    string                              `json:"passage_title,omitempty"`
	Passage         string                              `json:"passage"`
	Prompt          string                              `json:"prompt,omitempty"`
	CorrectAnswer   string                              `json:"correct_answer,omitempty"`
	CorrectOptionID string                              `json:"correct_option_id,omitempty"`
	Acceptable      []string                            `json:"acceptable,omitempty"`
	Explanation     *learningcontract.AnswerExplanation `json:"explanation,omitempty"`
	Questions       []readingQuestion                   `json:"questions,omitempty"`
}

type readingAnswerItem struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

type readingResponse struct {
	SelectedOptionID string              `json:"selected_option_id,omitempty"`
	TextAnswer       string              `json:"text_answer,omitempty"`
	Answer           string              `json:"answer,omitempty"`
	Answers          map[string]string   `json:"answers,omitempty"`
	Items            []readingAnswerItem `json:"items,omitempty"`
	ReadingMs        *int64              `json:"reading_ms,omitempty"`
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

	resp := parseReadingResponse(req.Response)
	wpm := calculateWPM(body.Passage, resp.ReadingMs)

	// Multi-question comprehension set
	if len(body.Questions) > 0 {
		return gradeQuestionSet(req.ContentVersionID, body, resp, wpm), nil
	}

	// Legacy single-question format (BR-CONTENT-01)
	score, correct := gradeSingle(resp, body)
	return buildSingleResult(req.ContentVersionID, score, correct, body, wpm), nil
}

func parseReadingResponse(raw json.RawMessage) readingResponse {
	if len(raw) == 0 {
		return readingResponse{}
	}

	var resp readingResponse
	if err := json.Unmarshal(raw, &resp); err == nil {
		if resp.Answers == nil && len(resp.Items) > 0 {
			resp.Answers = make(map[string]string, len(resp.Items))
			for _, item := range resp.Items {
				resp.Answers[item.ID] = item.Answer
			}
		}
		return resp
	}

	// Fallback for raw string answer
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return readingResponse{Answer: str}
	}

	return readingResponse{}
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
	resp readingResponse,
	wpm int,
) learningcontract.GradeResult {
	total := len(body.Questions)
	itemResults := make([]learningcontract.ItemResult, total)
	correctCount := 0

	for i, q := range body.Questions {
		submitted := ""
		if resp.Answers != nil {
			submitted = resp.Answers[q.ID]
		}
		isCorrect := matchQuestion(submitted, q)
		if isCorrect {
			correctCount++
		}

		ans := questionCanonicalAnswer(q)
		var ca *string
		if ans != "" {
			ca = &ans
		}

		itemResults[i] = learningcontract.ItemResult{
			ID:            q.ID,
			Correct:       isCorrect,
			CorrectAnswer: ca,
		}
	}

	score := int(math.Round(float64(correctCount) / float64(total) * float64(maxReadingScore)))
	correct := correctCount == total

	initialGrade := gradeAgain
	if correct {
		initialGrade = gradeGood
	}

	feedback := fmt.Sprintf("You scored %d%% (%d/%d questions correct).", score, correctCount, total)
	if correct {
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
		Score:       score,
		MaxScore:    maxReadingScore,
		Correct:     correct,
		Feedback:    feedback,
		Async:       false,
		ReviewItems: items,
		ItemResults: itemResults,
		Explanation: body.Explanation,
	}
}

func questionCanonicalAnswer(q readingQuestion) string {
	if q.CorrectOptionID != "" {
		return q.CorrectOptionID
	}
	if q.Answer != "" {
		return q.Answer
	}
	if q.CorrectAnswer != "" {
		return q.CorrectAnswer
	}
	if len(q.Acceptable) > 0 {
		return q.Acceptable[0]
	}
	return ""
}

func matchQuestion(submitted string, q readingQuestion) bool {
	if submitted == "" {
		return false
	}

	normSubmitted := normalise(submitted)

	if q.CorrectOptionID != "" && normSubmitted == normalise(q.CorrectOptionID) {
		return true
	}
	if q.Answer != "" && (normSubmitted == normalise(q.Answer) || sentence(submitted) == sentence(q.Answer)) {
		return true
	}
	if q.CorrectAnswer != "" && (normSubmitted == normalise(q.CorrectAnswer) || sentence(submitted) == sentence(q.CorrectAnswer)) {
		return true
	}
	for _, alt := range q.Acceptable {
		if normSubmitted == normalise(alt) || sentence(submitted) == sentence(alt) {
			return true
		}
	}
	return false
}

func gradeSingle(resp readingResponse, body readingQuizBody) (int, bool) {
	submitted := singleSubmittedAnswer(resp)
	if matchesSingle(submitted, body) {
		return maxReadingScore, true
	}
	return 0, false
}

func singleSubmittedAnswer(resp readingResponse) string {
	if resp.SelectedOptionID != "" {
		return resp.SelectedOptionID
	}
	if resp.TextAnswer != "" {
		return resp.TextAnswer
	}
	if resp.Answer != "" {
		return resp.Answer
	}
	return ""
}

func matchesSingle(submitted string, body readingQuizBody) bool {
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
