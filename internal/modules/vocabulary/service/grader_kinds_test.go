package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
)

// The four kinds added beside multiple choice, gap fill and flashcard.
//
// Three of them grade through the path that already existed — a submitted
// string against an authored answer — and are covered here only where they
// stretch it: `vocab_reorder` submits a whole sentence, and a sentence that
// differs from the authored one by a full stop is not a wrong answer about
// vocabulary. `vocab_match` is the one kind with a grading path of its own,
// because its submission is a set of pairs rather than a string, and it is the
// only kind that can be partly right.

// The body and response keys these tests build, and the fixture words. Named
// because a typo in a repeated literal produces a test that passes for the
// wrong reason.
const (
	keyCorrectAnswer = "correct_answer"
	keyCorrectPairs  = "correct_pairs"
	keyWordLemmas    = "word_lemmas"
	keyAcceptable    = "acceptable"
	keyTextAnswer    = "text_answer"
	keyPairs         = "pairs"

	wordHabit   = "habit"
	wordLeisure = "leisure"
	wordJourney = "journey"

	achieveSentence = "She worked hard to achieve her dream."

	optFreeTime = "opt_free_time"
)

func gradeBody(
	t *testing.T, body, response map[string]any, senses service.SenseResolver,
) learningcontract.GradeResult {
	t.Helper()

	versionID := uuid.New()
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)
	responseJSON, err := json.Marshal(response)
	require.NoError(t, err)

	grader := service.NewGrader(&fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: bodyJSON},
		},
	}, senses)

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         responseJSON,
	})
	require.NoError(t, err)
	return result
}

func TestGradedKinds_CoversTheFourNewActivityKinds(t *testing.T) {
	// cmd/api builds both the grader registry and learning's DeclaredKinds from
	// this list, and startup fails on a kind with no grader behind it. A kind
	// seeded but not listed fails the learner's request instead, which is why
	// the seed asserts against the same function.
	assert.Subset(t, contract.GradedKinds(), []string{
		"vocab_listen_type",
		"vocab_match",
		"vocab_reorder",
		"vocab_context_choice",
	})
}

func TestVocabularyGrader_ReorderIgnoresPunctuationAndSpacing(t *testing.T) {
	body := map[string]any{
		keyCorrectAnswer: achieveSentence,
		keyWordLemmas:    []string{"achieve"},
	}

	for name, submitted := range map[string]string{
		"exact":            achieveSentence,
		"no full stop":     "She worked hard to achieve her dream",
		"double spaces":    "She  worked hard  to achieve her dream.",
		"different casing": "she worked hard to achieve her dream.",
	} {
		t.Run(name, func(t *testing.T) {
			result := gradeBody(t, body, map[string]any{keyTextAnswer: submitted}, nil)
			assert.True(t, result.Correct, "punctuation and spacing are not what a vocabulary exercise tests")
			assert.Equal(t, 100, result.Score)
		})
	}
}

func TestVocabularyGrader_ReorderStillRejectsTheWrongOrder(t *testing.T) {
	result := gradeBody(t,
		map[string]any{keyCorrectAnswer: achieveSentence},
		map[string]any{keyTextAnswer: "Hard she worked her dream to achieve."},
		nil)

	assert.False(t, result.Correct)
	assert.Equal(t, 0, result.Score)
}

func TestVocabularyGrader_MatchScoresEveryPair(t *testing.T) {
	// Built from a word list so each id appears once. Eight hand-written
	// "d_habit"s is eight chances to write "d_habitt".
	words := []string{wordHabit, wordJourney, wordLeisure, "afford"}
	key := map[string]string{}
	right := map[string]string{}
	for _, word := range words {
		key["w_"+word] = "d_" + word
		right["w_"+word] = "d_" + word
	}
	body := map[string]any{keyCorrectPairs: key}

	t.Run("all four right", func(t *testing.T) {
		result := gradeBody(t, body, map[string]any{keyPairs: right}, nil)
		assert.True(t, result.Correct)
		assert.Equal(t, 100, result.Score)
	})

	t.Run("three of four is a partial score, not a failure", func(t *testing.T) {
		wrong := map[string]string{}
		for word, definition := range right {
			wrong[word] = definition
		}
		wrong["w_afford"] = "d_" + wordHabit // paired with the wrong meaning
		result := gradeBody(t, body, map[string]any{keyPairs: wrong}, nil)
		// Not correct — the exercise was not completed — but the score says
		// which three quarters the learner knew, which "incorrect" does not.
		assert.False(t, result.Correct)
		assert.Equal(t, 75, result.Score)
	})

	t.Run("nothing submitted", func(t *testing.T) {
		result := gradeBody(t, body, map[string]any{}, nil)
		assert.False(t, result.Correct)
		assert.Equal(t, 0, result.Score)
	})
}

func TestVocabularyGrader_MatchIsGradableWithoutACorrectAnswer(t *testing.T) {
	// A matching body has no single answer. Requiring `correct_answer` would
	// make every `vocab_match` activity fail as "not gradable yet".
	result := gradeBody(t,
		map[string]any{keyCorrectPairs: map[string]string{"w": "d"}},
		map[string]any{keyPairs: map[string]string{"w": "d"}},
		nil)

	assert.True(t, result.Correct)
}

func TestVocabularyGrader_MatchSchedulesEveryWordItAskedAbout(t *testing.T) {
	habit, journey := uuid.New(), uuid.New()
	senses := &fakeSenseResolver{byLemma: map[string]uuid.UUID{
		wordHabit: habit, wordJourney: journey,
	}}

	result := gradeBody(t,
		map[string]any{
			keyCorrectPairs: map[string]string{"w_" + wordHabit: "d_" + wordHabit, "w_" + wordJourney: "d_" + wordJourney},
			keyWordLemmas:   []string{wordHabit, wordJourney},
		},
		map[string]any{keyPairs: map[string]string{
			"w_" + wordHabit:   "d_" + wordHabit,
			"w_" + wordJourney: "d_" + wordJourney,
		}},
		senses)

	require.Len(t, result.ReviewItems, 2,
		"a matching exercise asks about several words and each one earns its own card")
	scheduled := []uuid.UUID{
		result.ReviewItems[0].ContentVersionID,
		result.ReviewItems[1].ContentVersionID,
	}
	assert.ElementsMatch(t, []uuid.UUID{habit, journey}, scheduled)
}

func TestVocabularyGrader_WordLemmasDropWhatDoesNotResolve(t *testing.T) {
	habit := uuid.New()
	senses := &fakeSenseResolver{byLemma: map[string]uuid.UUID{wordHabit: habit}}

	result := gradeBody(t,
		map[string]any{
			keyCorrectPairs: map[string]string{"w_" + wordHabit: "d_" + wordHabit},
			// "sprocket" has no dictionary entry. It must not become a card
			// pointing at nothing, and must not cost "habit" its card either.
			keyWordLemmas: []string{wordHabit, "sprocket", "  "},
		},
		map[string]any{keyPairs: map[string]string{"w_" + wordHabit: "d_" + wordHabit}},
		senses)

	require.Len(t, result.ReviewItems, 1)
	assert.Equal(t, habit, result.ReviewItems[0].ContentVersionID)
}

func TestVocabularyGrader_ListenTypeGradesTheSpelling(t *testing.T) {
	body := map[string]any{
		keyCorrectAnswer: wordLeisure,
		keyAcceptable:    []string{wordLeisure},
		keyWordLemmas:    []string{wordLeisure},
	}

	right := gradeBody(t, body, map[string]any{keyTextAnswer: "Leisure"}, nil)
	assert.True(t, right.Correct, "capitalisation is not a spelling mistake")

	wrong := gradeBody(t, body, map[string]any{keyTextAnswer: "liesure"}, nil)
	assert.False(t, wrong.Correct, "a misspelling is the whole point of this kind")
}

func TestVocabularyGrader_ContextChoiceGradesByOptionID(t *testing.T) {
	body := map[string]any{
		keyCorrectAnswer:    optFreeTime,
		"correct_option_id": optFreeTime,
		keyAcceptable:       []string{wordLeisure},
		keyWordLemmas:       []string{wordLeisure},
	}

	result := gradeBody(t, body, map[string]any{"selected_option_id": optFreeTime}, nil)
	assert.True(t, result.Correct)
	assert.Equal(t, optFreeTime, result.CorrectAnswer,
		"the renderer marks the right row by id, so that is what grading must hand back")
}
