package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/modules/questionbank/domain"
)

func TestQuestion_Validate(t *testing.T) {
	diffValid := 0.75
	diffInvalid := 1.5

	const testFingerprint = "abc"

	tests := []struct {
		name    string
		q       domain.Question
		wantErr error
	}{
		{
			name: "valid draft question",
			q: domain.Question{
				ID:            uuid.New(),
				ContentItemID: uuid.New(),
				Kind:          "mcq_gap",
				Skill:         "reading",
				CEFRLevel:     "B1",
				QuestionCount: 1,
				Fingerprint:   "abc12345def67890",
				Status:        domain.StatusDraft,
				Difficulty:    &diffValid,
			},
			wantErr: nil,
		},
		{
			name: "invalid question count zero",
			q: domain.Question{
				QuestionCount: 0,
				Fingerprint:   testFingerprint,
				Status:        domain.StatusDraft,
			},
			wantErr: domain.ErrInvalidCount,
		},
		{
			name: "invalid question count over 20",
			q: domain.Question{
				QuestionCount: 21,
				Fingerprint:   testFingerprint,
				Status:        domain.StatusDraft,
			},
			wantErr: domain.ErrInvalidCount,
		},
		{
			name: "empty fingerprint",
			q: domain.Question{
				QuestionCount: 1,
				Fingerprint:   "",
				Status:        domain.StatusDraft,
			},
			wantErr: domain.ErrEmptyFingerprint,
		},
		{
			name: "invalid status",
			q: domain.Question{
				QuestionCount: 1,
				Fingerprint:   testFingerprint,
				Status:        "unknown_status",
			},
			wantErr: domain.ErrInvalidStatus,
		},
		{
			name: "invalid difficulty",
			q: domain.Question{
				QuestionCount: 1,
				Fingerprint:   testFingerprint,
				Status:        domain.StatusDraft,
				Difficulty:    &diffInvalid,
			},
			wantErr: domain.ErrInvalidDifficulty,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.q.Validate()
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestQuestionCountFromBody: a group counts its questions, so the composer can
// fill a part of threes with Part 3 conversations.
func TestQuestionCountFromBody(t *testing.T) {
	part3 := []byte(`{"script":"...","questions":[{"id":"q1"},{"id":"q2"},{"id":"q3"}]}`)
	assert.Equal(t, 3, domain.QuestionCountFromBody("listening_comprehension", part3))
	assert.Equal(t, 4, domain.QuestionCountFromBody("text_completion", []byte(`{"passage":"..."}`)))
	assert.Equal(t, 1, domain.QuestionCountFromBody("question_response", []byte(`{"prompt":"..."}`)))
}
