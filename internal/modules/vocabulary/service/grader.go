// Package service implements the exercise grader for vocabulary activities.
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// SenseResolver finds the dictionary entry behind a word.
//
// Declared here, and narrow, because the grader needs exactly one thing from
// the repository and a grader that could reach the whole of it would eventually
// do so.
type SenseResolver interface {
	GetSenseContentVersionByLemma(ctx context.Context, lemma string) (*uuid.UUID, error)
}

// Grader evaluates vocabulary exercises such as quizzes, cloze, and recall tests.
//
// It is the first real learning.ExerciseGrader. A Phase 3 agent writing the
// grammar grader should be able to copy this file, change `matches`, and touch
// nothing else — everything module-specific is confined to the two body types
// and that one function.
type Grader struct {
	content ContentReader
	senses  SenseResolver
}

// NewGrader constructs a vocabulary exercise grader.
//
// `senses` may be nil, and then review cards fall back to the activity's own
// content version — the behaviour before this resolver existed. Nothing breaks
// without it; the cards are simply the poorer ones.
func NewGrader(content ContentReader, senses SenseResolver) *Grader {
	return &Grader{
		content: content,
		senses:  senses,
	}
}

// vocabularyQuizResponse is the learner's submission. The three field names are
// the shapes the P10.4 flashcard can send: a typed recall, a chosen option, or
// free text.
type vocabularyQuizResponse struct {
	Answer           string `json:"answer"`
	SelectedOption   string `json:"selected_option"`
	SelectedOptionID string `json:"selected_option_id"`
	Text             string `json:"text"`
	TextAnswer       string `json:"text_answer"`
}

// vocabularyQuizBody is the authored side, stored in the content version body.
type vocabularyQuizBody struct {
	CorrectAnswer string   `json:"correct_answer"`
	Acceptable    []string `json:"acceptable"`
	Prompt        string   `json:"prompt"`
	// Authored by the multiple-choice kinds. The renderer marks the right row
	// by id, so this is what it needs back after grading — the learner-facing
	// body no longer carries it.
	CorrectOptionID string `json:"correct_option_id"`
}

// Grade implements learningcontract.ExerciseGrader for vocabulary activity kinds.
func (g *Grader) Grade(
	ctx context.Context, req learningcontract.GradeRequest,
) (learningcontract.GradeResult, error) {
	body, err := g.loadBody(ctx, req.ContentVersionID)
	if err != nil {
		return learningcontract.GradeResult{}, err
	}

	correct := matches(submittedAnswer(req.Response), body)
	return buildResult(g.reviewVersion(ctx, req.ContentVersionID, body), correct, body), nil
}

// reviewVersion decides what the learner will be asked to remember.
//
// Not the activity. An exercise is one way of asking about a word; the thing
// worth reviewing in three days is the word. Scheduling the exercise instead is
// how every card a learner earned ended up pointing at a body holding a prompt
// and an answer key and nothing a flashcard can render — the review screen
// showed "This card has no content yet" for all of them, because it wants the
// dictionary entry and was handed a quiz.
//
// The lemma is the authored answer, so no new field is needed on the activity.
// Any failure falls back to the activity's own version: a review card pointing
// at something imperfect is better than a correct answer that schedules nothing,
// and the learner has already earned the card by the time this runs.
func (g *Grader) reviewVersion(
	ctx context.Context, activityVersion uuid.UUID, body vocabularyQuizBody,
) uuid.UUID {
	if g.senses == nil {
		return activityVersion
	}

	// `correct_answer` first, then the acceptable list.
	//
	// A multiple-choice activity's answer key is an option id — "opt_habit" —
	// because that is what the learner submits, and no dictionary has an entry
	// for it. The lemma is in `acceptable` beside it, put there so typing the
	// word also counts. Trying both means the resolver works whichever way the
	// answer key happens to be authored, without a content migration and
	// without the grader needing to know one activity kind from another.
	for _, candidate := range append([]string{body.CorrectAnswer}, body.Acceptable...) {
		lemma := normalise(candidate)
		if lemma == "" {
			continue
		}
		senseVersion, err := g.senses.GetSenseContentVersionByLemma(ctx, lemma)
		if err != nil || senseVersion == nil || *senseVersion == uuid.Nil {
			continue
		}
		return *senseVersion
	}
	return activityVersion
}

// loadBody reads the authored answer key for this content version.
//
// Every failure here is an error rather than a default verdict. An activity
// whose body cannot be read is an authoring or deployment fault, and the one
// thing a grader must never do about it is guess: marking the learner correct
// would inflate their progress and schedule a review card for a word they may
// not know, silently and for everyone the broken content reaches.
func (g *Grader) loadBody(ctx context.Context, versionID uuid.UUID) (vocabularyQuizBody, error) {
	notGradable := func(reason string) error {
		return apperr.New(
			apperr.PreconditionFailed,
			"VOCABULARY_ACTIVITY_NOT_GRADABLE",
			"This vocabulary activity cannot be graded yet.",
		).WithInternal(reason)
	}

	if g.content == nil || versionID == uuid.Nil {
		return vocabularyQuizBody{}, notGradable("no content reader or no content version id")
	}

	version, err := g.content.GetVersion(ctx, versionID)
	if err != nil {
		return vocabularyQuizBody{}, err
	}
	if version == nil || len(version.Body) == 0 {
		return vocabularyQuizBody{}, notGradable("content version " + versionID.String() + " has no body")
	}

	var body vocabularyQuizBody
	if err := json.Unmarshal(version.Body, &body); err != nil {
		return vocabularyQuizBody{}, notGradable("content version " + versionID.String() + " body is not a vocabulary quiz")
	}
	if strings.TrimSpace(body.CorrectAnswer) == "" {
		return vocabularyQuizBody{}, notGradable("content version " + versionID.String() + " declares no correct_answer")
	}
	return body, nil
}

// submittedAnswer normalises whichever of the response shapes arrived.
func submittedAnswer(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var resp vocabularyQuizResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ""
	}
	candidates := []string{
		resp.Answer, resp.SelectedOption, resp.SelectedOptionID, resp.Text, resp.TextAnswer,
	}
	for _, candidate := range candidates {
		if candidate != "" {
			return normalise(candidate)
		}
	}
	return ""
}

// matches applies BR-VOCABULARY-05: the authored answer, or any spelling the
// author listed as acceptable, counts.
func matches(answer string, body vocabularyQuizBody) bool {
	if answer == "" {
		return false
	}
	if answer == normalise(body.CorrectAnswer) {
		return true
	}
	for _, acceptable := range body.Acceptable {
		if answer == normalise(acceptable) {
			return true
		}
	}
	return false
}

func normalise(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// buildResult turns a verdict into the GradeResult and the single ReviewItem
// that puts this word into the learner's spaced repetition queue.
func buildResult(
	versionID uuid.UUID, correct bool, body vocabularyQuizBody,
) learningcontract.GradeResult {
	grade := gradeAgain
	score := 0
	feedback := "Incorrect answer. Review this word again."
	if correct {
		grade = gradeGood
		score = maxVocabularyScore
		feedback = "Correct! Well done."
	}

	// The option id where there is one, because that is what marks the right
	// row; the answer text otherwise.
	answer := body.CorrectOptionID
	if answer == "" {
		answer = body.CorrectAnswer
	}

	return learningcontract.GradeResult{
		Score:         score,
		MaxScore:      maxVocabularyScore,
		Correct:       correct,
		Feedback:      feedback,
		CorrectAnswer: answer,
		Async:         false,
		ReviewItems: []learningcontract.ReviewItem{
			{
				ContentVersionID: versionID,
				Skill:            skillVocabulary,
				InitialGrade:     grade,
			},
		},
	}
}

// The grades this grader seeds a new card with. srs owns the full four-value
// enum; a first-sight verdict is binary, so only these two are reachable here.
const (
	gradeAgain = "again"
	gradeGood  = "good"

	skillVocabulary    = "vocabulary"
	maxVocabularyScore = 100
)

// Compile-time check that Grader satisfies contract.Grader and learningcontract.ExerciseGrader.
var _ contract.Grader = (*Grader)(nil)
var _ learningcontract.ExerciseGrader = (*Grader)(nil)
