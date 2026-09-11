package service

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestReadingGrader_GradesMultiQuestionSet(t *testing.T) {
	versionID := uuid.New()
	body, err := json.Marshal(readingQuizBody{
		PassageTitle: "Renewable Energy Transition",
		Passage: "Renewable energy technologies such as solar and wind have experienced dramatic cost reductions " +
			"over the last decade. Many countries are accelerating solar adoption to replace coal power plants. " +
			"However, energy storage solutions remain crucial to address power grid intermittency.",
		Questions: []readingQuestion{
			{
				ID:     "q1",
				Type:   "multiple_choice",
				Prompt: "Which technologies experienced cost reductions?",
				Options: []readingOption{
					{ID: "opt_solar_wind", Text: "Solar and wind"},
					{ID: "opt_nuclear", Text: "Nuclear power"},
				},
				CorrectOptionID: "opt_solar_wind",
			},
			{
				ID:            "q2",
				Type:          "true_false_not_given",
				Prompt:        "Energy storage is needed for grid intermittency.",
				Answer:        "true",
				CorrectAnswer: "true",
				Acceptable:    []string{"True"},
			},
			{
				ID:            "q3",
				Type:          "gap_fill",
				Prompt:        "Solar is replacing ___ power plants.",
				Answer:        "coal",
				CorrectAnswer: "coal",
				Acceptable:    []string{"coal-fired"},
			},
		},
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

	// Case 1: Partial correctness (2 out of 3)
	resPartial, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response: json.RawMessage(`{
			"answers": {
				"q1": "opt_solar_wind",
				"q2": "false",
				"q3": "coal"
			}
		}`),
	})
	require.NoError(t, err)
	assert.False(t, resPartial.Correct)
	assert.Equal(t, 67, resPartial.Score) // 2/3 rounds to 67
	assert.Len(t, resPartial.ItemResults, 3)

	assert.Equal(t, "q1", resPartial.ItemResults[0].ID)
	assert.True(t, resPartial.ItemResults[0].Correct)
	require.NotNil(t, resPartial.ItemResults[0].CorrectAnswer)
	assert.Equal(t, "opt_solar_wind", *resPartial.ItemResults[0].CorrectAnswer)

	assert.Equal(t, "q2", resPartial.ItemResults[1].ID)
	assert.False(t, resPartial.ItemResults[1].Correct)
	require.NotNil(t, resPartial.ItemResults[1].CorrectAnswer)
	assert.Equal(t, "true", *resPartial.ItemResults[1].CorrectAnswer)

	assert.Equal(t, "q3", resPartial.ItemResults[2].ID)
	assert.True(t, resPartial.ItemResults[2].Correct)

	// Review item on partial is again
	assert.Len(t, resPartial.ReviewItems, 1)
	assert.Equal(t, "again", resPartial.ReviewItems[0].InitialGrade)

	// Case 2: Full marks (3 out of 3)
	resFull, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response: json.RawMessage(`{
			"items": [
				{"id": "q1", "answer": "opt_solar_wind"},
				{"id": "q2", "answer": "True"},
				{"id": "q3", "answer": "coal"}
			]
		}`),
	})
	require.NoError(t, err)
	assert.True(t, resFull.Correct)
	assert.Equal(t, 100, resFull.Score)
	assert.Len(t, resFull.ItemResults, 3)
	assert.True(t, resFull.ItemResults[0].Correct)
	assert.True(t, resFull.ItemResults[1].Correct)
	assert.True(t, resFull.ItemResults[2].Correct)
	assert.Equal(t, "good", resFull.ReviewItems[0].InitialGrade)
}

func TestReadingGrader_ReadingSpeedWPM(t *testing.T) {
	versionID := uuid.New()
	// 30 words passage
	passage := "The quick brown fox jumps over the lazy dog every morning before breakfast. " +
		"Nature is beautiful in the spring when trees blossom and birds sing songs of joy and warmth."

	body, err := json.Marshal(readingQuizBody{
		PassageTitle:    "Animal Life",
		Passage:         passage,
		Prompt:          "Who jumps?",
		CorrectOptionID: "opt_fox",
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

	// Valid reading_ms: 15,000 ms (0.25 minutes). 30 words / 0.25 min = 120 WPM.
	readingMs := int64(15000)
	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response: json.RawMessage(fmt.Sprintf(`{
			"selected_option_id": "opt_fox",
			"reading_ms": %d
		}`, readingMs)),
	})
	require.NoError(t, err)
	assert.True(t, res.Correct)
	assert.Contains(t, res.Feedback, "Reading speed: 120 WPM.")

	// Ignored reading_ms: <= 0
	resZero, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response: json.RawMessage(`{
			"selected_option_id": "opt_fox",
			"reading_ms": 0
		}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, resZero.Feedback, "Reading speed:")

	// Ignored reading_ms: negative
	resNegative, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response: json.RawMessage(`{
			"selected_option_id": "opt_fox",
			"reading_ms": -5000
		}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, resNegative.Feedback, "Reading speed:")

	// Ignored reading_ms: over an hour (> 3,600,000 ms)
	resTooLong, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response: json.RawMessage(`{
			"selected_option_id": "opt_fox",
			"reading_ms": 3600001
		}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, resTooLong.Feedback, "Reading speed:")
}

