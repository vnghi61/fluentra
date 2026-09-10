package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
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

// The fields the vocab_verify template asks the model to set, and the fixture
// words. Named because a typo in a repeated literal produces a test that
// passes for the wrong reason.
const (
	keyValid          = "valid"
	keyReason         = "reason"
	keyLemma          = "lemma"
	keyPartOfSpeech   = "part_of_speech"
	keyCEFRLevel      = "cefr_level"
	keyDefinition     = "definition"
	keyDefinitionVi   = "definition_vi"
	keyMeaningMatches = "meaning_matches"
	keyExamples       = "examples"

	keySentence   = "sentence"
	keySentenceVi = "sentence_vi"

	posNoun        = "noun"
	posProperNoun  = "proper noun"
	wordTyme       = "tyme"
	wordTime       = "time"
	wordBook       = "book"
	wordApple      = "apple"
	defWrittenWork = "A written work."
)

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

type stubQuotaAI struct {
	stubAI
	hasQuota bool
}

func (s *stubQuotaAI) HasQuota(_ context.Context, _ ai.Task) (bool, error) {
	return s.hasQuota, nil
}

// uploadRepo records what the pipeline did to each item.
type uploadRepo struct {
	*fakeRepo

	pending                 []sqlc.SkillVocabUploadItem
	verified                map[uuid.UUID]string
	rejected                map[uuid.UUID]string
	attempts                map[uuid.UUID]string
	queued                  map[uuid.UUID]string
	enrichVerified          map[uuid.UUID]string
	enrichRejected          map[uuid.UUID]string
	enrichFailed            map[uuid.UUID]string
	uploads                 map[uuid.UUID]sqlc.SkillVocabUpload
	uploadItems             map[uuid.UUID][]sqlc.ListUploadItemsRow
	sensesNeedingEnrichment []sqlc.ListSensesNeedingExampleEnrichmentRow
	enrichUpdates           []sqlc.UpdateWordSenseEnrichmentParams
}

func newUploadRepo(items ...sqlc.SkillVocabUploadItem) *uploadRepo {
	return &uploadRepo{
		fakeRepo:                newFakeRepo(),
		pending:                 items,
		verified:                map[uuid.UUID]string{},
		rejected:                map[uuid.UUID]string{},
		attempts:                map[uuid.UUID]string{},
		queued:                  map[uuid.UUID]string{},
		enrichVerified:          map[uuid.UUID]string{},
		enrichRejected:          map[uuid.UUID]string{},
		enrichFailed:            map[uuid.UUID]string{},
		uploads:                 map[uuid.UUID]sqlc.SkillVocabUpload{},
		uploadItems:             map[uuid.UUID][]sqlc.ListUploadItemsRow{},
		sensesNeedingEnrichment: []sqlc.ListSensesNeedingExampleEnrichmentRow{},
		enrichUpdates:           []sqlc.UpdateWordSenseEnrichmentParams{},
	}
}

func (r *uploadRepo) ListSensesNeedingExampleEnrichment(
	_ context.Context, _ int32,
) ([]sqlc.ListSensesNeedingExampleEnrichmentRow, error) {
	return r.sensesNeedingEnrichment, nil
}

func (r *uploadRepo) UpdateWordSenseEnrichment(
	_ context.Context, arg sqlc.UpdateWordSenseEnrichmentParams,
) (sqlc.SkillWordSense, error) {
	r.enrichUpdates = append(r.enrichUpdates, arg)
	return sqlc.SkillWordSense{ID: arg.ID}, nil
}

func (r *uploadRepo) GetUpload(ctx context.Context, id, userID uuid.UUID) (sqlc.SkillVocabUpload, error) {
	if u, ok := r.uploads[id]; ok {
		return u, nil
	}
	return r.fakeRepo.GetUpload(ctx, id, userID)
}

func (r *uploadRepo) ListUploadItems(
	ctx context.Context, uploadID, userID uuid.UUID,
) ([]sqlc.ListUploadItemsRow, error) {
	if it, ok := r.uploadItems[uploadID]; ok {
		return it, nil
	}
	return r.fakeRepo.ListUploadItems(ctx, uploadID, userID)
}

func (r *uploadRepo) ClaimPendingUploadItems(
	_ context.Context, _, _ int32,
) ([]sqlc.SkillVocabUploadItem, error) {
	return r.pending, nil
}

// The limit is honoured rather than ignored: a fake that returns everything
// however small the batch is cannot fail when the caller stops passing one.
func (r *uploadRepo) ClaimPendingUploadItemsByUploadID(
	_ context.Context, uploadID uuid.UUID, _, limit int32,
) ([]sqlc.SkillVocabUploadItem, error) {
	var items []sqlc.SkillVocabUploadItem
	for _, it := range r.pending {
		if it.UploadID != uploadID {
			continue
		}
		if limit > 0 && len(items) >= int(limit) {
			break
		}
		items = append(items, it)
	}
	return items, nil
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

func (r *uploadRepo) MarkUploadItemQueued(
	_ context.Context, id uuid.UUID, _ *uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	r.queued[id] = reason
	return sqlc.SkillVocabUploadItem{ID: id, Status: statusQueued}, nil
}

func (r *uploadRepo) ClaimQueuedUploadItems(
	_ context.Context, _, _ int32,
) ([]sqlc.SkillVocabUploadItem, error) {
	return r.pending, nil
}

func (r *uploadRepo) MarkQueuedUploadItemVerified(
	_ context.Context, id uuid.UUID, model, _ string,
) (sqlc.SkillVocabUploadItem, error) {
	r.enrichVerified[id] = model
	return sqlc.SkillVocabUploadItem{ID: id, Status: statusVerified}, nil
}

func (r *uploadRepo) MarkQueuedUploadItemRejected(
	_ context.Context, id uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	r.enrichRejected[id] = reason
	return sqlc.SkillVocabUploadItem{ID: id, Status: "rejected"}, nil
}

func (r *uploadRepo) MarkQueuedUploadItemFailed(
	_ context.Context, id uuid.UUID, reason string,
) (sqlc.SkillVocabUploadItem, error) {
	r.enrichFailed[id] = reason
	return sqlc.SkillVocabUploadItem{ID: id, Status: "failed"}, nil
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
		Lemma: wordLeisure, IPA: "/ˈliːʒə(ɹ)/", PartOfSpeech: posNoun,
		Definition: "Time when one is not working or occupied; free time.",
		AudioURL:   "https://example.test/leisure.mp3",
	}
}

func accepted(t *testing.T) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		keyValid: true, "lemma": wordLeisure, "part_of_speech": posNoun,
		"cefr_level": "B1", "definition": "Free time.",
		"meaning_matches": true,
		// Objects, not strings: v4 asks for each sentence with its Vietnamese,
		// and the flashcard's reveal button has nothing to show without it.
		"examples": []map[string]string{
			{keySentence: "He reads at leisure.", keySentenceVi: "Anh ấy đọc lúc rảnh."},
			{keySentence: "She has little leisure.", keySentenceVi: "Cô ấy có ít thời gian rảnh."},
			{keySentence: "Leisure time is short.", keySentenceVi: "Thời gian rảnh thì ngắn."},
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
		// Submit writes its rows and enqueues its job in one transaction, so a
		// pipeline without a transaction source cannot submit. Supplied here so
		// the fixture is the shape cmd/api actually builds.
		Beginner: stubPool{},
	}), author
}

func newPipelineWithReviews(
	t *testing.T, repo *uploadRepo, dict *stubDictionary, model ai.Client, reviews *spyReviewScheduler,
) (*service.Uploads, *stubContentAuthor) {
	t.Helper()
	author := &stubContentAuthor{}
	svc := service.New(service.Deps{Repo: repo, Reviews: reviews})
	return service.NewUploads(svc, repo, service.UploadDeps{
		Dictionary: dict,
		AI:         model,
		Content:    author,
		AuthorID:   uuid.New(),
		Beginner:   stubPool{},
	}), author
}

type stubEnqueuer struct {
	enqueued []uuid.UUID
	err      error
}

func (e *stubEnqueuer) EnqueueVerifyUploadTx(_ context.Context, _ pgx.Tx, uploadID uuid.UUID) error {
	if e.err != nil {
		return e.err
	}
	e.enqueued = append(e.enqueued, uploadID)
	return nil
}

type fakeTx struct {
	pgx.Tx
	outcome *string
}

func (t fakeTx) Commit(context.Context) error {
	if t.outcome != nil {
		*t.outcome = "commit"
	}
	return nil
}

func (t fakeTx) Rollback(context.Context) error {
	if t.outcome != nil {
		*t.outcome = "rollback"
	}
	return nil
}

// stubPool records how the transaction ended.
//
// It cannot prove the rows disappeared -- uploadRepo has no transaction to undo
// and WithTx hands back itself -- so this pins the half a unit test can reach:
// a failed enqueue rolls back rather than commits. That the rollback discards
// the rows is Postgres's job, and the integration suite's.
type stubPool struct{ outcome *string }

func (p stubPool) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return fakeTx{outcome: p.outcome}, nil
}

func (stubPool) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (r *uploadRepo) WithTx(_ pgx.Tx) repository.Repository {
	return r
}

func (r *uploadRepo) InsertUpload(
	_ context.Context, arg sqlc.InsertUploadParams,
) (sqlc.SkillVocabUpload, error) {
	return sqlc.SkillVocabUpload{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		RawText:   arg.RawText,
		ItemCount: arg.ItemCount,
		Status:    "pending",
	}, nil
}

// -------------------------------------------------------------- submitting

func TestSubmit_RefusesTextWithNoWordsInIt(t *testing.T) {
	repo := newUploadRepo()
	uploads, _ := newPipeline(t, repo, &stubDictionary{}, nil)

	_, err := uploads.Submit(context.Background(), uuid.New(), "---\n42\n\n")
	require.Error(t, err, "a paste of dividers and page numbers is a mistake, not an upload")
}

func TestSubmit_EnqueuesVerificationJobInTx(t *testing.T) {
	repo := newUploadRepo()
	enqueuer := &stubEnqueuer{}
	pool := stubPool{}

	svc := service.New(service.Deps{Repo: repo})
	uploads := service.NewUploads(svc, repo, service.UploadDeps{
		Pool:     pool,
		Beginner: pool,
		Enqueuer: enqueuer,
	})

	userID := uuid.New()
	res, err := uploads.Submit(context.Background(), userID, "leisure - free time\nserendipity")
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)
	assert.Equal(t, 2, res.ItemCount)
	require.Len(t, enqueuer.enqueued, 1)
	assert.Equal(t, res.ID, enqueuer.enqueued[0])
}

type stubNudger struct {
	nudged int
}

func (n *stubNudger) Nudge(context.Context) {
	n.nudged++
}

func TestSubmit_NudgesWorkerAfterCommit(t *testing.T) {
	repo := newUploadRepo()
	enqueuer := &stubEnqueuer{}
	nudger := &stubNudger{}
	pool := stubPool{}

	svc := service.New(service.Deps{Repo: repo})
	uploads := service.NewUploads(svc, repo, service.UploadDeps{
		Pool:     pool,
		Beginner: pool,
		Enqueuer: enqueuer,
		Nudger:   nudger,
	})

	userID := uuid.New()
	_, err := uploads.Submit(context.Background(), userID, "leisure - free time\nserendipity")
	require.NoError(t, err)
	assert.Equal(t, 1, nudger.nudged)
}

func TestSubmit_RollsBackWhenTheJobCannotBeEnqueued(t *testing.T) {
	// The transaction exists for this case and no other. Words stored with no
	// job to collect them wait for the hourly sweep with nothing telling the
	// learner why, and that is the outcome the rollback removes.
	outcome := ""
	pool := stubPool{outcome: &outcome}
	repo := newUploadRepo()

	svc := service.New(service.Deps{Repo: repo})
	uploads := service.NewUploads(svc, repo, service.UploadDeps{
		Pool:     pool,
		Beginner: pool,
		Enqueuer: &stubEnqueuer{err: errors.New("river is unreachable")},
	})

	_, err := uploads.Submit(context.Background(), uuid.New(), "leisure - free time")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "river is unreachable",
		"the reason the submission failed must reach the caller")
	assert.Equal(t, "rollback", outcome, "a failed enqueue must not commit the words")
}

func TestSubmit_RefusedWithoutATransactionSource(t *testing.T) {
	repo := newUploadRepo()
	svc := service.New(service.Deps{Repo: repo})
	uploads := service.NewUploads(svc, repo, service.UploadDeps{})

	_, err := uploads.Submit(context.Background(), uuid.New(), "leisure - free time")

	require.Error(t, err, "storing words outside a transaction is not a supported shape")
}

func TestVerifyUpload_VerifiesPendingWordsForSpecificUpload(t *testing.T) {
	uploadID1 := uuid.New()
	uploadID2 := uuid.New()

	item1 := item(wordLeisure, "thời gian rảnh")
	item1.UploadID = uploadID1
	item2 := item("ephemeral", "lasting for a very short time")
	item2.UploadID = uploadID2

	repo := newUploadRepo(item1, item2)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: accepted(t)})

	require.NoError(t, uploads.VerifyUpload(context.Background(), uploadID1))

	assert.Contains(t, repo.verified, item1.ID)
	assert.NotContains(t, repo.verified, item2.ID, "items from other uploads should not be touched")
	require.Len(t, author.published, 1)
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
	assert.Equal(t, wordLeisure, body["word"])
	assert.NotEmpty(t, body["definition"])
	// The dictionary's IPA and its link to a human recording, not a stored file.
	assert.Equal(t, "/ˈliːʒə(ɹ)/", body["ipa"])
	assert.Equal(t, "https://example.test/leisure.mp3", body["audio_url"])
	// The learner's own note becomes the gloss: it is what they will recognise.
	assert.Equal(t, "thời gian rảnh", body["definition_vi"])
	assert.NotEmpty(t, body["example_sentences"])
	// And the grader can score it.
	assert.Equal(t, wordLeisure, body["correct_answer"])
}

func TestVerifyPending_RejectsAWordNeitherSourceRecognises(t *testing.T) {
	entry := item("asdfgh", "")
	repo := newUploadRepo(entry)
	refusal, err := json.Marshal(map[string]any{
		keyValid: false, keyReason: "That does not look like an English word.",
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

func TestVerifyPending_QuotaExhausted_MarksQueuedAndFlashcardCreated(t *testing.T) {
	entry := item(wordLeisure, "time off")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	uploads, author := newPipeline(t, repo, dict, &stubAI{err: ai.ErrQuotaExhausted})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	// Item must be marked queued, not verified and not rejected.
	assert.Contains(t, repo.queued, entry.ID)
	assert.Empty(t, repo.verified)
	assert.Empty(t, repo.rejected)

	// Flashcard must be created and published so learner can review.
	require.NotEmpty(t, author.published)
	assert.Equal(t, "", author.published[0].CEFRLevel,
		"queued word must not fabricate B1 level when model was never asked")
}

func TestEnrichQueued_YieldsWhenNoQuota(t *testing.T) {
	entry := item(wordLeisure, "")
	entry.Status = statusQueued
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	model := &stubQuotaAI{
		stubAI:   stubAI{reply: accepted(t)},
		hasQuota: false,
	}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.EnrichQueued(context.Background()))
	assert.Equal(t, 0, model.calls, "must yield immediately without calling model")
	assert.Empty(t, repo.enrichVerified)
}

func TestEnrichQueued_Success(t *testing.T) {
	entry := item(wordLeisure, "")
	entry.Status = statusQueued
	senseID := uuid.New()
	entry.WordSenseID = &senseID
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	model := &stubQuotaAI{
		stubAI:   stubAI{reply: accepted(t)},
		hasQuota: true,
	}
	uploads, author := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.EnrichQueued(context.Background()))
	assert.Contains(t, repo.enrichVerified, entry.ID)
	assert.NotEmpty(t, author.published)
}

func TestEnrichQueued_QuotaExhausted_BreaksLoop(t *testing.T) {
	entry1 := item(wordLeisure, "")
	entry1.Status = statusQueued
	entry2 := item("bank", "")
	entry2.Status = statusQueued
	repo := newUploadRepo(entry1, entry2)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
		"bank":      leisureEntry(),
	}}

	model := &stubQuotaAI{
		stubAI:   stubAI{err: ai.ErrQuotaExhausted},
		hasQuota: true,
	}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.EnrichQueued(context.Background()))
	assert.Equal(t, 1, model.calls, "should break loop on first quota exhaustion")
	assert.Empty(t, repo.enrichFailed, "quota exhaustion should not fail the item")
}

func TestEnrichQueued_MaxAttempts_TransitionsToFailed(t *testing.T) {
	entry := item(wordLeisure, "")
	entry.Status = statusQueued
	entry.Attempts = 2 // will become attempt 3
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	model := &stubQuotaAI{
		stubAI:   stubAI{err: errors.New("500 upstream model failure")},
		hasQuota: true,
	}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.EnrichQueued(context.Background()))
	assert.Contains(t, repo.enrichFailed, entry.ID, "3rd failed attempt must transition to failed status")
}

func TestEnrichQueued_InvalidWord_MarksRejected(t *testing.T) {
	entry := item("notawordxyz", "")
	entry.Status = statusQueued
	repo := newUploadRepo(entry)
	dict := &stubDictionary{}

	invalidReply, err := json.Marshal(map[string]any{
		"valid":   false,
		keyReason: "Not an English word.",
	})
	require.NoError(t, err)

	model := &stubQuotaAI{
		stubAI:   stubAI{reply: string(invalidReply)},
		hasQuota: true,
	}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.EnrichQueued(context.Background()))
	assert.Contains(t, repo.enrichRejected, entry.ID, "invalid word must be rejected")
}

func TestVerify_AWordPastedWithNoMeaningStillGetsVietnamese(t *testing.T) {
	// The case that matters: a bare list. Nobody typing thirty words writes a
	// gloss for each, and before the model was asked for one, definition_vi was
	// left null and the flashcard showed the learner nothing in their own
	// language — for most of what they paste.
	repo := newUploadRepo(item(wordTime, ""))
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordTime: {Lemma: "time", PartOfSpeech: posNoun, Definition: "The indefinite continued progress of existence."},
	}}
	reply := map[string]any{
		keyValid: true, "reason": "", keyLemma: "time", keyPartOfSpeech: posNoun,
		keyCEFRLevel: "A1", keyDefinition: "The indefinite continued progress of existence.",
		keyDefinitionVi: "thời gian", keyMeaningMatches: true,
		keyExamples: []map[string]string{
			{keySentence: "We do not have much time.", keySentenceVi: "Chúng ta không có nhiều thời gian."},
		},
	}
	body, err := json.Marshal(reply)
	require.NoError(t, err)

	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: string(body)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	require.Len(t, author.published, 1)
	var published map[string]any
	require.NoError(t, json.Unmarshal(author.published[0].Body, &published))
	assert.Equal(t, "thời gian", published["definition_vi"])
}

func TestVerify_TheLearnersOwnWordingWinsOverTheModels(t *testing.T) {
	// Their note is what they will recognise, and it is theirs. The model's
	// gloss is the fallback, not the correction.
	repo := newUploadRepo(item(wordBook, "quyển sách của tôi"))
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordBook: {Lemma: wordBook, PartOfSpeech: posNoun, Definition: defWrittenWork},
	}}
	reply := map[string]any{
		keyValid: true, keyLemma: wordBook, keyPartOfSpeech: posNoun, keyCEFRLevel: "A1",
		keyDefinition: defWrittenWork, keyDefinitionVi: "sách",
		keyMeaningMatches: true,
		keyExamples: []map[string]string{
			{keySentence: "She read the book.", keySentenceVi: "Cô ấy đã đọc quyển sách."},
		},
	}
	body, err := json.Marshal(reply)
	require.NoError(t, err)

	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: string(body)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	require.Len(t, author.published, 1)
	var published map[string]any
	require.NoError(t, json.Unmarshal(author.published[0].Body, &published))
	assert.Equal(t, "quyển sách của tôi", published["definition_vi"])
}

func TestVerify_ModelTopicCreatesTopicDeck(t *testing.T) {
	entry := item(wordApple, "")
	repo := newUploadRepo(entry)
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordApple: {Lemma: wordApple, PartOfSpeech: posNoun, Definition: "A round fruit."},
	}}
	reply := map[string]any{
		keyValid: true, keyLemma: wordApple, keyPartOfSpeech: posNoun, keyCEFRLevel: "A1",
		keyDefinition: "A round fruit.", keyDefinitionVi: "quả táo",
		"topic": "food", keyMeaningMatches: true,
		keyExamples: []map[string]string{
			{keySentence: "She ate an apple.", keySentenceVi: "Cô ấy đã ăn một quả táo."},
		},
	}
	body, err := json.Marshal(reply)
	require.NoError(t, err)

	uploads, _ := newPipeline(t, repo, dict, &stubAI{reply: string(body)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	// Assert that a deck with slug "my-words-food" was created
	foundFoodDeck := false
	for _, deck := range repo.decks {
		if deck.Slug == "my-words-food" {
			foundFoodDeck = true
			assert.Equal(t, "My words: Food", deck.Name)
			break
		}
	}
	assert.True(t, foundFoodDeck, "deck my-words-food should be created for food words")
}

func TestUpload_GetIncludesEnrichedDetails(t *testing.T) {
	uploadID := uuid.New()
	userID := uuid.New()
	def := "A written work."
	defVi := "sách"
	topic := "study"
	type exItem struct {
		Sentence string `json:"sentence"`
	}
	exBytes, _ := json.Marshal([]exItem{{Sentence: "She read a book."}})

	repo := newUploadRepo()
	repo.uploads[uploadID] = sqlc.SkillVocabUpload{
		ID:        uploadID,
		UserID:    userID,
		ItemCount: 1,
		Status:    "completed",
	}
	repo.uploadItems[uploadID] = []sqlc.ListUploadItemsRow{
		{
			ID:           uuid.New(),
			UploadID:     uploadID,
			UserID:       userID,
			Term:         "book",
			Status:       "verified",
			Definition:   &def,
			DefinitionVi: &defVi,
			Topic:        &topic,
			Examples:     exBytes,
		},
	}

	uploads, _ := newPipeline(t, repo, &stubDictionary{}, &stubAI{})
	res, err := uploads.Get(context.Background(), userID, uploadID)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)

	assert.Equal(t, "book", res.Items[0].Term)
	assert.Equal(t, def, res.Items[0].Definition)
	assert.Equal(t, defVi, res.Items[0].DefinitionVi)
	assert.Equal(t, topic, res.Items[0].Topic)
	require.Len(t, res.Items[0].Examples, 1)
	assert.Equal(t, "She read a book.", res.Items[0].Examples[0])
}

func TestVerify_ANameIsNotAWord(t *testing.T) {
	// "tyme" is a misspelling of "time" that the free dictionaries carry as a
	// male given name. It reached a learner's deck with an IPA, five example
	// sentences and a review card. Refused here rather than left to the model,
	// because the part of speech is a value we already hold.
	repo := newUploadRepo(item(wordTyme, ""))
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordTyme: {Lemma: wordTyme, PartOfSpeech: posProperNoun, Definition: "A male given name."},
	}}
	reply := map[string]any{
		keyValid: true, keyLemma: wordTyme, keyPartOfSpeech: posProperNoun,
		keyCEFRLevel: "C1", keyDefinition: "A male given name.",
		keyDefinitionVi: "tên riêng", keyMeaningMatches: true,
		keyExamples: []map[string]string{{keySentence: "Tyme arrived.", keySentenceVi: "Tyme đã đến."}},
	}
	body, err := json.Marshal(reply)
	require.NoError(t, err)

	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: string(body)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	assert.Contains(t, repo.rejected, repo.pending[0].ID, "a given name is not a vocabulary entry")
	assert.Empty(t, author.published, "nothing should be published for a name")
}

func TestVerify_ExampleSentencesCarryTheirTranslation(t *testing.T) {
	// The flashcard has a reveal button under each sentence, and
	// domain.ExampleSentence has carried SentenceVi since the table was
	// written. Until v4 asked for it, every learner-added word gave that
	// button nothing to show.
	repo := newUploadRepo(item(wordLeisure, ""))
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	uploads, author := newPipeline(t, repo, dict, &stubAI{reply: accepted(t)})
	require.NoError(t, uploads.VerifyPending(context.Background()))

	require.Len(t, author.published, 1)
	var published map[string]any
	require.NoError(t, json.Unmarshal(author.published[0].Body, &published))

	sentences, ok := published["example_sentences"].([]any)
	require.True(t, ok, "the body must carry example_sentences")
	require.NotEmpty(t, sentences)

	first, ok := sentences[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "He reads at leisure.", first[keySentence])
	assert.Equal(t, "Anh ấy đọc lúc rảnh.", first[keySentenceVi],
		"web/src/lib/examples.ts reads sentence_vi; without it the reveal button stays hidden")
}

func TestEnrichExamples_ExpandsUpTo15AndRepoints(t *testing.T) {
	oldVersionID := uuid.New()
	senseID := uuid.New()
	existingEx, err := json.Marshal([]domain.ExampleSentence{
		{Sentence: "Existing sentence one."},
		{Sentence: "Existing sentence two."},
		{Sentence: "Existing sentence three."},
		{Sentence: "Existing sentence four."},
		{Sentence: "Existing sentence five."},
	})
	require.NoError(t, err)

	repo := newUploadRepo()
	repo.sensesNeedingEnrichment = []sqlc.ListSensesNeedingExampleEnrichmentRow{
		{
			ID:               senseID,
			WordID:           uuid.New(),
			ContentVersionID: &oldVersionID,
			Definition:       defLeisure,
			Examples:         existingEx,
			Lemma:            wordLeisure,
			Pos:              posNoun,
			CefrLevel:        "B1",
		},
	}

	modelReply := `{"examples": [
		{"sentence": "New sentence six.", "sentence_vi": "Câu mới sáu."},
		{"sentence": "New sentence seven.", "sentence_vi": "Câu mới bảy."},
		{"sentence": "New sentence eight.", "sentence_vi": "Câu mới tám."},
		{"sentence": "New sentence nine.", "sentence_vi": "Câu mới chín."},
		{"sentence": "New sentence ten.", "sentence_vi": "Câu mới mười."}
	]}`

	spyReviews := &spyReviewScheduler{}
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}

	uploads, author := newPipelineWithReviews(t, repo, dict, &stubAI{reply: modelReply}, spyReviews)

	err = uploads.EnrichExamples(context.Background())
	require.NoError(t, err)

	require.Len(t, repo.enrichUpdates, 1, "must update sense enrichment")
	update := repo.enrichUpdates[0]
	assert.Equal(t, senseID, update.ID)
	assert.NotNil(t, update.ContentVersionID)
	assert.NotEqual(t, oldVersionID, *update.ContentVersionID, "must have a new content version")

	var updatedEx []domain.ExampleSentence
	require.NoError(t, json.Unmarshal(update.Examples, &updatedEx))
	assert.Len(t, updatedEx, 10, "5 existing + 5 new = 10 examples")

	require.Len(t, author.published, 1, "must republish content version")
	assert.Equal(t, author.id, *update.ContentVersionID)

	assert.Equal(t, 1, spyReviews.repointCalls, "must call srs.RepointCards")
	assert.Equal(t, oldVersionID, spyReviews.repointedOld)
	assert.Equal(t, author.id, spyReviews.repointedNew)
}

func TestEnrichExamples_SkipsDuplicates(t *testing.T) {
	oldVersionID := uuid.New()
	senseID := uuid.New()
	existingEx, err := json.Marshal([]domain.ExampleSentence{
		{Sentence: "He reads at leisure."},
	})
	require.NoError(t, err)

	repo := newUploadRepo()
	repo.sensesNeedingEnrichment = []sqlc.ListSensesNeedingExampleEnrichmentRow{
		{
			ID:               senseID,
			WordID:           uuid.New(),
			ContentVersionID: &oldVersionID,
			Definition:       defLeisure,
			Examples:         existingEx,
			Lemma:            wordLeisure,
			Pos:              posNoun,
			CefrLevel:        "B1",
		},
	}

	// Model returns duplicate of sentence 1, plus 1 distinct sentence.
	modelReply := `{"examples": [
		{"sentence": "  he reads at LEISURE.  ", "sentence_vi": "Anh ấy đọc lúc rảnh."},
		{"sentence": "A unique leisure activity.", "sentence_vi": "Một hoạt động giải trí độc đáo."}
	]}`

	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{"leisure": leisureEntry()}}
	uploads, _ := newPipelineWithReviews(t, repo, dict, &stubAI{reply: modelReply}, &spyReviewScheduler{})

	err = uploads.EnrichExamples(context.Background())
	require.NoError(t, err)

	require.Len(t, repo.enrichUpdates, 1)
	var updatedEx []domain.ExampleSentence
	require.NoError(t, json.Unmarshal(repo.enrichUpdates[0].Examples, &updatedEx))
	assert.Len(t, updatedEx, 2, "duplicate must be skipped; only 1 new example added")
}

func TestEnrichExamples_YieldsOnQuotaExhausted(t *testing.T) {
	oldVersionID := uuid.New()
	senseID := uuid.New()
	existingEx, err := json.Marshal([]domain.ExampleSentence{
		{Sentence: "Sentence 1."},
	})
	require.NoError(t, err)

	repo := newUploadRepo()
	repo.sensesNeedingEnrichment = []sqlc.ListSensesNeedingExampleEnrichmentRow{
		{
			ID:               senseID,
			WordID:           uuid.New(),
			ContentVersionID: &oldVersionID,
			Definition:       defLeisure,
			Examples:         existingEx,
			Lemma:            wordLeisure,
			Pos:              posNoun,
			CefrLevel:        "B1",
		},
	}

	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{"leisure": leisureEntry()}}
	uploads, _ := newPipelineWithReviews(t, repo, dict, &stubAI{err: ai.ErrQuotaExhausted}, &spyReviewScheduler{})

	err = uploads.EnrichExamples(context.Background())
	require.NoError(t, err, "must yield cleanly without error when quota exhausted")
	assert.Empty(t, repo.enrichUpdates, "no updates made when quota exhausted")
}

// TestEnrichExamples_KeepsThePronunciation.
//
// The job republishes the whole sense body to add sentences to it, so every
// field the flashcard renders has to be carried across. The first version built
// the body from the columns it happened to have and passed an empty dictionary
// entry, which dropped `ipa` and `audio_url` from every word the sweep touched —
// a word gained sentences and silently lost its pronunciation.
func TestEnrichExamples_KeepsThePronunciation(t *testing.T) {
	oldVersionID := uuid.New()
	ipa := "/ˈliːʒə(ɹ)/"
	existingEx, err := json.Marshal([]domain.ExampleSentence{
		{Sentence: "Existing sentence one."},
	})
	require.NoError(t, err)

	repo := newUploadRepo()
	repo.sensesNeedingEnrichment = []sqlc.ListSensesNeedingExampleEnrichmentRow{
		{
			ID:               uuid.New(),
			WordID:           uuid.New(),
			ContentVersionID: &oldVersionID,
			Definition:       defLeisure,
			Examples:         existingEx,
			Lemma:            wordLeisure,
			Pos:              posNoun,
			CefrLevel:        "B1",
			Ipa:              &ipa,
		},
	}

	modelReply := `{"examples": [{"sentence": "A brand new sentence.", "sentence_vi": "Câu mới."}]}`
	dict := &stubDictionary{entries: map[string]repository.DictionaryEntry{
		wordLeisure: leisureEntry(),
	}}
	uploads, author := newPipelineWithReviews(
		t, repo, dict, &stubAI{reply: modelReply}, &spyReviewScheduler{},
	)

	require.NoError(t, uploads.EnrichExamples(context.Background()))
	require.Len(t, author.published, 1, "must republish the sense")

	var body map[string]any
	require.NoError(t, json.Unmarshal(author.published[0].Body, &body))
	assert.Equal(t, ipa, body["ipa"], "the republished body must keep the pronunciation")

	// And the sentences it was republished for are there too, so this cannot
	// pass by the job having done nothing.
	sentences, ok := body["example_sentences"].([]any)
	require.True(t, ok, "body carries example sentences")
	assert.Len(t, sentences, 2)
}
