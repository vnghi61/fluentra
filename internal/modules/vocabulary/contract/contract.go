// Package contract exposes the public interfaces, DTOs and event payloads
// of the vocabulary skill module.
package contract

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

const (
	// Aggregate identifies vocabulary domain events.
	Aggregate = "vocabulary"

	// EventWordLearned is emitted when a learner masters a word sense.
	EventWordLearned = "vocabulary.word_learned"
)

// WordSense models a single definition and examples for a vocabulary entry.
type WordSense struct {
	ID               uuid.UUID       `json:"id"`
	WordID           uuid.UUID       `json:"word_id"`
	Definition       string          `json:"definition"`
	Register         *string         `json:"register,omitempty"`
	Domain           *string         `json:"domain,omitempty"`
	Examples         json.RawMessage `json:"examples,omitempty"`
	ContentVersionID *uuid.UUID      `json:"content_version_id,omitempty"`
	AudioURL         *string         `json:"audio_url,omitempty"`
}

// WordDetail models a full word entry including its lemma, pronunciation, and senses.
type WordDetail struct {
	ID            uuid.UUID   `json:"id"`
	Lemma         string      `json:"lemma"`
	Pos           string      `json:"pos"`
	CEFRLevel     string      `json:"cefr_level"`
	FrequencyRank *int        `json:"frequency_rank,omitempty"`
	IPA           *string     `json:"ipa,omitempty"`
	Senses        []WordSense `json:"senses"`
}

// Reader provides vocabulary lookup and sense inspection.
type Reader interface {
	LookupWord(ctx context.Context, lemma string) (*WordDetail, error)
	GetSenses(ctx context.Context, senseIDs []uuid.UUID) ([]WordSense, error)
}

// Grader defines the exercise grading contract implemented by vocabulary module.
type Grader interface {
	learningcontract.ExerciseGrader
}

// WordLearned payload for vocabulary.word_learned event.
type WordLearned struct {
	UserID      uuid.UUID `json:"user_id"`
	WordSenseID uuid.UUID `json:"word_sense_id"`
	CEFRLevel   string    `json:"cefr_level"`
	OccurredAt  time.Time `json:"occurred_at"`
}
