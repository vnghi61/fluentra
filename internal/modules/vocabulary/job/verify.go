// Package job implements background workers for the vocabulary module.
package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// VerifyUploadArgs are the River job parameters for verifying a learner's uploaded words.
type VerifyUploadArgs struct {
	UploadID uuid.UUID `json:"upload_id"`
}

// Kind identifies this job type to River.
func (VerifyUploadArgs) Kind() string { return "vocabulary.verify_upload" }

// InsertOpts configures execution limits for the job.
func (VerifyUploadArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "ai",
		MaxAttempts: 3,
	}
}

// UploadVerifier defines the verification operation needed by VerifyUploadWorker.
type UploadVerifier interface {
	VerifyUpload(ctx context.Context, uploadID uuid.UUID) error
}

// VerifyUploadWorker processes uploaded vocabulary verification via River.
type VerifyUploadWorker struct {
	river.WorkerDefaults[VerifyUploadArgs]
	verifier UploadVerifier
}

// NewVerifyUploadWorker constructs a new VerifyUploadWorker.
func NewVerifyUploadWorker(verifier UploadVerifier) *VerifyUploadWorker {
	return &VerifyUploadWorker{verifier: verifier}
}

// Work processes the upload verification job.
func (w *VerifyUploadWorker) Work(ctx context.Context, job *river.Job[VerifyUploadArgs]) error {
	if w.verifier == nil {
		return nil
	}
	return w.verifier.VerifyUpload(ctx, job.Args.UploadID)
}
