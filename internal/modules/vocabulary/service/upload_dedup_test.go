package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/service"
)

// Two learners pasting the same word pay for it once, and an upload larger than
// one batch is finished by its own job rather than by the hourly sweep.

func TestVerifyUpload_ReusesAnotherLearnersRejection(t *testing.T) {
	repo := newUploadRepo()
	note, suggestion := "spelling_suggestion", "school"
	repo.rejections = map[string]sqlc.FindRecentTermRejectionRow{
		"shcool": {Reason: `Did you mean "school"?`, SuggestedTerm: &suggestion, NoteCode: &note},
	}
	entry := item("shcool", "")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{}
	model := &stubAI{err: assert.AnError}
	uploads, _ := newPipeline(t, repo, dict, model)

	require.NoError(t, uploads.VerifyUpload(context.Background(), entry.UploadID, nil))

	assert.Equal(t, 0, dict.calls, "an earlier verdict on the word costs no dictionary call")
	assert.Equal(t, 0, model.calls, "an earlier verdict on the word costs no model call")
	assert.Contains(t, repo.rejected, entry.ID)
	assert.Equal(t, note, repo.noteCodes[entry.ID])
	assert.Equal(t, suggestion, repo.suggestedTerms[entry.ID])
}

func TestVerifyUpload_DoesNotReuseARejectionWhenTheLearnerGaveAMeaning(t *testing.T) {
	repo := newUploadRepo()
	note := "not_a_word"
	repo.rejections = map[string]sqlc.FindRecentTermRejectionRow{
		"shcool": {Reason: "Not a word.", NoteCode: &note},
	}
	entry := item("shcool", "trường học")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}

	dict := &stubDictionary{}
	uploads, _ := newPipeline(t, repo, dict, &stubAI{err: assert.AnError})

	_ = uploads.VerifyUpload(context.Background(), entry.UploadID, nil)

	assert.Positive(t, dict.calls, "a gloss can change the verdict, so the word is checked afresh")
}

func TestVerifyUpload_LocksTheTermItChecks(t *testing.T) {
	repo := newUploadRepo()
	entry := item("  Serendipity ", "")
	repo.pending = []sqlc.SkillVocabUploadItem{entry}
	uploads, _ := newPipeline(t, repo, &stubDictionary{}, &stubAI{err: assert.AnError})

	_ = uploads.VerifyUpload(context.Background(), entry.UploadID, nil)

	assert.Equal(t, []string{"serendipity"}, repo.lockedTerms,
		"the lock is on the normalised term, so every learner's spelling of it shares one")
}

func TestVerifyUpload_ClaimsOnlyTheWordsItWasGiven(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	mine, other := item(wordLeisure, ""), item(wordLeisure, "")
	other.UploadID = mine.UploadID
	repo.pending = []sqlc.SkillVocabUploadItem{mine, other}
	uploads, _ := newPipeline(t, repo, &stubDictionary{}, &stubAI{err: assert.AnError})

	require.NoError(t, uploads.VerifyUpload(context.Background(), mine.UploadID, []uuid.UUID{mine.ID}))

	assert.Contains(t, repo.verified, mine.ID)
	assert.NotContains(t, repo.verified, other.ID, "Submit owns the words its job was not given")
}

func TestVerifyUpload_FinishesAnUploadLargerThanOneBatch(t *testing.T) {
	repo := newUploadRepo()
	seedKnownWord(repo, wordLeisure, "thời gian rảnh")
	uploadID := uuid.New()
	for range 120 {
		entry := item(wordLeisure, "")
		entry.UploadID = uploadID
		repo.pending = append(repo.pending, entry)
	}
	uploads, _ := newPipeline(t, repo, &stubDictionary{}, &stubAI{err: assert.AnError})

	require.NoError(t, uploads.VerifyUpload(context.Background(), uploadID, nil))

	assert.Len(t, repo.verified, 120, "a hundred and twenty words are three batches, all of them this job's")
}

// TestVerifyUpload_TriesAFailingWordOncePerRun: a failed word stays pending and
// the next claim returns it first. Retrying it there and then would spend all
// its attempts in one second; it is left to River's backoff instead.
func TestVerifyUpload_TriesAFailingWordOncePerRun(t *testing.T) {
	repo := newUploadRepo()
	uploadID := uuid.New()
	for range 60 {
		entry := item("serendipity", "")
		entry.UploadID = uploadID
		repo.pending = append(repo.pending, entry)
	}
	dict := &stubDictionary{err: assert.AnError}
	uploads, _ := newPipeline(t, repo, dict, &stubAI{err: assert.AnError})

	err := uploads.VerifyUpload(context.Background(), uploadID, nil)

	require.Error(t, err, "words left to retry hand the job back to River")
	assert.Equal(t, 60, dict.calls, "each word is tried once in a run")
}

func TestSearchItems_RefusesAnUnknownStatus(t *testing.T) {
	uploads, _ := newPipeline(t, newUploadRepo(), &stubDictionary{}, nil)

	_, err := uploads.SearchItems(context.Background(), uuid.New(), service.ItemQuery{
		Statuses: []string{"deleted"}, Limit: 20,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_STATUS")
}

func TestSearchItems_RefusesACursorItDidNotIssue(t *testing.T) {
	uploads, _ := newPipeline(t, newUploadRepo(), &stubDictionary{}, nil)

	_, err := uploads.SearchItems(context.Background(), uuid.New(), service.ItemQuery{
		Cursor: "not-a-cursor", Limit: 20,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_CURSOR")
}
