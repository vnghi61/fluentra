package service_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/contract"
	"github.com/fluentra/fluentra/internal/modules/speaking/domain"
	"github.com/fluentra/fluentra/internal/modules/speaking/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/platform/media"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

type fakeContentReader struct {
	version *contentcontract.Version
	err     error
}

func (f *fakeContentReader) GetVersion(_ context.Context, _ uuid.UUID) (*contentcontract.Version, error) {
	return f.version, f.err
}

type fakeAttemptReader struct {
	attempt *learningcontract.AttemptDetail
	err     error
}

func (f *fakeAttemptReader) GetAttemptForGrading(_ context.Context, _ uuid.UUID) (*learningcontract.AttemptDetail, error) {
	return f.attempt, f.err
}

type fakeCompleter struct {
	completedID uuid.UUID
	result      learningcontract.GradeResult
	failedID    uuid.UUID
	failReason  string
}

func (f *fakeCompleter) CompleteAsyncGrading(_ context.Context, attemptID uuid.UUID, result learningcontract.GradeResult) (bool, error) {
	f.completedID = attemptID
	f.result = result
	return true, nil
}

func (f *fakeCompleter) FailAsyncGrading(_ context.Context, attemptID uuid.UUID, reason string) (bool, error) {
	f.failedID = attemptID
	f.failReason = reason
	return true, nil
}

type fakeJobEnqueuer struct {
	enqueuedAttemptID uuid.UUID
}

func (f *fakeJobEnqueuer) EnqueueGradeRecording(_ context.Context, attemptID uuid.UUID) error {
	f.enqueuedAttemptID = attemptID
	return nil
}

type fakeFeedbackWriter struct {
	savedFeedback *contract.SpeakingFeedback
}

func (f *fakeFeedbackWriter) InsertFeedback(_ context.Context, fb *contract.SpeakingFeedback) error {
	f.savedFeedback = fb
	return nil
}

type fakeTranscriber struct {
	result *media.TranscribeResult
	err    error
}

func (f *fakeTranscriber) Transcribe(_ context.Context, _ io.Reader, _ string) (*media.TranscribeResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type fakeAIClient struct {
	response ai.Response
	err      error
}

func (f *fakeAIClient) Complete(_ context.Context, _ ai.Request) (ai.Response, error) {
	if f.err != nil {
		return ai.Response{}, f.err
	}
	return f.response, nil
}

func TestGrader_Grade_Validation(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()
	versionID := uuid.New()

	enqueuer := &fakeJobEnqueuer{}
	grader := service.NewGrader(service.GraderDeps{
		Enqueuer:   enqueuer,
		DailyLimit: 30,
	})

	// 1. Empty response -> 0 score immediately
	res, err := grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        attemptID,
		UserID:           userID,
		ContentVersionID: versionID,
		Response:         nil,
	})
	require.NoError(t, err)
	assert.False(t, res.Async)
	assert.Equal(t, 0, res.Score)
	require.NotEmpty(t, res.ReviewItems)
	assert.Equal(t, "again", res.ReviewItems[0].InitialGrade)

	// 2. Invalid recording key (belongs to other user)
	otherUser := uuid.New()
	invalidKey, _ := json.Marshal(map[string]string{
		"audio_object_key": "recordings/" + otherUser.String() + "/01HX.webm",
	})
	_, err = grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        attemptID,
		UserID:           userID,
		ContentVersionID: versionID,
		Response:         invalidKey,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidRecordingKey)

	// 3. Valid key -> enqueued
	validKey, _ := json.Marshal(map[string]string{
		"audio_object_key": "recordings/" + userID.String() + "/01HX.webm",
	})
	res, err = grader.Grade(context.Background(), learningcontract.GradeRequest{
		AttemptID:        attemptID,
		UserID:           userID,
		ContentVersionID: versionID,
		Response:         validKey,
	})
	require.NoError(t, err)
	assert.True(t, res.Async)
	assert.Equal(t, attemptID, enqueuer.enqueuedAttemptID)
}

func TestGrader_GradeSubmission_ReadAloud(t *testing.T) {
	attemptID := uuid.New()
	userID := uuid.New()
	versionID := uuid.New()
	recordingKey := "recordings/" + userID.String() + "/01ABC.webm"

	bodyJSON, _ := json.Marshal(map[string]any{
		"task_type":      "read_aloud",
		"reference_text": "The quick brown fox jumps over the lazy dog.",
	})
	contentReader := &fakeContentReader{
		version: &contentcontract.Version{
			ID:   versionID,
			Kind: contract.KindSpeakingTask,
			Body: bodyJSON,
		},
	}

	respJSON, _ := json.Marshal(map[string]string{
		"audio_object_key": recordingKey,
	})
	attemptReader := &fakeAttemptReader{
		attempt: &learningcontract.AttemptDetail{
			ID:               attemptID,
			UserID:           userID,
			ContentVersionID: versionID,
			Status:           "grading",
			Response:         respJSON,
		},
	}

	completer := &fakeCompleter{}
	feedbackWriter := &fakeFeedbackWriter{}
	mockStore := &mockStorageStore{}
	transcriber := &fakeTranscriber{
		result: &media.TranscribeResult{
			Text:     "The fast brown fox jumps over a lazy dog.",
			Duration: 5.0,
		},
	}

	aiPayload, _ := json.Marshal(map[string]any{
		"overall_band": 7.0,
		"score":        80,
		"correct":      true,
		"criteria": []map[string]any{
			{"name": "fluency_coherence", "band": 7.0, "comment_en": "Good flow", "comment_vi": "Lưu loát"},
		},
		"feedback_en": "Good reading aloud.",
		"feedback_vi": "Đọc tốt.",
	})
	aiClient := &fakeAIClient{
		response: ai.Response{Text: string(aiPayload), Model: "mock-model"},
	}

	grader := service.NewGrader(service.GraderDeps{
		Content:     contentReader,
		Attempts:    attemptReader,
		Completer:   completer,
		Storage:     mockStore,
		Transcriber: transcriber,
		AI:          aiClient,
		Feedback:    feedbackWriter,
		Clock:       clock.NewFake(time.Now()),
	})

	err := grader.GradeSubmission(context.Background(), attemptID, true)
	require.NoError(t, err)

	// Feedback should be saved
	require.NotNil(t, feedbackWriter.savedFeedback)
	assert.Equal(t, attemptID, feedbackWriter.savedFeedback.AttemptID)
	assert.Equal(t, "The fast brown fox jumps over a lazy dog.", feedbackWriter.savedFeedback.Transcript)
	require.NotNil(t, feedbackWriter.savedFeedback.ReadAloudAccuracy)
	assert.InDelta(t, 77.78, *feedbackWriter.savedFeedback.ReadAloudAccuracy, 0.1)
	require.NotNil(t, feedbackWriter.savedFeedback.WordsPerMinute)

	// Completer should receive the async grading result
	assert.Equal(t, attemptID, completer.completedID)
	assert.True(t, completer.result.Correct)
	assert.True(t, completer.result.Score > 50)
	require.NotEmpty(t, completer.result.ReviewItems)
	assert.Equal(t, "good", completer.result.ReviewItems[0].InitialGrade)
}
