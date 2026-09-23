package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// chooseSenseAI answers only the choose-sense task, picking the id it was given.
type chooseSenseAI struct {
	pick uuid.UUID
}

func (a *chooseSenseAI) Complete(_ context.Context, _ ai.Request) (ai.Response, error) {
	payload, err := json.Marshal(map[string]any{
		"sense_id": a.pick.String(),
		"is_new":   a.pick == uuid.Nil,
	})
	if err != nil {
		return ai.Response{}, err
	}
	return ai.Response{Text: string(payload), Model: "stub"}, nil
}

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
// that matches no stored sense and that the model calls new is left to the
// normal path, which adds a sense to the existing word rather than duplicating
// it.
func TestVerifyUpload_KnownWordWithANewMeaningTakesTheDictionaryPath(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	entry := item(wordLeisure, "sự giải trí")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	// The model says the meaning is new, so the dictionary path follows.
	model := &chooseSenseAI{pick: uuid.Nil}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Positive(t, dict.calls, "a new meaning is not answered from the database alone")
}

// TestVerifyUpload_KnownWordTheModelMatchesToAStoredSenseCostsNoDictionary. The
// learner's wording differs from every stored sense, but the model recognises it
// as one of them, so the word is reused with no dictionary call (step 3).
func TestVerifyUpload_KnownWordTheModelMatchesToAStoredSenseCostsNoDictionary(t *testing.T) {
	repo := newUploadRepo()
	sense := seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	entry := item(wordLeisure, "sự nhàn rỗi")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{}
	model := &chooseSenseAI{pick: sense.ID}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Equal(t, 0, dict.calls, "the model's choice needs no dictionary")
	assert.Contains(t, repo.verified, entry.ID)
}

// TestSubmit_LabelsKnownWordsBeforeTheJobRuns is step 5: the response says which
// pasted words the database already holds.
func TestSubmit_LabelsKnownWordsBeforeTheJobRuns(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")

	svc := service.New(service.Deps{Repo: repo})
	uploads := service.NewUploads(svc, repo, service.UploadDeps{
		Pool:     stubPool{},
		Beginner: stubPool{},
	})

	res, err := uploads.Submit(context.Background(), uuid.New(), "leisure - free time\nserendipity")
	require.NoError(t, err)
	require.Len(t, res.Items, 2)

	byTerm := map[string]string{}
	for _, it := range res.Items {
		byTerm[it.Term] = it.Status
	}
	assert.Equal(t, "known", byTerm[wordLeisure])
	assert.Equal(t, "checking", byTerm["serendipity"])
}
