package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// seedKnownWord puts a word and its Vietnamese sense in the fake database, as a
// previous upload would have.
func seedKnownWord(repo *uploadRepo, lemma, meaning string) sqlc.SkillWordSense {
	word := sqlc.SkillWord{
		ID: uuid.New(), Lemma: lemma, Pos: posNoun, CefrLevel: "B1",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	repo.words[word.ID] = word
	sense := sqlc.SkillWordSense{
		ID: uuid.New(), WordID: word.ID, Definition: "Free time.",
		DefinitionVi: &meaning, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	repo.senses[sense.ID] = sense
	return sense
}

// TestVerifyUpload_KnownWordCostsNoDictionaryAndNoModel is the WO 22 Stage B
// gate: a word the database already holds is added with no dictionary and no
// model call.
func TestVerifyUpload_KnownWordCostsNoDictionaryAndNoModel(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	entry := item(wordLeisure, "")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{}
	model := &stubAI{err: assert.AnError}
	uploads, author := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Equal(t, 0, dict.calls, "a known word must not call the dictionary")
	assert.Equal(t, 0, model.calls, "a known word must not call the model")
	assert.Contains(t, repo.verified, entry.ID)
	assert.Equal(t, "known_word", repo.noteCodes[entry.ID])
	assert.Empty(t, author.published, "a known word reuses the stored sense, creating no content")
}

// TestVerifyUpload_KnownWordWithAMatchingMeaningCostsNothingEither. The learner
// wrote the meaning themselves; it matches a stored sense, so the word is reused
// with no calls.
func TestVerifyUpload_KnownWordWithAMatchingMeaningCostsNothingEither(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	entry := item(wordLeisure, "Thời gian rảnh!")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{}
	model := &stubAI{err: assert.AnError}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Equal(t, 0, dict.calls)
	assert.Equal(t, 0, model.calls)
	assert.Contains(t, repo.verified, entry.ID)
}

// TestVerifyUpload_KnownWordWithANewMeaningTakesTheDictionaryPath. A meaning
// that matches no stored sense is left to the normal path, which adds a sense to
// the existing word rather than duplicating it.
func TestVerifyUpload_KnownWordWithANewMeaningTakesTheDictionaryPath(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	entry := item(wordLeisure, "sự giải trí")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	model := &stubAI{reply: accepted(t)}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Positive(t, dict.calls, "a new meaning is not answered from the database alone")
	assert.Positive(t, model.calls)
}
