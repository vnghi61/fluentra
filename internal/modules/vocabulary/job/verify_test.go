package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocabjob "github.com/fluentra/fluentra/internal/modules/vocabulary/job"
)

type fakeVerifier struct {
	verified []uuid.UUID
	itemIDs  [][]uuid.UUID
	err      error
}

func (f *fakeVerifier) VerifyUpload(_ context.Context, uploadID uuid.UUID, itemIDs []uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.verified = append(f.verified, uploadID)
	f.itemIDs = append(f.itemIDs, itemIDs)
	return nil
}

func TestVerifyUploadArgs_JobMetadata(t *testing.T) {
	args := vocabjob.VerifyUploadArgs{
		UploadID: uuid.New(),
	}

	assert.Equal(t, "vocabulary.verify_upload", args.Kind())
	opts := args.InsertOpts()
	assert.Equal(t, "ai", opts.Queue)
	assert.Equal(t, 3, opts.MaxAttempts)
}

func TestInsertOptsAt(t *testing.T) {
	assert.Nil(t, vocabjob.InsertOptsAt(time.Time{}), "a zero time means at once, with the args' own options")

	runAt := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	opts := vocabjob.InsertOptsAt(runAt)
	require.NotNil(t, opts)
	assert.Equal(t, runAt, opts.ScheduledAt)
	assert.Empty(t, opts.Queue, "the queue stays the args' own, which River falls back to")
}

func TestVerifyUploadWorker_Work(t *testing.T) {
	uploadID := uuid.New()
	verifier := &fakeVerifier{}
	worker := vocabjob.NewVerifyUploadWorker(verifier)

	itemIDs := []uuid.UUID{uuid.New()}
	job := &river.Job[vocabjob.VerifyUploadArgs]{
		Args: vocabjob.VerifyUploadArgs{
			UploadID: uploadID,
			ItemIDs:  itemIDs,
		},
	}

	err := worker.Work(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{uploadID}, verifier.verified)
	assert.Equal(t, [][]uuid.UUID{itemIDs}, verifier.itemIDs, "the job verifies only the words it was created for")
}

func TestVerifyUploadWorker_Work_ErrorPropagation(t *testing.T) {
	uploadID := uuid.New()
	wantErr := errors.New("boom")
	verifier := &fakeVerifier{err: wantErr}
	worker := vocabjob.NewVerifyUploadWorker(verifier)

	job := &river.Job[vocabjob.VerifyUploadArgs]{
		Args: vocabjob.VerifyUploadArgs{
			UploadID: uploadID,
		},
	}

	err := worker.Work(context.Background(), job)
	require.ErrorIs(t, err, wantErr)
}
