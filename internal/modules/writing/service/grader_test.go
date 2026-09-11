package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/writing/contract"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
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

func TestWritingGrader_SpendsMoney(t *testing.T) {
	grader := NewGrader(nil, nil)
	assert.True(t, grader.SpendsMoney())

	var metered learningcontract.MeteredGrader = grader
	assert.True(t, metered.SpendsMoney())
}

type mockEnqueuer struct {
	enqueued []uuid.UUID
	err      error
}

func (m *mockEnqueuer) EnqueueGradeSubmission(_ context.Context, attemptID uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	m.enqueued = append(m.enqueued, attemptID)
	return nil
}

type mockNudger struct {
	nudged int
}

func (m *mockNudger) Nudge(_ context.Context) {
	m.nudged++
}

type mockAttemptCounter struct {
	count int
	err   error
}

func (m *mockAttemptCounter) CountGradedAttemptsSince(
	_ context.Context, _ uuid.UUID, _ string, _ time.Time,
) (int, error) {
	return m.count, m.err
}

type mockAttemptReader struct {
	attempts map[uuid.UUID]*learningcontract.AttemptDetail
	err      error
}

func (m *mockAttemptReader) GetAttemptForGrading(
	_ context.Context, id uuid.UUID,
) (*learningcontract.AttemptDetail, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.attempts[id], nil
}

type mockCompleter struct {
	completed map[uuid.UUID]learningcontract.GradeResult
	failed    map[uuid.UUID]string
	err       error
}

func (m *mockCompleter) CompleteAsyncGrading(
	_ context.Context, id uuid.UUID, res learningcontract.GradeResult,
) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.completed == nil {
		m.completed = make(map[uuid.UUID]learningcontract.GradeResult)
	}
	m.completed[id] = res
	return true, nil
}

func (m *mockCompleter) FailAsyncGrading(
	_ context.Context, id uuid.UUID, reason string,
) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.failed == nil {
		m.failed = make(map[uuid.UUID]string)
	}
	m.failed[id] = reason
	return true, nil
}

func TestWritingGrader_EmptySubmissionScoredSynchronously(t *testing.T) {
	versionID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write"}`)},
		},
	}
	enq := &mockEnqueuer{}
	grader := NewGraderWithDeps(GraderDeps{
		Content:  reader,
		Enqueuer: enq,
	})

	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "   "}`),
	})
	require.NoError(t, err)
	assert.False(t, res.Async)
	assert.Equal(t, 0, res.Score)
	assert.Empty(t, enq.enqueued)
}

func TestWritingGrader_UnderMinWordsScoredSynchronously(t *testing.T) {
	versionID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":20}`)},
		},
	}
	enq := &mockEnqueuer{}
	grader := NewGraderWithDeps(GraderDeps{
		Content:  reader,
		Enqueuer: enq,
	})

	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "Only four words here"}`),
	})
	require.NoError(t, err)
	assert.False(t, res.Async)
	assert.Equal(t, 10, res.Score) // (4 * 50) / 20 = 10
	assert.Empty(t, enq.enqueued)
}

func TestWritingGrader_DailyLimitReached(t *testing.T) {
	versionID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":5}`)},
		},
	}
	enq := &mockEnqueuer{}
	counter := &mockAttemptCounter{count: 10}
	grader := NewGraderWithDeps(GraderDeps{
		Content:    reader,
		Enqueuer:   enq,
		Counter:    counter,
		DailyLimit: 10,
		Clock:      clock.NewFake(time.Now()),
	})

	_, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		UserID:           uuid.New(),
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "This is a valid long answer with enough words"}`),
	})
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 429, appErr.Status())
	assert.Equal(t, "WRITING_DAILY_LIMIT_REACHED", appErr.Code)
	assert.Empty(t, enq.enqueued)
}

func TestWritingGrader_EnqueuesJobAndNudges(t *testing.T) {
	versionID := uuid.New()
	attemptID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":5}`)},
		},
	}
	enq := &mockEnqueuer{}
	nudger := &mockNudger{}
	counter := &mockAttemptCounter{count: 2}
	grader := NewGraderWithDeps(GraderDeps{
		Content:    reader,
		Enqueuer:   enq,
		Nudger:     nudger,
		Counter:    counter,
		DailyLimit: 10,
		Clock:      clock.NewFake(time.Now()),
	})

	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        attemptID,
		UserID:           uuid.New(),
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "This is a valid long answer with enough words"}`),
	})
	require.NoError(t, err)
	assert.True(t, res.Async)
	assert.Equal(t, []uuid.UUID{attemptID}, enq.enqueued)
	assert.Equal(t, 1, nudger.nudged)
}

func TestWritingGrader_EnqueueFailureReturnsError(t *testing.T) {
	versionID := uuid.New()
	attemptID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":5}`)},
		},
	}
	enqErr := errors.New("db failure")
	enq := &mockEnqueuer{err: enqErr}
	grader := NewGraderWithDeps(GraderDeps{
		Content:  reader,
		Enqueuer: enq,
	})

	_, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        attemptID,
		ContentVersionID: versionID,
		Response:         json.RawMessage(`{"text_answer": "This is a valid long answer with enough words"}`),
	})
	require.ErrorIs(t, err, enqErr)
}

func TestWritingGrader_GradeSubmission_Success(t *testing.T) {
	versionID := uuid.New()
	attemptID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":5}`)},
		},
	}
	attempts := &mockAttemptReader{
		attempts: map[uuid.UUID]*learningcontract.AttemptDetail{
			attemptID: {
				ID:               attemptID,
				ContentVersionID: versionID,
				Status:           statusGrading,
				Response:         json.RawMessage(`{"text_answer": "This is a valid response with several words"}`),
			},
		},
	}
	completer := &mockCompleter{}
	grader := NewGraderWithDeps(GraderDeps{
		Content:   reader,
		Attempts:  attempts,
		Completer: completer,
	})

	err := grader.GradeSubmission(context.Background(), attemptID)
	require.NoError(t, err)
	assert.Contains(t, completer.completed, attemptID)
	assert.GreaterOrEqual(t, completer.completed[attemptID].Score, 50)
}

func TestWritingGrader_GradeSubmission_AlreadyFailedChangesNothing(t *testing.T) {
	versionID := uuid.New()
	attemptID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":5}`)},
		},
	}
	attempts := &mockAttemptReader{
		attempts: map[uuid.UUID]*learningcontract.AttemptDetail{
			attemptID: {
				ID:               attemptID,
				ContentVersionID: versionID,
				Status:           "failed", // Sweep already failed this attempt
				Response:         json.RawMessage(`{"text_answer": "Some text"}`),
			},
		},
	}
	completer := &mockCompleter{}
	grader := NewGraderWithDeps(GraderDeps{
		Content:   reader,
		Attempts:  attempts,
		Completer: completer,
	})

	err := grader.GradeSubmission(context.Background(), attemptID)
	require.NoError(t, err)
	assert.Empty(t, completer.completed)
	assert.Empty(t, completer.failed)
}

type failingAIClient struct{}

func (f *failingAIClient) CompleteJSON(_ context.Context, _ ai.Request, _ any) error {
	return errors.New("ai provider connection error")
}

func (f *failingAIClient) Complete(_ context.Context, _ ai.Request) (ai.Response, error) {
	return ai.Response{}, errors.New("ai provider connection error")
}

func TestWritingGrader_GradeSubmission_ProviderErrorLeavesAttemptFailed(t *testing.T) {
	versionID := uuid.New()
	attemptID := uuid.New()
	reader := &mockContentReader{
		versions: map[uuid.UUID]*contentcontract.Version{
			versionID: {ID: versionID, Body: []byte(`{"prompt":"Write","min_words":5}`)},
		},
	}
	attempts := &mockAttemptReader{
		attempts: map[uuid.UUID]*learningcontract.AttemptDetail{
			attemptID: {
				ID:               attemptID,
				ContentVersionID: versionID,
				Status:           statusGrading,
				Response:         json.RawMessage(`{"text_answer": "This is a valid response with several words"}`),
			},
		},
	}
	completer := &mockCompleter{}
	grader := NewGraderWithDeps(GraderDeps{
		Content:   reader,
		AI:        &failingAIClient{},
		Attempts:  attempts,
		Completer: completer,
	})

	err := grader.GradeSubmission(context.Background(), attemptID)
	require.Error(t, err)
	assert.Contains(t, completer.failed, attemptID)
	assert.Empty(t, completer.completed)
}
