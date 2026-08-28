package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
)

type fakeContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (f *fakeContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	if v, ok := f.versions[id]; ok {
		return v, nil
	}
	return nil, nil
}

const (
	correctAnswerField = "correct_answer"
	promptField        = "prompt"
	answerField        = "answer"

	// A word the fixtures reuse; goconst objects to the literal repeating.
	wordMeticulous = "meticulous"
)

func TestVocabularyGrader_CorrectAnswer(t *testing.T) {
	contentID := uuid.New()
	bodyJSON, _ := json.Marshal(map[string]any{
		correctAnswerField: wordMeticulous,
		"acceptable":       []string{"meticulously"},
		promptField:        "Showing great attention to detail.",
	})

	contentReader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {
				ID:   contentID,
				Body: bodyJSON,
			},
		},
	}

	grader := service.NewGrader(contentReader, nil)

	respJSON, _ := json.Marshal(map[string]string{
		answerField: wordMeticulous,
	})

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        uuid.New(),
		ActivityID:       uuid.New(),
		ContentVersionID: contentID,
		UserID:           uuid.New(),
		Response:         respJSON,
	})
	require.NoError(t, err)
	assert.True(t, result.Correct)
	assert.Equal(t, 100, result.Score)
	require.Len(t, result.ReviewItems, 1)
	assert.Equal(t, "good", result.ReviewItems[0].InitialGrade)
	assert.Equal(t, "vocabulary", result.ReviewItems[0].Skill)
}

func TestVocabularyGrader_AcceptableAlternative(t *testing.T) {
	contentID := uuid.New()
	bodyJSON, _ := json.Marshal(map[string]any{
		correctAnswerField: "flavour",
		"acceptable":       []string{"flavor"},
		promptField:        "Taste of something.",
	})

	contentReader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {
				ID:   contentID,
				Body: bodyJSON,
			},
		},
	}

	grader := service.NewGrader(contentReader, nil)

	respJSON, _ := json.Marshal(map[string]string{
		answerField: "flavor", // BR-VOCABULARY-05: British and American spelling both accepted
	})

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        uuid.New(),
		ActivityID:       uuid.New(),
		ContentVersionID: contentID,
		UserID:           uuid.New(),
		Response:         respJSON,
	})
	require.NoError(t, err)
	assert.True(t, result.Correct)
	assert.Equal(t, 100, result.Score)
	require.Len(t, result.ReviewItems, 1)
	assert.Equal(t, "good", result.ReviewItems[0].InitialGrade)
}

func TestVocabularyGrader_WrongAnswer(t *testing.T) {
	contentID := uuid.New()
	bodyJSON, _ := json.Marshal(map[string]any{
		correctAnswerField: "ephemeral",
		promptField:        "Lasting for a very short time.",
	})

	contentReader := &fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {
				ID:   contentID,
				Body: bodyJSON,
			},
		},
	}

	grader := service.NewGrader(contentReader, nil)

	respJSON, _ := json.Marshal(map[string]string{
		answerField: "permanent",
	})

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        uuid.New(),
		ActivityID:       uuid.New(),
		ContentVersionID: contentID,
		UserID:           uuid.New(),
		Response:         respJSON,
	})
	require.NoError(t, err)
	assert.False(t, result.Correct)
	assert.Equal(t, 0, result.Score)
	require.Len(t, result.ReviewItems, 1)
	assert.Equal(t, "again", result.ReviewItems[0].InitialGrade)
}

// TestVocabularyGrader_UngradableContentIsAnError pins the behaviour that
// replaced a "when in doubt, mark it correct" fallback. Content with no answer
// key is a deployment fault; grading it as a pass would inflate progress and
// schedule a review card for a word the learner may not know.
func TestVocabularyGrader_UngradableContentIsAnError(t *testing.T) {
	respJSON, _ := json.Marshal(map[string]string{answerField: "anything at all"})

	cases := map[string]json.RawMessage{
		"no answer key": json.RawMessage(`{"prompt":"Showing great attention to detail."}`),
		"empty body":    json.RawMessage(``),
		"not a quiz":    json.RawMessage(`"just a string"`),
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			contentID := uuid.New()
			grader := service.NewGrader(&fakeContentReader{
				versions: map[uuid.UUID]*contentcontract.Version{
					contentID: {ID: contentID, Body: body},
				},
			}, nil)

			_, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
				ContentVersionID: contentID,
				UserID:           uuid.New(),
				Response:         respJSON,
			})
			require.Error(t, err, "an ungradable activity must not silently pass the learner")
		})
	}
}

// TestVocabularyGrader_UnknownContentIsAnError covers the version the content
// module does not have at all.
func TestVocabularyGrader_UnknownContentIsAnError(t *testing.T) {
	grader := service.NewGrader(&fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{}}, nil)

	_, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: uuid.New(),
		UserID:           uuid.New(),
		Response:         json.RawMessage(`{"answer":"anything"}`),
	})
	require.Error(t, err)
}

// TestVocabularyGrader_EmptyAnswerIsWrong: skipping the question is not a pass.
func TestVocabularyGrader_EmptyAnswerIsWrong(t *testing.T) {
	contentID := uuid.New()
	grader := service.NewGrader(&fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			contentID: {ID: contentID, Body: json.RawMessage(`{"correct_answer":"` + wordMeticulous + `"}`)},
		},
	}, nil)

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: contentID,
		UserID:           uuid.New(),
		Response:         json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.False(t, result.Correct)
	assert.Equal(t, "again", result.ReviewItems[0].InitialGrade)
}

// fakeSenseResolver maps a lemma to the content version of its dictionary entry.
type fakeSenseResolver struct {
	byLemma map[string]uuid.UUID
	calls   int
}

func (f *fakeSenseResolver) GetSenseContentVersionByLemma(
	_ context.Context, lemma string,
) (*uuid.UUID, error) {
	f.calls++
	id, ok := f.byLemma[lemma]
	if !ok {
		return nil, errNoSense
	}
	return &id, nil
}

var errNoSense = errors.New("no sense for that lemma")

// TestVocabularyGrader_SchedulesTheWordNotTheExercise is the fix for every
// review card rendering "This card has no content yet".
//
// The card used to point at the activity's own content version — a body holding
// a prompt and an answer key, which is what grades an exercise and not what a
// flashcard shows. The review screen wants the dictionary entry, so that is what
// gets scheduled: an exercise is one way of asking about a word, and the thing
// worth remembering in three days is the word.
func TestVocabularyGrader_SchedulesTheWordNotTheExercise(t *testing.T) {
	activityVersion, senseVersion := uuid.New(), uuid.New()
	senses := &fakeSenseResolver{byLemma: map[string]uuid.UUID{wordMeticulous: senseVersion}}

	grader := service.NewGrader(&fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			activityVersion: {
				ID:   activityVersion,
				Body: json.RawMessage(`{"correct_answer":"` + wordMeticulous + `","prompt":"p"}`),
			},
		},
	}, senses)

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: activityVersion,
		Response:         json.RawMessage(`{"answer":"` + wordMeticulous + `"}`),
	})
	require.NoError(t, err)
	require.Len(t, result.ReviewItems, 1)

	assert.Equal(t, senseVersion, result.ReviewItems[0].ContentVersionID,
		"the card must schedule the word's dictionary entry, not the exercise that asked about it")
	assert.NotEqual(t, activityVersion, result.ReviewItems[0].ContentVersionID)
	assert.Equal(t, 1, senses.calls)
}

// A word with no dictionary entry still earns a card. Falling back to the
// activity's version keeps the schedule intact — the learner answered, and
// losing that because the content is thin would be the worse failure.
func TestVocabularyGrader_FallsBackWhenTheWordIsUnknown(t *testing.T) {
	activityVersion := uuid.New()
	senses := &fakeSenseResolver{byLemma: map[string]uuid.UUID{}}

	grader := service.NewGrader(&fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			activityVersion: {
				ID:   activityVersion,
				Body: json.RawMessage(`{"correct_answer":"` + wordMeticulous + `"}`),
			},
		},
	}, senses)

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: activityVersion,
		Response:         json.RawMessage(`{"answer":"` + wordMeticulous + `"}`),
	})
	require.NoError(t, err)
	require.Len(t, result.ReviewItems, 1)
	assert.Equal(t, activityVersion, result.ReviewItems[0].ContentVersionID)
}

// The lookup is by normalised lemma, because the authored answer is authored by
// a person: "Meticulous " and "meticulous" are the same word.
func TestVocabularyGrader_ResolvesTheLemmaCaseInsensitively(t *testing.T) {
	activityVersion, senseVersion := uuid.New(), uuid.New()
	senses := &fakeSenseResolver{byLemma: map[string]uuid.UUID{wordMeticulous: senseVersion}}

	grader := service.NewGrader(&fakeContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			activityVersion: {
				ID:   activityVersion,
				Body: json.RawMessage(`{"correct_answer":"  Meticulous  "}`),
			},
		},
	}, senses)

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: activityVersion,
		Response:         json.RawMessage(`{"answer":"` + wordMeticulous + `"}`),
	})
	require.NoError(t, err)
	require.Len(t, result.ReviewItems, 1)
	assert.Equal(t, senseVersion, result.ReviewItems[0].ContentVersionID)
}
