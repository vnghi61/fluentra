package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// The upload pipeline, exercised through the verification job.
//
// The rules that matter are about what happens when something goes wrong: a
// dictionary that is unreachable must not reject a learner's good word, a model
// that refuses must not silently accept one, and a word that nothing can
// resolve must eventually stop being retried.

// keyValid is the field the vocab_verify template asks the model to set.
const keyValid = "valid"

// ---------------------------------------------------------------- fakes

type stubDictionary struct {
	entries map[string]repository.DictionaryEntry
	err     error
	calls   int
}

func (s *stubDictionary) Lookup(
	_ context.Context, word string,
) (repository.DictionaryEntry, error) {
	s.calls++
	if s.err != nil {
		return repository.DictionaryEntry{}, s.err
	}
	entry, ok := s.entries[word]
	if !ok {
		return repository.DictionaryEntry{}, repository.ErrWordNotFound
	}
	return entry, nil
}

// stubAI answers with whatever verdict the test wants.
type stubAI struct {
	reply string
	err   error
	calls int
}

func (s *stubAI) Complete(_ context.Context, _ ai.Request) (ai.Response, error) {
	s.calls++
	if s.err != nil {
		return ai.Response{}, s.err
	}
	return ai.Response{Text: s.reply, Model: "stub"}, nil
}

type stubContentAuthor struct {
	published []contentcontract.AuthorSpec
	id        uuid.UUID
}

func (s *stubContentAuthor) EnsurePublished(
	_ context.Context, spec contentcontract.AuthorSpec,
) (uuid.UUID, error) {
	s.published = append(s.published, spec)
	if s.id == uuid.Nil {
		s.id = uuid.New()
	}
	return s.id, nil
}

// uploadRepo records what the pipeline did to each item.
type uploadRepo struct {
	*fakeRepo

	pending  []sqlc.SkillVocabUploadItem
	verified map[uuid.UUID]string
	rejected map[uuid.UUID]string
	attempts map[uuid.UUID]string
}

func newUploadRepo(items ...sqlc.SkillVocabUploadItem) *uploadRepo {
	return &uploadRepo{
		fakeRepo: newFakeRepo(),
		pending:  items,
		verified: map[uuid.UUID]string{},
		rejected: map[uuid.UUID]string{},
		attempts: map[uuid.UUID]string{},
	}
}

func (r *uploadRepo) ClaimPendingUploadItems(
	_ context.Context, _, _ int32,
) ([]sqlc.SkillVocabUploadItem, error) {
	return r.pending, nil
}

func (r *uploadRepo) MarkUploadItemVerified(
	_ context.Context, id uuid.UUID, _ *uuid.UUID, model, _ string,
) (sqlc.SkillVocabUploadItem, error) {
	r.verified[id] = model
	return sqlc.SkillVocabUploadItem{ID: id}, nil
}

func (r *uploadRepo) MarkUploadItemRejected(
	_ context.Context, id uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	r.rejected[id] = reason
	return sqlc.SkillVocabUploadItem{ID: id}, nil
}

func (r *uploadRepo) RecordUploadItemAttempt(
	_ context.Context, id uuid.UUID, reason string,
) error {
	r.attempts[id] = reason
	return nil
}

// ------------------------------------------------------------- fixtures

func item(term, meaning string) sqlc.SkillVocabUploadItem {
	return sqlc.SkillVocabUploadItem{
		ID: uuid.New(), UploadID: uuid.New(), UserID: uuid.New(),
		Term: term, ProvidedMeaning: meaning, Status: "pending",
	}
}

func leisureEntry() repository.DictionaryEntry {
	return repository.DictionaryEntry{
		Lemma: wordLeisure, IPA: "/ˈliːʒə(ɹ)/", PartOfSpeech: "noun",
		Definition: "Time when one is not working or occupied; free time.",
		AudioURL:   "https://example.test/leisure.mp3",
	}
}

func accepted(t *testing.T) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		keyValid: true, "lemma": wordLeisure, "part_of_speech": "noun",
		"cefr_level": "B1", "definition": "Free time.",
		"meaning_matches": true,
		"examples": []string{
			"He reads at leisure.", "She has little leisure.", "Leisure time is short.",
		},
	})
	require.NoError(t, err)
	return string(payload)
}

func newPipeline(
	t *testing.T, repo *uploadRepo, dict *stubDictionary, model ai.Client,
) (*service.Uploads, *stubContentAuthor) {
	t.Helper()
	author := &stubContentAuthor{}
	svc := service.New(service.Deps{Repo: repo})
	return service.NewUploads(svc, repo, service.UploadDeps{
		Dictionary: dict,
		AI:         model,
		Content:    author,
		AuthorID:   uuid.New(),
	}), author
}

// -------------------------------------------------------------- submitting

func TestSubmit_RefusesTextWithNoWordsInIt(t *testing.T) {
	repo := newUploadRepo()
	uploads, _ := newPipeline(t, repo, &stubDictionary{}, nil)

	_, err := uploads.Submit(context.Background(), uuid.New(), "---\n42\n\n")
	require.Error(t, err, "a paste of dividers and page numbers is a mistake, not an upload")
}

// ------------------------------------------------------------ verifying

func TestVerifyPending_AcceptsAWordTheDictionaryKnows(t *testing.T) {
	entry := item(wordLeisure, "thời gian rảnh")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: accepted(t)})

	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Contains(t, repo.verified, entry.ID)
	assert.Empty(t, repo.rejected)
	require.Len(t, author.published, 1, "a verified word needs a content version to review against")
}

func TestVerifyPending_TheStoredContentCarriesWhatAFlashcardNeeds(t *testing.T) {
	entry := item(wordLeisure, "thời gian rảnh")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: accepted(t)})

	require.NoError(t, uploads.VerifyPending(context.Background()))
	require.Len(t, author.published, 1)

	var body map[string]any
	require.NoError(t, json.Unmarshal(author.published[0].Body, &body))

	// The review screen returns null unless both are present, and a null there
	// is the "this card has no content yet" state.
	assert.Equal(t, "leisure", body["word"])
	assert.NotEmpty(t, body["definition"])
	// The dictionary's IPA and its link to a human recording, not a stored file.
	assert.Equal(t, "/ˈliːʒə(ɹ)/", body["ipa"])
	assert.Equal(t, "https://example.test/leisure.mp3", body["audio_url"])
	// The learner's own note becomes the gloss: it is what they will recognise.
	assert.Equal(t, "thời gian rảnh", body["definition_vi"])
	assert.NotEmpty(t, body["example_sentences"])
	// And the grader can score it.
	assert.Equal(t, "leisure", body["correct_answer"])
}

func TestVerifyPending_RejectsAWordNeitherSourceRecognises(t *testing.T) {
	entry := item("asdfgh", "")
	repo := newUploadRepo(entry)
	refusal, err := json.Marshal(map[string]any{
		keyValid: false, "reason": "That does not look like an English word.",
	})
	require.NoError(t, err)

	uploads, _ := newPipeline(t, repo, &stubDictionary{}, &stubAI{reply: string(refusal)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Contains(t, repo.rejected, entry.ID)
	assert.Contains(t, repo.rejected[entry.ID], "English word",
		"the learner reads this, so it has to say something")
	assert.Empty(t, repo.verified)
}

func TestVerifyPending_TheDictionaryOverrulesTheModelOnExistence(t *testing.T) {
	// A model asked "is this a word" will sometimes say no about a word that is
	// plainly in the dictionary. The dictionary found it, so it exists.
	entry := item(wordLeisure, "")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	refusal, err := json.Marshal(map[string]any{keyValid: false, "reason": "Unsure."})
	require.NoError(t, err)

	uploads, _ := newPipeline(t, repo, dict, &stubAI{reply: string(refusal)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Contains(t, repo.verified, entry.ID)
	assert.Empty(t, repo.rejected)
}

func TestVerifyPending_AnUnreachableDictionaryLeavesTheWordPending(t *testing.T) {
	// The single most important rule here: a network blip must not reject a
	// learner's perfectly good word. It stays pending and the next run retries.
	entry := item(wordLeisure, "")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{err: errors.New("connection refused")}

	uploads, _ := newPipeline(t, repo, dict, &stubAI{reply: accepted(t)})
	require.NoError(t, uploads.VerifyPending(context.Background()),
		"one word's transport failure must not fail the run")

	assert.Empty(t, repo.verified)
	assert.Empty(t, repo.rejected)
	assert.Contains(t, repo.attempts, entry.ID, "the attempt is recorded, so it eventually retires")
}

func TestVerifyPending_AModelThatRefusesToAnswerIsNotAVerdict(t *testing.T) {
	// A refusal or a timeout is a failure to ask, not an answer. Reading it as
	// "not a word" would reject good vocabulary because a provider was busy.
	entry := item(wordLeisure, "")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	uploads, _ := newPipeline(t, repo, dict, &stubAI{err: errors.New("429 rate limited")})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Empty(t, repo.rejected)
	assert.Contains(t, repo.attempts, entry.ID)
}

func TestVerifyPending_WorksWithNoModelAtAll(t *testing.T) {
	// A deployment that has configured no AI still verifies against the
	// dictionary, which answers the question that matters most.
	entry := item(wordLeisure, "")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	uploads, _ := newPipeline(t, repo, dict, nil)
	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Equal(t, "dictionary", repo.verified[entry.ID],
		"the stored verification names what answered, so a dictionary-only one is legible later")
}

func TestVerifyPending_OneBadWordDoesNotCostTheRest(t *testing.T) {
	good, bad := item(wordLeisure, ""), item("asdfgh", "")
	repo := newUploadRepo(bad, good)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	// The model refuses the first and accepts the second.
	uploads, _ := newPipeline(t, repo, dict, &stubAI{reply: accepted(t)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Contains(t, repo.verified, good.ID)
}

func TestVerifyPending_DoesNothingWhenUnconfigured(t *testing.T) {
	// cmd/api builds this module without a dictionary: it stores uploads and
	// the worker verifies them. Running there must be a quiet no-op.
	entry := item(wordLeisure, "")
	repo := newUploadRepo(entry)
	svc := service.New(service.Deps{Repo: repo})
	uploads := service.NewUploads(svc, repo, service.UploadDeps{})

	require.NoError(t, uploads.VerifyPending(context.Background()))
	assert.Empty(t, repo.verified)
	assert.Empty(t, repo.rejected)
	assert.Empty(t, repo.attempts)
}
