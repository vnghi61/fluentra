// Package media provides audio synthesis and speech-to-text recognition capabilities.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

var (
	// ErrTTSNotFound indicates that a requested TTS clip is not cached and the engine is offline.
	ErrTTSNotFound = apperr.New(apperr.NotFound, "TTS_NOT_FOUND", "tts audio not found in cache")
	// ErrTranscribeFailed indicates an error during audio transcription.
	ErrTranscribeFailed = apperr.New(apperr.Unavailable, "TRANSCRIBE_FAILED", "audio transcription failed")
	// ErrEmptyAudio indicates an empty or invalid audio stream was provided.
	ErrEmptyAudio = apperr.New(apperr.BadRequest, "EMPTY_AUDIO", "empty audio stream provided")
)


// WordTiming represents word-level timestamp information from transcription.
type WordTiming struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// TranscribeResult holds the transcribed text and associated metadata.
type TranscribeResult struct {
	Text     string       `json:"text"`
	Words    []WordTiming `json:"words,omitempty"`
	Language string       `json:"language,omitempty"`
	Duration float64      `json:"duration,omitempty"`
}

// Transcriber transcribes audio data into text with word timings.
type Transcriber interface {
	Transcribe(ctx context.Context, audio io.Reader, filename string) (*TranscribeResult, error)
}

// Synthesiser turns text into pre-rendered audio, returning the object key.
type Synthesiser interface {
	Synthesise(ctx context.Context, text, voice string) (objectKey string, err error)
}

// TTSCache provides lookup and insertion into the tts_cache store.
type TTSCache interface {
	Get(ctx context.Context, textHash, voice string) (objectKey string, found bool, err error)
	Put(ctx context.Context, textHash, voice, engine, engineVersion, objectKey string) error
}

// AudioUploader is an abstraction for uploading rendered audio to object storage.
type AudioUploader interface {
	Put(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) error
}

// HashText returns a deterministic SHA-256 hex string of the trimmed text.
func HashText(text string) string {
	normalized := strings.TrimSpace(text)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
