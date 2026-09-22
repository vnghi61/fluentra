package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ClassifyResourceArgs holds the parameters for classifying extracted resource text.
type ClassifyResourceArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

// Kind identifies this job type to River.
func (ClassifyResourceArgs) Kind() string { return "resource.classify" }

// InsertOpts configures execution limits for the classification job.
func (ClassifyResourceArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "ai",
		MaxAttempts: 3,
	}
}

// ResourceClassifier defines the classification operation required by the worker.
type ResourceClassifier interface {
	ClassifyResource(ctx context.Context, resourceID uuid.UUID) error
}

// ClassifyResourceWorker processes asynchronous resource classification via River.
type ClassifyResourceWorker struct {
	river.WorkerDefaults[ClassifyResourceArgs]
	classifier ResourceClassifier
}

// NewClassifyResourceWorker constructs a new ClassifyResourceWorker.
func NewClassifyResourceWorker(classifier ResourceClassifier) *ClassifyResourceWorker {
	return &ClassifyResourceWorker{classifier: classifier}
}

// Work executes the classification job.
func (w *ClassifyResourceWorker) Work(ctx context.Context, job *river.Job[ClassifyResourceArgs]) error {
	if w.classifier == nil {
		return nil
	}
	return w.classifier.ClassifyResource(ctx, job.Args.ResourceID)
}
