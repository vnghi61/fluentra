package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
)

// ExampleSentenceDTO models an example sentence in HTTP responses.
type ExampleSentenceDTO struct {
	Sentence   string  `json:"sentence"`
	SentenceVi *string `json:"sentence_vi,omitempty"`
	AudioURL   *string `json:"audio_url,omitempty"`
}

// WordSenseDTO models a sense in HTTP responses.
type WordSenseDTO struct {
	ID               uuid.UUID            `json:"id"`
	WordID           uuid.UUID            `json:"word_id"`
	ContentVersionID *uuid.UUID           `json:"content_version_id,omitempty"`
	Definition       string               `json:"definition"`
	DefinitionVi     *string              `json:"definition_vi,omitempty"`
	Register         *string              `json:"register,omitempty"`
	Domain           *string              `json:"domain,omitempty"`
	Examples         []ExampleSentenceDTO `json:"examples"`
	CreatedAt        time.Time            `json:"created_at"`
}

// WordRelationDTO models a semantic link between words.
type WordRelationDTO struct {
	ID          uuid.UUID `json:"id"`
	ToWordID    uuid.UUID `json:"to_word_id"`
	Relation    string    `json:"relation"`
	TargetLemma string    `json:"target_lemma"`
	TargetPOS   string    `json:"target_pos"`
}

// WordDetailDTO models a full dictionary word with senses and relations.
type WordDetailDTO struct {
	ID            uuid.UUID         `json:"id"`
	Lemma         string            `json:"lemma"`
	POS           string            `json:"pos"`
	CEFRLevel     string            `json:"cefr_level"`
	FrequencyRank *int              `json:"frequency_rank,omitempty"`
	IPA           *string           `json:"ipa,omitempty"`
	AudioURL      *string           `json:"audio_url,omitempty"`
	Senses        []WordSenseDTO    `json:"senses"`
	Relations     []WordRelationDTO `json:"relations"`
}

// DictionaryLookupResponse models dictionary lookup results.
type DictionaryLookupResponse struct {
	Words []WordDetailDTO `json:"words"`
}

// WordSummaryDTO is the search-result shape: enough to render a result row and
// decide which entry to open, without the senses a full lookup carries.
type WordSummaryDTO struct {
	ID        uuid.UUID `json:"id"`
	Lemma     string    `json:"lemma"`
	POS       string    `json:"pos"`
	CEFRLevel string    `json:"cefr_level"`
	IPA       *string   `json:"ipa,omitempty"`
}

// SearchWordsResponse models word search results.
type SearchWordsResponse struct {
	Results []WordSummaryDTO `json:"results"`
	Total   int              `json:"total"`
}

// DeckDTO models a vocabulary deck.
type DeckDTO struct {
	ID          uuid.UUID  `json:"id"`
	OwnerID     *uuid.UUID `json:"owner_id,omitempty"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	IsPublic    bool       `json:"is_public"`
	ItemCount   int        `json:"item_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListDecksResponse models listing learner and curated decks.
type ListDecksResponse struct {
	Decks []DeckDTO `json:"decks"`
}

// CreateDeckRequest models request body to create a deck.
type CreateDeckRequest struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	IsPublic    bool    `json:"is_public"`
}

// AddWordToDeckRequest models adding a sense to a deck.
type AddWordToDeckRequest struct {
	WordSenseID uuid.UUID `json:"word_sense_id"`
}

// DeckItemDTO models a word item inside a deck.
type DeckItemDTO struct {
	SenseID      uuid.UUID            `json:"sense_id"`
	WordID       uuid.UUID            `json:"word_id"`
	Lemma        string               `json:"lemma"`
	POS          string               `json:"pos"`
	CEFRLevel    string               `json:"cefr_level"`
	IPA          *string              `json:"ipa,omitempty"`
	AudioURL     *string              `json:"audio_url,omitempty"`
	Definition   string               `json:"definition"`
	DefinitionVi *string              `json:"definition_vi,omitempty"`
	Examples     []ExampleSentenceDTO `json:"examples"`
	AddedAt      time.Time            `json:"added_at"`
}

// ListDeckWordsResponse models words inside a deck.
type ListDeckWordsResponse struct {
	Items []DeckItemDTO `json:"items"`
}

// SetWordStateRequest models updating learner status for a sense.
type SetWordStateRequest struct {
	Status string `json:"status"`
}

// SetWordStateResponse models word status output.
type SetWordStateResponse struct {
	UserID      uuid.UUID  `json:"user_id"`
	WordSenseID uuid.UUID  `json:"word_sense_id"`
	Status      string     `json:"status"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// CreateWordRequest models admin creation of dictionary word entries.
type CreateWordRequest struct {
	Lemma         string         `json:"lemma"`
	POS           string         `json:"pos"`
	CEFRLevel     string         `json:"cefr_level"`
	FrequencyRank *int           `json:"frequency_rank,omitempty"`
	IPA           *string        `json:"ipa,omitempty"`
	AudioAssetID  *uuid.UUID     `json:"audio_asset_id,omitempty"`
	Senses        []WordSenseDTO `json:"senses,omitempty"`
}

// mapWordSummary is the search-result projection of a word.
func mapWordSummary(w domain.Word) WordSummaryDTO {
	return WordSummaryDTO{
		ID:        w.ID,
		Lemma:     w.Lemma,
		POS:       string(w.POS),
		CEFRLevel: string(w.CEFRLevel),
		IPA:       w.IPA,
	}
}

func mapWordDetail(w domain.Word) WordDetailDTO {
	senses := make([]WordSenseDTO, 0, len(w.Senses))
	for _, s := range w.Senses {
		examples := make([]ExampleSentenceDTO, 0, len(s.Examples))
		for _, e := range s.Examples {
			examples = append(examples, ExampleSentenceDTO{
				Sentence:   e.Sentence,
				SentenceVi: e.SentenceVi,
				AudioURL:   e.AudioURL,
			})
		}
		senses = append(senses, WordSenseDTO{
			ID:               s.ID,
			WordID:           s.WordID,
			ContentVersionID: s.ContentVersionID,
			Definition:       s.Definition,
			DefinitionVi:     s.DefinitionVi,
			Register:         s.Register,
			Domain:           s.Domain,
			Examples:         examples,
			CreatedAt:        s.CreatedAt,
		})
	}

	relations := make([]WordRelationDTO, 0, len(w.Relations))
	for _, r := range w.Relations {
		relations = append(relations, WordRelationDTO{
			ID:          r.ID,
			ToWordID:    r.ToWordID,
			Relation:    r.Relation,
			TargetLemma: r.TargetLemma,
			TargetPOS:   string(r.TargetPOS),
		})
	}

	return WordDetailDTO{
		ID:            w.ID,
		Lemma:         w.Lemma,
		POS:           string(w.POS),
		CEFRLevel:     string(w.CEFRLevel),
		FrequencyRank: w.FrequencyRank,
		IPA:           w.IPA,
		AudioURL:      w.AudioURL,
		Senses:        senses,
		Relations:     relations,
	}
}

func mapDeckDTO(d domain.Deck) DeckDTO {
	return DeckDTO{
		ID:          d.ID,
		OwnerID:     d.OwnerID,
		Slug:        d.Slug,
		Name:        d.Name,
		Description: d.Description,
		IsPublic:    d.IsPublic,
		ItemCount:   d.ItemCount,
		CreatedAt:   d.CreatedAt,
	}
}
