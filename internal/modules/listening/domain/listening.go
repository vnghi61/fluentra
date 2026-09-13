// Package domain defines the entities, errors, and business invariants for listening comprehension.
package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// Exercise kind and context type constants.
const (
	KindListeningComprehension = "listening_comprehension"

	ContextTypeAttempt = "attempt"
	ContextTypeExam    = "exam"

	MaxExamPlays    = 1
	MaxAttemptPlays = 3
)

var (
	// ErrPlayLimitReached is returned when the user has exhausted allowed plays.
	ErrPlayLimitReached = apperr.New(apperr.Forbidden, "PLAY_LIMIT_REACHED", "no plays remaining for this attempt")
	// ErrTranscriptLocked is returned when attempting to access the transcript before the attempt is graded.
	ErrTranscriptLocked = apperr.New(apperr.Forbidden, "TRANSCRIPT_LOCKED", "transcript not yet available")
	// ErrAudioNotReady is returned when audio has not been synthesised or uploaded.
	ErrAudioNotReady = apperr.New(apperr.Conflict, "AUDIO_NOT_READY", "media still processing")
	// ErrInvalidContext is returned when context_type is not 'attempt' or 'exam'.
	ErrInvalidContext = apperr.New(apperr.BadRequest, "INVALID_CONTEXT", "context_type must be attempt or exam")
	// ErrItemNotFound is returned when the listening content version does not exist.
	ErrItemNotFound = apperr.New(apperr.NotFound, "ITEM_NOT_FOUND", "listening item not found")
)

// MaxPlays returns the maximum number of audio plays allowed for a given context type.
func MaxPlays(contextType string) int {
	if contextType == ContextTypeExam {
		return MaxExamPlays
	}
	return MaxAttemptPlays
}

// PlayRecord records a single audio playback event.
type PlayRecord struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	ContentVersionID uuid.UUID
	ContextType      string
	ContextID        uuid.UUID
	PlayedAt         time.Time
}

// PlayResult represents the outcome of recording a play and issuing a presigned audio URL.
type PlayResult struct {
	AudioURL     string    `json:"audio_url"`
	PlaysUsed    int       `json:"plays_used"`
	PlaysAllowed int       `json:"plays_allowed"`
	ExpiresAt    time.Time `json:"expires_at"`
}
