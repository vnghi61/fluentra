// Package job implements background workers for the writing module.
package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// GradeSubmissionArgs are the River job parameters for grading a writing submission.
type GradeSubmissionArgs struct {
	AttemptID uuid.UUID `json:"attempt_id"`
}

// Kind identifies this job type to River.
func (GradeSubmissionArgs) Kind() string { return "writing.grade_submission" }

// InsertOpts configures execution limits for the job.
func (GradeSubmissionArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "ai",
		MaxAttempts: 3,
	}
}

// SubmissionGrader defines the grading operation needed by GradeSubmissionWorker.
type SubmissionGrader interface {
	// GradeSubmission grades one essay. finalAttempt is true when River will not
	// run the job again, and only then may a failure be recorded on the attempt.
	GradeSubmission(ctx context.Context, attemptID uuid.UUID, finalAttempt bool) error
}

// GradeSubmissionWorker processes writing exercise submissions via River.
type GradeSubmissionWorker struct {
	river.WorkerDefaults[GradeSubmissionArgs]
	grader SubmissionGrader
}

// NewGradeSubmissionWorker constructs a new GradeSubmissionWorker.
func NewGradeSubmissionWorker(grader SubmissionGrader) *GradeSubmissionWorker {
	return &GradeSubmissionWorker{grader: grader}
}

// Work processes the writing submission grading job.
func (w *GradeSubmissionWorker) Work(ctx context.Context, job *river.Job[GradeSubmissionArgs]) error {
	if w.grader == nil {
		return nil
	}
	// River always supplies the row. A job built without one — in a test — is
	// treated as its last attempt, which is the conservative reading.
	finalAttempt := job.JobRow == nil || job.Attempt >= job.MaxAttempts
	return w.grader.GradeSubmission(ctx, job.Args.AttemptID, finalAttempt)
}
