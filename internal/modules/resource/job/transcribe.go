package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// TranscribeResourceArgs holds the parameters for transcribing an audio/video resource.
type TranscribeResourceArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

// Kind identifies this job type to River.
func (TranscribeResourceArgs) Kind() string { return "resource.transcribe" }

// InsertOpts configures execution limits for the transcription job.
func (TranscribeResourceArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "media",
		MaxAttempts: 3,
	}
}

// ResourceTranscriber defines the transcription operation required by the worker.
type ResourceTranscriber interface {
	TranscribeResource(ctx context.Context, resourceID uuid.UUID) error
}

// TranscribeResourceWorker processes asynchronous audio/video transcription via River.
type TranscribeResourceWorker struct {
	river.WorkerDefaults[TranscribeResourceArgs]
	transcriber ResourceTranscriber
}

// NewTranscribeResourceWorker constructs a new TranscribeResourceWorker.
func NewTranscribeResourceWorker(transcriber ResourceTranscriber) *TranscribeResourceWorker {
	return &TranscribeResourceWorker{transcriber: transcriber}
}

// Work executes the transcription job.
func (w *TranscribeResourceWorker) Work(ctx context.Context, job *river.Job[TranscribeResourceArgs]) error {
	if w.transcriber == nil {
		return nil
	}
	return w.transcriber.TranscribeResource(ctx, job.Args.ResourceID)
}
