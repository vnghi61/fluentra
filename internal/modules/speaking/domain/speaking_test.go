package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/modules/speaking/domain"
)

func TestIsValidAudioFormat(t *testing.T) {
	assert.True(t, domain.IsValidAudioFormat("audio/webm"))
	assert.True(t, domain.IsValidAudioFormat("audio/webm;codecs=opus"))
	assert.True(t, domain.IsValidAudioFormat("audio/mp4"))
	assert.True(t, domain.IsValidAudioFormat("audio/wav"))
	assert.False(t, domain.IsValidAudioFormat("application/json"))
	assert.False(t, domain.IsValidAudioFormat("video/avi"))
	assert.False(t, domain.IsValidAudioFormat(""))
}

func TestValidateRecordingKey(t *testing.T) {
	userID := uuid.New()
	validKey := "recordings/" + userID.String() + "/01HXABC123.webm"
	assert.True(t, domain.ValidateRecordingKey(validKey, userID))

	otherUser := uuid.New()
	assert.False(t, domain.ValidateRecordingKey(validKey, otherUser))

	traversalKey := "recordings/" + userID.String() + "/../../etc/passwd"
	assert.False(t, domain.ValidateRecordingKey(traversalKey, userID))

	nestedKey := "recordings/" + userID.String() + "/sub/file.webm"
	assert.False(t, domain.ValidateRecordingKey(nestedKey, userID))
}

func TestComputeReadAloudAccuracy(t *testing.T) {
	ref := "The quick brown fox jumps over the lazy dog."
	hypExact := "The quick brown fox jumps over the lazy dog."
	assert.Equal(t, 100.0, domain.ComputeReadAloudAccuracy(ref, hypExact))

	hypClose := "The fast brown fox jumps over a lazy dog."
	// 2 substitutions ("quick"->"fast", "the"->"a") out of 9 words: (1 - 2/9) * 100 ≈ 77.78%
	acc := domain.ComputeReadAloudAccuracy(ref, hypClose)
	assert.InDelta(t, 77.78, acc, 0.1)

	hypCompletelyWrong := "Nothing matches here at all whatsoever."
	accWrong := domain.ComputeReadAloudAccuracy(ref, hypCompletelyWrong)
	assert.True(t, accWrong < 25.0)

	assert.Equal(t, 100.0, domain.ComputeReadAloudAccuracy("", ""))
}

func TestComputeWordsPerMinute(t *testing.T) {
	// 60 words in 30 seconds -> 120 WPM
	assert.Equal(t, 120, domain.ComputeWordsPerMinute(60, 30.0))
	// 0 words or negative duration -> 0
	assert.Equal(t, 0, domain.ComputeWordsPerMinute(0, 30.0))
	assert.Equal(t, 0, domain.ComputeWordsPerMinute(60, 0.0))
}
