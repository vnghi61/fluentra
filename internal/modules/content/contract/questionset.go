package contract

import (
	"encoding/json"
	"math"
	"strings"
	"unicode"
)

// QuestionOption is a choice inside a multiple-choice question item.
type QuestionOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// QuestionItem is a single question inside a comprehension question set (reading or listening).
type QuestionItem struct {
	ID              string           `json:"id"`
	Type            string           `json:"type"` // multiple_choice, true_false_not_given, gap_fill
	Prompt          string           `json:"prompt"`
	Options         []QuestionOption `json:"options,omitempty"`
	Answer          string           `json:"answer,omitempty"`
	CorrectAnswer   string           `json:"correct_answer,omitempty"`
	CorrectOptionID string           `json:"correct_option_id,omitempty"`
	Key             string           `json:"key,omitempty"`
	Acceptable      []string         `json:"acceptable,omitempty"`
	// Explanation is authored per question, in either spelling; graders pass it
	// back with the verdict. Raw, because its type belongs to learning.
	Explanation json.RawMessage `json:"explanation,omitempty"`
}

// QuestionItemResult represents the graded outcome of one question in a question set.
type QuestionItemResult struct {
	ID            string
	Correct       bool
	CorrectAnswer *string
	Explanation   json.RawMessage
}

// QuestionSetGradeResult is the aggregated score and breakdown for a question set.
type QuestionSetGradeResult struct {
	Score        int
	MaxScore     int
	CorrectCount int
	TotalCount   int
	AllCorrect   bool
	ItemResults  []QuestionItemResult
}

// AnswerItem represents an element in a submitted items array.
type AnswerItem struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

// ComprehensionResponse is the parsed submitted answer payload for comprehension exercises.
type ComprehensionResponse struct {
	SelectedOptionID string            `json:"selected_option_id,omitempty"`
	TextAnswer       string            `json:"text_answer,omitempty"`
	Answer           string            `json:"answer,omitempty"`
	Answers          map[string]string `json:"answers,omitempty"`
	Items            []AnswerItem      `json:"items,omitempty"`
	ReadingMs        *int64            `json:"reading_ms,omitempty"`
}

// ParseComprehensionResponse unmarshals the raw response into a structured ComprehensionResponse.
func ParseComprehensionResponse(raw json.RawMessage) ComprehensionResponse {
	if len(raw) == 0 {
		return ComprehensionResponse{}
	}

	var resp ComprehensionResponse
	if err := json.Unmarshal(raw, &resp); err == nil {
		if resp.Answers == nil && len(resp.Items) > 0 {
			resp.Answers = make(map[string]string, len(resp.Items))
			for _, item := range resp.Items {
				resp.Answers[item.ID] = item.Answer
			}
		}
		if resp.Answers == nil && resp.Answer == "" && resp.SelectedOptionID == "" && resp.TextAnswer == "" {
			var directMap map[string]string
			if err := json.Unmarshal(raw, &directMap); err == nil && len(directMap) > 0 {
				resp.Answers = directMap
			}
		}
		return resp
	}

	// Fallback for raw string answer
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return ComprehensionResponse{Answer: str}
	}

	return ComprehensionResponse{}
}

// GradeQuestionSet grades an array of questions against a submitted answers map.
func GradeQuestionSet(questions []QuestionItem, answers map[string]string, maxScore int) QuestionSetGradeResult {
	total := len(questions)
	if total == 0 {
		return QuestionSetGradeResult{MaxScore: maxScore}
	}

	results := make([]QuestionItemResult, total)
	correctCount := 0

	for i, q := range questions {
		submitted := ""
		if answers != nil {
			submitted = answers[q.ID]
		}
		isCorrect := MatchQuestion(submitted, q)
		if isCorrect {
			correctCount++
		}

		ans := QuestionCanonicalAnswer(q)
		var ca *string
		if ans != "" {
			ca = &ans
		}

		results[i] = QuestionItemResult{
			ID:            q.ID,
			Correct:       isCorrect,
			CorrectAnswer: ca,
			Explanation:   q.Explanation,
		}
	}

	score := int(math.Round(float64(correctCount) / float64(total) * float64(maxScore)))
	allCorrect := correctCount == total

	return QuestionSetGradeResult{
		Score:        score,
		MaxScore:     maxScore,
		CorrectCount: correctCount,
		TotalCount:   total,
		AllCorrect:   allCorrect,
		ItemResults:  results,
	}
}

// SingleSubmittedAnswer extracts the single string answer from a comprehension response.
func SingleSubmittedAnswer(resp ComprehensionResponse) string {
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

// MatchesSingleAnswer checks if submitted string matches single-answer target parameters.
func MatchesSingleAnswer(submitted, correctOptionID, correctAnswer string, acceptable []string) bool {
	if submitted == "" {
		return false
	}

	normSubmitted := NormaliseText(submitted)

	if correctOptionID != "" && normSubmitted == NormaliseText(correctOptionID) {
		return true
	}

	if correctAnswer != "" &&
		(normSubmitted == NormaliseText(correctAnswer) || SentenceWords(submitted) == SentenceWords(correctAnswer)) {
		return true
	}

	for _, alt := range acceptable {
		if normSubmitted == NormaliseText(alt) || SentenceWords(submitted) == SentenceWords(alt) {
			return true
		}
	}

	return false
}

// QuestionCanonicalAnswer determines the displayable canonical answer for a question.
func QuestionCanonicalAnswer(q QuestionItem) string {
	if q.CorrectOptionID != "" {
		return q.CorrectOptionID
	}
	if q.Key != "" {
		return q.Key
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

// MatchQuestion reports whether the submitted answer satisfies the question's criteria.
func MatchQuestion(submitted string, q QuestionItem) bool {
	if submitted == "" {
		return false
	}

	normSubmitted := NormaliseText(submitted)

	if q.CorrectOptionID != "" && normSubmitted == NormaliseText(q.CorrectOptionID) {
		return true
	}
	if q.Key != "" && normSubmitted == NormaliseText(q.Key) {
		return true
	}
	if q.Answer != "" &&
		(normSubmitted == NormaliseText(q.Answer) || SentenceWords(submitted) == SentenceWords(q.Answer)) {
		return true
	}
	if q.CorrectAnswer != "" &&
		(normSubmitted == NormaliseText(q.CorrectAnswer) || SentenceWords(submitted) == SentenceWords(q.CorrectAnswer)) {
		return true
	}
	for _, alt := range q.Acceptable {
		if normSubmitted == NormaliseText(alt) || SentenceWords(submitted) == SentenceWords(alt) {
			return true
		}
	}
	return false
}

// NormaliseText trims whitespace and converts string to lowercase.
func NormaliseText(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// SentenceWords normalises words for case- and punctuation-insensitive sentence comparison.
func SentenceWords(s string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return strings.Join(fields, " ")
}
