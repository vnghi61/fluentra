// Package job implements background workers for the speaking module.
package job

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// GradeRecordingArgs are the River job arguments for evaluating a speaking recording.
type GradeRecordingArgs struct {
	AttemptID uuid.UUID `json:"attempt_id"`
}

// Kind identifies this job type to River.
func (GradeRecordingArgs) Kind() string { return "speaking.grade_recording" }

// InsertOpts configures execution limits for the job.
func (GradeRecordingArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       "ai",
		MaxAttempts: 3,
	}
}

// RecordingGrader defines the grading operation needed by GradeRecordingWorker.
type RecordingGrader interface {
	// GradeSubmission grades one recording. finalAttempt is true when River will not
	// retry the job again, and only then should a failure be recorded on the attempt.
	GradeSubmission(ctx context.Context, attemptID uuid.UUID, finalAttempt bool) error
}

// GradeRecordingWorker processes speaking exercise recordings via River.
type GradeRecordingWorker struct {
	river.WorkerDefaults[GradeRecordingArgs]
	grader RecordingGrader
}

// NewGradeRecordingWorker constructs a new GradeRecordingWorker.
func NewGradeRecordingWorker(grader RecordingGrader) *GradeRecordingWorker {
	return &GradeRecordingWorker{grader: grader}
}

// Work processes the speaking recording evaluation job.
func (w *GradeRecordingWorker) Work(ctx context.Context, job *river.Job[GradeRecordingArgs]) error {
	if w.grader == nil {
		return nil
	}
	finalAttempt := job.JobRow == nil || job.Attempt >= job.MaxAttempts
	return w.grader.GradeSubmission(ctx, job.Args.AttemptID, finalAttempt)
}
