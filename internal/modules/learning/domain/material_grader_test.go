package domain_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

// Marking a material done is the whole task, so {"done": true} scores full
// marks; anything else completes nothing (WO 20, D20-8).
func TestMaterialGrader(t *testing.T) {
	grader := domain.NewMaterialGrader()

	done, err := grader.Grade(context.Background(), contract.GradeRequest{
		Response: json.RawMessage(`{"done": true}`),
	})
	require.NoError(t, err)
	assert.Equal(t, 100, done.Score)
	assert.Equal(t, 100, done.MaxScore)
	assert.True(t, done.Correct)

	notDone, err := grader.Grade(context.Background(), contract.GradeRequest{
		Response: json.RawMessage(`{"done": false}`),
	})
	require.NoError(t, err)
	assert.Equal(t, 0, notDone.Score)
	assert.False(t, notDone.Correct)
}
