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
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

type mockContentReader struct {
	versions map[uuid.UUID]*contentcontract.Version
}

func (m *mockContentReader) GetVersion(_ context.Context, id uuid.UUID) (*contentcontract.Version, error) {
	return m.versions[id], nil
}

func TestGradedKinds_IncludesWritingPrompt(t *testing.T) {
	kinds := contract.GradedKinds()
	assert.Contains(t, kinds, contract.KindWritingPrompt)
}

func TestWritingGrader_GradesWithAI(t *testing.T) {
	registry, err := ai.NewRegistry()
	require.NoError(t, err)

	aiClient := ai.NewMockProvider(registry)

	versionID := uuid.New()
	body, err := json.Marshal(writingPromptBody{
		Prompt:   "Describe your favorite holiday destination and why you enjoy visiting it.",
		MinWords: 5,
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

	grader := NewGrader(reader, aiClient)

	// Valid submission
	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "I love visiting Da Nang because the beaches are beautiful and peaceful."}`),
	})
	require.NoError(t, err)
	assert.True(t, res.Correct)
	assert.GreaterOrEqual(t, res.Score, 60)
	assert.NotEmpty(t, res.Feedback)
	assert.Len(t, res.ReviewItems, 1)
	assert.Equal(t, "good", res.ReviewItems[0].InitialGrade)
	assert.Equal(t, "writing", res.ReviewItems[0].Skill)
}

func TestWritingGrader_GradesWithoutAI_Fallback(t *testing.T) {
	versionID := uuid.New()
	body, err := json.Marshal(writingPromptBody{
		Prompt:   "Write about your daily routine.",
		MinWords: 10,
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

	// No AI provider (nil client)
	grader := NewGrader(reader, nil)

	// Submission too short
	resShort, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "I wake up early."}`),
	})
	require.NoError(t, err)
	assert.False(t, resShort.Correct)
	assert.Less(t, resShort.Score, 60)

	// Sufficient length submission
	resValid, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "Every morning I wake up at six and make a fresh cup of coffee before starting work."}`),
	})
	require.NoError(t, err)
	assert.True(t, resValid.Correct)
	assert.GreaterOrEqual(t, resValid.Score, 60)
}
