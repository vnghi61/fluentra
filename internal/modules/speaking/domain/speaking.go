// Package domain implements entities, business invariants, and algorithms for speaking exercises.
package domain

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

var (
	// ErrUnsupportedAudioFormat is returned when the uploaded audio content type is not supported.
	ErrUnsupportedAudioFormat = apperr.New(apperr.BadRequest, "UNSUPPORTED_AUDIO_FORMAT", "audio format not supported")
	// ErrDailyLimitReached is returned when the user exceeds the daily speaking recording quota.
	ErrDailyLimitReached = apperr.New(apperr.RateLimited, "SPEECH_DAILY_LIMIT_REACHED", "daily speaking recordings limit reached")
	// ErrRecordingNotFound is returned when audio recording object does not exist.
	ErrRecordingNotFound = apperr.New(apperr.NotFound, "RECORDING_NOT_FOUND", "recording not found")
	// ErrInvalidRecordingKey is returned when the object key does not belong to the learner.
	ErrInvalidRecordingKey = apperr.New(apperr.BadRequest, "INVALID_RECORDING_KEY", "recording key is invalid or belongs to another user")
	// ErrFeedbackNotFound is returned when speaking feedback does not exist.
	ErrFeedbackNotFound = apperr.New(apperr.NotFound, "FEEDBACK_NOT_FOUND", "speaking feedback not found")
	// ErrSpeakingQueueUnavailable is returned when River enqueuer is missing.
	ErrSpeakingQueueUnavailable = apperr.New(apperr.Internal, "SPEAKING_QUEUE_UNAVAILABLE", "speaking grading queue is not configured")
)

// SupportedAudioTypes defines MIME types accepted for browser recordings.
var SupportedAudioTypes = map[string]bool{
	"audio/webm":          true,
	"audio/webm;codecs=opus": true,
	"audio/mp4":           true,
	"audio/ogg":           true,
	"audio/ogg;codecs=opus":  true,
	"audio/wav":           true,
	"audio/x-wav":         true,
	"audio/mpeg":          true,
}

// IsValidAudioFormat checks if the content type is an accepted audio format.
func IsValidAudioFormat(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		return false
	}
	if SupportedAudioTypes[ct] {
		return true
	}
	// Also accept base type without parameters if matching prefix
	base := strings.Split(ct, ";")[0]
	return SupportedAudioTypes[base]
}

// ValidateRecordingKey ensures the object key conforms to recordings/{userID}/{name}.
func ValidateRecordingKey(key string, userID uuid.UUID) bool {
	if strings.Contains(key, "..") {
		return false
	}
	prefix := fmt.Sprintf("recordings/%s/", userID.String())
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	filename := strings.TrimPrefix(key, prefix)
	return len(filename) > 0 && !strings.Contains(filename, "/")
}

// TokenizeWords converts text into normalized word tokens.
func TokenizeWords(text string) []string {
	var tokens []string
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}

// ComputeReadAloudAccuracy calculates word-level accuracy between reference text and transcribed text.
// Returns a percentage from 0.0 to 100.0.
func ComputeReadAloudAccuracy(referenceText, transcribedText string) float64 {
	refWords := TokenizeWords(referenceText)
	hypWords := TokenizeWords(transcribedText)

	if len(refWords) == 0 {
		if len(hypWords) == 0 {
			return 100.0
		}
		return 0.0
	}

	dist := wordLevenshtein(refWords, hypWords)
	acc := (1.0 - (float64(dist) / float64(len(refWords)))) * 100.0
	if acc < 0.0 {
		acc = 0.0
	}
	if acc > 100.0 {
		acc = 100.0
	}
	return math.Round(acc*100) / 100
}

// ComputeWordsPerMinute computes speaking rate from word count and duration in seconds.
func ComputeWordsPerMinute(wordCount int, durationSeconds float64) int {
	if wordCount <= 0 || durationSeconds <= 0 {
		return 0
	}
	minutes := durationSeconds / 60.0
	return int(math.Round(float64(wordCount) / minutes))
}

func wordLevenshtein(s1, s2 []string) int {
	d := make([][]int, len(s1)+1)
	for i := range d {
		d[i] = make([]int, len(s2)+1)
	}
	for i := 0; i <= len(s1); i++ {
		d[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		d[0][j] = j
	}
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}
	return d[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
