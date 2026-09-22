package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
)

func TestExamVersion_Validate(t *testing.T) {
	t.Parallel()

	v := &domain.ExamVersion{
		ID:           uuid.New(),
		ExamFamily:   domain.FamilyToeicLR,
		Code:         "TOEIC_LR_2026",
		Title:        "TOEIC LR",
		TotalMinutes: 120,
		Scoring:      json.RawMessage(`{}`),
		SourceURL:    "https://example.com",
		VerifiedAt:   time.Now(),
		IsCurrent:    true,
	}
	require.NoError(t, v.Validate())

	v.SourceURL = "http://insecure.com"
	assert.ErrorIs(t, v.Validate(), domain.ErrInvalidSourceURL)

	v.SourceURL = "https://example.com"
	v.ExamFamily = "unknown_family"
	assert.ErrorIs(t, v.Validate(), domain.ErrInvalidExamFamily)
}

func TestExamPart_Validate(t *testing.T) {
	t.Parallel()

	part := &domain.ExamPart{
		ID:            uuid.New(),
		VersionID:     uuid.New(),
		Section:       "listening",
		PartNumber:    3,
		Kind:          "listening_comprehension",
		QuestionCount: 39,
		GroupSize:     3,
	}
	require.NoError(t, part.Validate())

	// Non-divisible group
	part.QuestionCount = 40
	assert.ErrorIs(t, part.Validate(), domain.ErrInvalidGroupSize)

	// Zero group size
	part.GroupSize = 0
	assert.ErrorIs(t, part.Validate(), domain.ErrInvalidGroupSize)
}

func TestBlueprint_Validate(t *testing.T) {
	t.Parallel()

	bp := &domain.Blueprint{
		ID:               uuid.New(),
		VersionID:        uuid.New(),
		Name:             "toeic_default",
		CefrDistribution: json.RawMessage(`{"B1": 0.5}`),
	}
	require.NoError(t, bp.Validate())

	bp.CefrDistribution = nil
	assert.ErrorIs(t, bp.Validate(), domain.ErrInvalidDistribution)
}
