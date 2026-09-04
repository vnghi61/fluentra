package job_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocabjob "github.com/fluentra/fluentra/internal/modules/vocabulary/job"
)

type fakeVerifier struct {
	verified []uuid.UUID
	err      error
}

func (f *fakeVerifier) VerifyUpload(_ context.Context, uploadID uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.verified = append(f.verified, uploadID)
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

func TestVerifyUploadWorker_Work(t *testing.T) {
	uploadID := uuid.New()
	verifier := &fakeVerifier{}
	worker := vocabjob.NewVerifyUploadWorker(verifier)

	job := &river.Job[vocabjob.VerifyUploadArgs]{
		Args: vocabjob.VerifyUploadArgs{
			UploadID: uploadID,
		},
	}

	err := worker.Work(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{uploadID}, verifier.verified)
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
