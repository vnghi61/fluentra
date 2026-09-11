package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func TestRepository_NilDB_ReturnsNotFound(t *testing.T) {
	repo := New(nil)
	attemptID := uuid.New()
	userID := uuid.New()

	fb, err := repo.GetWritingFeedback(context.Background(), attemptID, userID)
	require.Error(t, err)
	assert.Nil(t, fb)
	assert.True(t, apperr.Is(err, apperr.NotFound))
}

func TestRepository_NilDB_InsertReturnsNil(t *testing.T) {
	repo := New(nil)
	err := repo.InsertWritingFeedback(context.Background(), contract.WritingFeedback{})
	assert.NoError(t, err)
}
