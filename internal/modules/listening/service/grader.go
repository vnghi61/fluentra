package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

const (
	gradeAgain = "again"
	gradeGood  = "good"

	skillListening    = "listening"
	maxListeningScore = 100
)

// Grader evaluates listening comprehension activities.
type Grader struct {
	content ContentReader
}

// NewGrader constructs a listening Grader.
func NewGrader(content ContentReader) *Grader {
	return &Grader{content: content}
}

func (g *Grader) loadBody(ctx context.Context, versionID uuid.UUID) (listeningBody, error) {
	if g.content == nil {
		return listeningBody{}, apperr.New(
			apperr.Internal,
			"CONTENT_READER_REQUIRED",
			"listening grader requires a content reader",
		)
	}

	if temp, ok := contentcontract.TempVersionFromContext(ctx, versionID); ok && temp != nil {
		var body listeningBody
		if len(temp.Body) > 0 {
			if err := json.Unmarshal(temp.Body, &body); err != nil {
				return listeningBody{}, fmt.Errorf("unmarshal temp listening content body: %w", err)
			}
		}
		return body, nil
	}

	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return listeningBody{}, fmt.Errorf("load listening content version: %w", err)
	}
	if version == nil {
		return listeningBody{}, apperr.New(
			apperr.NotFound,
			"CONTENT_VERSION_NOT_FOUND",
			"listening content version not found",
		)
	}

	var body listeningBody
	if len(version.Body) > 0 {
		if err := json.Unmarshal(version.Body, &body); err != nil {
			return listeningBody{}, fmt.Errorf("unmarshal listening content body: %w", err)
		}
	}
	return body, nil
}

// Grade implements learningcontract.ExerciseGrader for listening comprehension activities.
func (g *Grader) Grade(
	ctx context.Context, req learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	body, err := g.loadBody(ctx, req.ContentVersionID)
	if err != nil {
		return learningcontract.GradeResult{}, err
	}

	resp := contentcontract.ParseComprehensionResponse(req.Response)

	// Multi-question comprehension set
	if len(body.Questions) > 0 {
		return gradeListeningQuestionSet(req.ContentVersionID, body, resp), nil
	}

	// Single-question fallback
	score, correct := gradeListeningSingle(resp, body)
	return buildListeningSingleResult(req.ContentVersionID, score, correct, body), nil
}

func gradeListeningQuestionSet(
	contentVersionID uuid.UUID,
	body listeningBody,
	resp contentcontract.ComprehensionResponse,
) learningcontract.GradeResult {
	qResult := contentcontract.GradeQuestionSet(body.Questions, resp.Answers, maxListeningScore)

	itemResults := make([]learningcontract.ItemResult, len(qResult.ItemResults))
	for i, r := range qResult.ItemResults {
		itemResults[i] = learningcontract.ItemResult{
			ID:            r.ID,
			Correct:       r.Correct,
			CorrectAnswer: r.CorrectAnswer,
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
		feedback = "All answers correct! Excellent listening comprehension."
	}

	var items []learningcontract.ReviewItem
	if contentVersionID != uuid.Nil {
		items = []learningcontract.ReviewItem{
			{
				ContentVersionID: contentVersionID,
				Skill:            skillListening,
				InitialGrade:     initialGrade,
			},
		}
	}

	return learningcontract.GradeResult{
		Score:       qResult.Score,
		MaxScore:    maxListeningScore,
		Correct:     qResult.AllCorrect,
		Feedback:    feedback,
		Async:       false,
		ReviewItems: items,
		ItemResults: itemResults,
		Explanation: body.Explanation,
	}
}

func gradeListeningSingle(resp contentcontract.ComprehensionResponse, body listeningBody) (int, bool) {
	submitted := contentcontract.SingleSubmittedAnswer(resp)
	if contentcontract.MatchesSingleAnswer(submitted, body.CorrectOptionID, body.CorrectAnswer, body.Acceptable) {
		return maxListeningScore, true
	}
	return 0, false
}

func buildListeningSingleResult(
	contentVersionID uuid.UUID,
	score int,
	correct bool,
	body listeningBody,
) learningcontract.GradeResult {
	initialGrade := gradeAgain
	feedback := "Incorrect. Listen to the audio again to verify your answer."
	if correct {
		initialGrade = gradeGood
		feedback = "Correct! Great listening."
	}

	var items []learningcontract.ReviewItem
	if contentVersionID != uuid.Nil {
		items = []learningcontract.ReviewItem{
			{
				ContentVersionID: contentVersionID,
				Skill:            skillListening,
				InitialGrade:     initialGrade,
			},
		}
	}

	return learningcontract.GradeResult{
		Score:         score,
		MaxScore:      maxListeningScore,
		Correct:       correct,
		Feedback:      feedback,
		CorrectAnswer: body.CorrectAnswer,
		Async:         false,
		ReviewItems:   items,
		Explanation:   body.Explanation,
	}
}
