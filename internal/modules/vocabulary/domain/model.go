// Package domain defines the entities, value objects, and business rules for vocabulary.
// Pure Go only. No I/O, no database, no HTTP.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// CEFRLevel represents Common European Framework of Reference language proficiency levels.
type CEFRLevel string

// The six CEFR levels, from beginner to mastery.
const (
	CEFRA1 CEFRLevel = "A1"
	CEFRA2 CEFRLevel = "A2"
	CEFRB1 CEFRLevel = "B1"
	CEFRB2 CEFRLevel = "B2"
	CEFRC1 CEFRLevel = "C1"
	CEFRC2 CEFRLevel = "C2"
)

// PartOfSpeech represents lexical categories.
type PartOfSpeech string

// The parts of speech a dictionary entry may carry.
const (
	POSNoun         PartOfSpeech = "noun"
	POSVerb         PartOfSpeech = "verb"
	POSAdjective    PartOfSpeech = "adjective"
	POSAdverb       PartOfSpeech = "adverb"
	POSPronoun      PartOfSpeech = "pronoun"
	POSPreposition  PartOfSpeech = "preposition"
	POSConjunction  PartOfSpeech = "conjunction"
	POSInterjection PartOfSpeech = "interjection"
	POSIdiom        PartOfSpeech = "idiom"
	POSPhrase       PartOfSpeech = "phrase"
)

// WordStatus represents learner state for a word sense.
type WordStatus string

// A learner's relationship with a word sense. "known" and "ignored" both mean
// stop scheduling it; see service.syncReviewScheduling.
const (
	StatusNew      WordStatus = "new"
	StatusLearning WordStatus = "learning"
	StatusKnown    WordStatus = "known"
	StatusIgnored  WordStatus = "ignored"
)

// ExampleSentence models a contextual sentence illustrating a word sense.
type ExampleSentence struct {
	Sentence   string  `json:"sentence"`
	SentenceVi *string `json:"sentence_vi,omitempty"`
	AudioURL   *string `json:"audio_url,omitempty"`
}

// WordSense models a single distinct meaning of a word.
type WordSense struct {
	ID               uuid.UUID         `json:"id"`
	WordID           uuid.UUID         `json:"word_id"`
	ContentVersionID *uuid.UUID        `json:"content_version_id,omitempty"`
	Definition       string            `json:"definition"`
	DefinitionVi     *string           `json:"definition_vi,omitempty"`
	Register         *string           `json:"register,omitempty"`
	Domain           *string           `json:"domain,omitempty"`
	Examples         []ExampleSentence `json:"examples"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// WordRelation models semantic relationships between words.
type WordRelation struct {
	ID          uuid.UUID    `json:"id"`
	FromWordID  uuid.UUID    `json:"from_word_id"`
	ToWordID    uuid.UUID    `json:"to_word_id"`
	Relation    string       `json:"relation"`
	TargetLemma string       `json:"target_lemma"`
	TargetPOS   PartOfSpeech `json:"target_pos"`
	CreatedAt   time.Time    `json:"created_at"`
}

// Word models a lemma-level vocabulary entry.
type Word struct {
	ID            uuid.UUID      `json:"id"`
	Lemma         string         `json:"lemma"`
	POS           PartOfSpeech   `json:"pos"`
	CEFRLevel     CEFRLevel      `json:"cefr_level"`
	FrequencyRank *int           `json:"frequency_rank,omitempty"`
	IPA           *string        `json:"ipa,omitempty"`
	AudioAssetID  *uuid.UUID     `json:"audio_asset_id,omitempty"`
	AudioURL      *string        `json:"audio_url,omitempty"`
	Senses        []WordSense    `json:"senses,omitempty"`
	Relations     []WordRelation `json:"relations,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Deck models a curated or learner-created collection of word senses.
type Deck struct {
	ID          uuid.UUID  `json:"id"`
	OwnerID     *uuid.UUID `json:"owner_id,omitempty"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	IsPublic    bool       `json:"is_public"`
	ItemCount   int        `json:"item_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// DeckItem models an item within a deck.
type DeckItem struct {
	ID          uuid.UUID  `json:"id"`
	DeckID      uuid.UUID  `json:"deck_id"`
	WordSenseID uuid.UUID  `json:"word_sense_id"`
	Sense       *WordSense `json:"sense,omitempty"`
	Word        *Word      `json:"word,omitempty"`
	AddedAt     time.Time  `json:"added_at"`
}

// UserWordState models learner progress on a specific word sense.
type UserWordState struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	WordSenseID uuid.UUID  `json:"word_sense_id"`
	Status      WordStatus `json:"status"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
