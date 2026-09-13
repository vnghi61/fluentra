package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// ExpireAttemptArgs holds parameters for the scheduled expiry job.
type ExpireAttemptArgs struct {
	AttemptID uuid.UUID `json:"attempt_id"`
}

// Kind returns the River job kind name.
func (ExpireAttemptArgs) Kind() string { return "exam.expire_attempt" }

// ExamExpirer defines the service interface needed by the worker.
type ExamExpirer interface {
	ExpireAttempt(ctx context.Context, attemptID uuid.UUID) error
}

// ExpireAttemptWorker processes expired exam sittings.
type ExpireAttemptWorker struct {
	river.WorkerDefaults[ExpireAttemptArgs]
	service ExamExpirer
}

// NewExpireAttemptWorker constructs an ExpireAttemptWorker.
func NewExpireAttemptWorker(service ExamExpirer) *ExpireAttemptWorker {
	return &ExpireAttemptWorker{service: service}
}

// Work executes the job.
func (w *ExpireAttemptWorker) Work(ctx context.Context, job *river.Job[ExpireAttemptArgs]) error {
	return w.service.ExpireAttempt(ctx, job.Args.AttemptID)
}
