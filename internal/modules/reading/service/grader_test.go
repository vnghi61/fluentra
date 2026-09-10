package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/reading/contract"
)

type mockContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (m *mockContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	return m.versions[id], nil
}

func TestGradedKinds_IncludesReadingComprehension(t *testing.T) {
	kinds := contract.GradedKinds()
	assert.Contains(t, kinds, contract.KindReadingComprehension)
}

func TestReadingGrader_GradesCorrectOption(t *testing.T) {
	versionID := uuid.New()
	body, err := json.Marshal(readingQuizBody{
		PassageTitle:    "The Discovery of Coffee",
		Passage:         "According to legend, coffee was discovered by an Ethiopian goat herder named Kaldi.",
		Prompt:          "Who discovered coffee according to legend?",
		CorrectOptionID: "opt_kaldi",
	})
	require.NoError(t, err)

	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {
				ID:   versionID,
				Body: body,
			},
		},
	}

	grader := NewGrader(reader)

	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"selected_option_id": "opt_kaldi"}`),
	})
	require.NoError(t, err)
	assert.True(t, res.Correct)
	assert.Equal(t, 100, res.Score)
	assert.Len(t, res.ReviewItems, 1)
	assert.Equal(t, "good", res.ReviewItems[0].InitialGrade)
	assert.Equal(t, "reading", res.ReviewItems[0].Skill)
}

func TestReadingGrader_GradesIncorrectAnswer(t *testing.T) {
	versionID := uuid.New()
	body, err := json.Marshal(readingQuizBody{
		PassageTitle:  "Solar System",
		Passage:       "Mars is often called the Red Planet because of iron oxide on its surface.",
		Prompt:        "Why is Mars red?",
		CorrectAnswer: "iron oxide",
		Acceptable:    []string{"rust"},
	})
	require.NoError(t, err)

	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {
				ID:   versionID,
				Body: body,
			},
		},
	}

	grader := NewGrader(reader)

	// Incorrect response
	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "copper"}`),
	})
	require.NoError(t, err)
	assert.False(t, res.Correct)
	assert.Equal(t, 0, res.Score)
	assert.Equal(t, "again", res.ReviewItems[0].InitialGrade)

	// Correct acceptable response
	resAcceptable, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "rust"}`),
	})
	require.NoError(t, err)
	assert.True(t, resAcceptable.Correct)
	assert.Equal(t, 100, resAcceptable.Score)
}
