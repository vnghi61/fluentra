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

	grader := service.NewGrader(contentReader)

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

	grader := service.NewGrader(contentReader)

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

	grader := service.NewGrader(contentReader)

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
			})

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
	grader := service.NewGrader(&fakeContentReader{versions: map[uuid.UUID]*contentcontract.Version{}})

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
	})

	result, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: contentID,
		UserID:           uuid.New(),
		Response:         json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.False(t, result.Correct)
	assert.Equal(t, "again", result.ReviewItems[0].InitialGrade)
}
