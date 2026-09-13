package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// The learner's meaning for "form": they meant "from".
const boundMeaning = "từ"

// countingModel counts the model calls a verification makes.
type countingModel struct {
	inner ai.Client
	calls int
}

func (c *countingModel) Complete(ctx context.Context, req ai.Request) (ai.Response, error) {
	c.calls++
	return c.inner.Complete(ctx, req)
}

func boundVerdict(t *testing.T, lemma string, meaningMatches bool, intended string) string {
	t.Helper()
	out, err := json.Marshal(map[string]any{
		"valid":           true,
		"lemma":           lemma,
		"part_of_speech":  posNoun,
		"cefr_level":      "A2",
		"meaning_matches": meaningMatches,
		"definition":      "A word.",
		"definition_vi":   boundMeaning,
		"intended_term":   intended,
		"examples":        []any{},
	})
	require.NoError(t, err)
	return string(out)
}

// boundPipeline offers five close spellings of "form", in the order Datamuse
// would, and only the last of them — "from" — fits the learner's meaning.
func boundPipeline(t *testing.T, modelIntends string) (*uploadRepo, *stubDictionary, *countingModel, uuid.UUID) {
	t.Helper()
	neighbours := []string{"farm", "foam", "fork", "fort", wordFrom}

	entries := map[string]repository.DictionaryEntry{
		wordForm: {Lemma: wordForm, PartOfSpeech: posNoun, Definition: "A shape."},
	}
	replies := map[string]string{
		wordForm: boundVerdict(t, wordForm, false, modelIntends),
	}
	for _, word := range neighbours {
		entries[word] = repository.DictionaryEntry{Lemma: word, PartOfSpeech: posNoun, Definition: "A word."}
		replies[word] = boundVerdict(t, word, word == wordFrom, "")
	}

	item := sqlc.SkillVocabUploadItem{
		ID:              uuid.New(),
		UploadID:        uuid.New(),
		UserID:          uuid.New(),
		Term:            wordForm,
		ProvidedMeaning: boundMeaning,
		Status:          "pending",
	}
	repo := newUploadRepo(item)
	dict := &stubDictionary{entries: entries, candidates: map[string][]string{wordForm: neighbours}}
	model := &countingModel{inner: &routingAI{responses: replies}}

	uploads := newTypedWrongPipeline(t, repo, dict, model, &spyEvents{})
	require.NoError(t, uploads.VerifyPending(context.Background()))
	return repo, dict, model, item.ID
}

// TestUploads_SpellingCandidatesAreBounded. Confirming a candidate costs a
// dictionary lookup and a model call, and a short word has many neighbours within
// one edit. The check stops at three candidates, so the fifth is never reached and
// the word is added as typed, with the mismatch note — one call for the word, and
// at most three for its neighbours. Unbounded, this made six of each.
func TestUploads_SpellingCandidatesAreBounded(t *testing.T) {
	repo, dict, model, itemID := boundPipeline(t, "")

	assert.LessOrEqual(t, model.calls, 4, "model calls for one mistyped word")
	assert.LessOrEqual(t, dict.calls, 4, "dictionary lookups for one mistyped word")
	assert.Equal(t, "meaning_mismatch", repo.noteCodes[itemID], "the fifth candidate is never tried")
}

// TestUploads_TheModelsGuessIsTriedFirst. The bound drops the least likely
// candidates, not the best one: the model's own intended_term is checked before
// the dictionary's list, even when the dictionary lists it last.
func TestUploads_TheModelsGuessIsTriedFirst(t *testing.T) {
	repo, _, model, itemID := boundPipeline(t, wordFrom)

	assert.Equal(t, 2, model.calls, "the word as typed, then the model's guess")
	assert.Equal(t, wordFrom, repo.correctedTerms[itemID])
	assert.Equal(t, "meaning_corrected", repo.noteCodes[itemID])
}
