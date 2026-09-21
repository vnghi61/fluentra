package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ValidateResourceArgs holds the parameters for validating a resource.
type ValidateResourceArgs struct {
	ResourceID uuid.UUID `json:"resource_id"`
}

// Kind identifies this job type to River.
func (ValidateResourceArgs) Kind() string { return "resource.validate" }

// InsertOpts configures execution limits for the job.
func (ValidateResourceArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "batch",
		MaxAttempts: 3,
	}
}

// ResourceValidator defines the validation operation required by the worker.
type ResourceValidator interface {
	ValidateResource(ctx context.Context, resourceID uuid.UUID) error
}

// ValidateResourceWorker processes asynchronous resource validation via River.
type ValidateResourceWorker struct {
	river.WorkerDefaults[ValidateResourceArgs]
	validator ResourceValidator
}

// NewValidateResourceWorker constructs a new ValidateResourceWorker.
func NewValidateResourceWorker(validator ResourceValidator) *ValidateResourceWorker {
	return &ValidateResourceWorker{validator: validator}
}

// Work executes the validation job.
func (w *ValidateResourceWorker) Work(ctx context.Context, job *river.Job[ValidateResourceArgs]) error {
	if w.validator == nil {
		return nil
	}
	return w.validator.ValidateResource(ctx, job.Args.ResourceID)
}
