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

	// EventWordsVerified is emitted when a learner's uploaded words pass
	// verification. gamification consumes it and pays XP per word.
	EventWordsVerified = "vocabulary.words_verified"
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

// GradedKinds are the activity kinds this module's grader can score.
//
// It is one list with three readers, and that is the point. `cmd/api` builds
// both the grader registry and DeclaredKinds from it, and the content seed's
// tests assert every kind it authors appears here. When they were three separate
// literals, a kind in the seed that the registry did not know failed the
// learner's request, and a kind declared with no grader behind it failed the
// process at boot — both deliberate, both avoidable.
func GradedKinds() []string {
	return []string{
		"vocabulary_quiz",
		"vocab_multiple_choice",
		"vocab_gap_fill",
		"vocab_flashcard",

		// Added alongside the four above rather than in a module of their own,
		// because a vocabulary exercise is a vocabulary exercise however it is
		// presented: all four ask whether the learner knows a word, and all four
		// schedule the same review card when they are answered.
		//
		// `vocab_listen_type` hears a word and spells it.
		// `vocab_match` pairs words with their definitions.
		// `vocab_reorder` rebuilds an example sentence from shuffled tokens.
		// `vocab_context_choice` picks the meaning a sentence actually uses.
		"vocab_listen_type",
		"vocab_match",
		"vocab_reorder",
		"vocab_context_choice",
	}
}

// Grader defines the exercise grading contract implemented by vocabulary module.
type Grader interface {
	learningcontract.ExerciseGrader
}

// WordsVerified reports how many of a learner's uploaded words were accepted in
// one verification run.
//
// A count rather than one event per word: gamification pays per word either way,
// and a learner who pasted three hundred words should not produce three hundred
// outbox rows.
type WordsVerified struct {
	UserID     uuid.UUID `json:"user_id"`
	Count      int       `json:"count"`
	OccurredAt time.Time `json:"occurred_at"`
}

// WordLearned payload for vocabulary.word_learned event.
type WordLearned struct {
	UserID      uuid.UUID `json:"user_id"`
	WordSenseID uuid.UUID `json:"word_sense_id"`
	CEFRLevel   string    `json:"cefr_level"`
	OccurredAt  time.Time `json:"occurred_at"`
}
