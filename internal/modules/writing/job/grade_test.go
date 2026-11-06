package job_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	writingjob "github.com/fluentra/fluentra/internal/modules/writing/job"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

type fakeSubmissionGrader struct {
	graded []uuid.UUID
	finals []bool
	err    error
}

func (f *fakeSubmissionGrader) GradeSubmission(_ context.Context, attemptID uuid.UUID, finalAttempt bool) error {
	f.finals = append(f.finals, finalAttempt)
	if f.err != nil {
		return f.err
	}
	f.graded = append(f.graded, attemptID)
	return nil
}

// TestGradeSubmissionWorker_OnlyTheLastAttemptIsFinal is what lets a transient
// provider error be retried. The grader records a failure only when told the run
// is final; with every run final, the first error failed the attempt and each
// retry found nothing left to grade.
func TestGradeSubmissionWorker_OnlyTheLastAttemptIsFinal(t *testing.T) {
	for _, tc := range []struct {
		attempt, maxAttempts int
		wantFinal            bool
	}{
		{attempt: 1, maxAttempts: 3, wantFinal: false},
		{attempt: 2, maxAttempts: 3, wantFinal: false},
		{attempt: 3, maxAttempts: 3, wantFinal: true},
	} {
		grader := &fakeSubmissionGrader{}
		worker := writingjob.NewGradeSubmissionWorker(grader)
		job := &river.Job[writingjob.GradeSubmissionArgs]{
			JobRow: &rivertype.JobRow{Attempt: tc.attempt, MaxAttempts: tc.maxAttempts},
			Args:   writingjob.GradeSubmissionArgs{AttemptID: uuid.New()},
		}
		require.NoError(t, worker.Work(context.Background(), job))
		assert.Equal(t, []bool{tc.wantFinal}, grader.finals,
			"attempt %d of %d", tc.attempt, tc.maxAttempts)
	}
}

func TestGradeSubmissionArgs_JobMetadata(t *testing.T) {
	args := writingjob.GradeSubmissionArgs{
		AttemptID: uuid.New(),
	}

	assert.Equal(t, "writing.grade_submission", args.Kind())
	opts := args.InsertOpts()
	assert.Equal(t, "ai", opts.Queue)
	assert.Equal(t, 3, opts.MaxAttempts)
}

func TestGradeSubmissionWorker_Work(t *testing.T) {
	attemptID := uuid.New()
	grader := &fakeSubmissionGrader{}
	worker := writingjob.NewGradeSubmissionWorker(grader)

	job := &river.Job[writingjob.GradeSubmissionArgs]{
		Args: writingjob.GradeSubmissionArgs{
			AttemptID: attemptID,
		},
	}

	err := worker.Work(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{attemptID}, grader.graded)
}

func TestGradeSubmissionWorker_Work_ErrorPropagation(t *testing.T) {
	attemptID := uuid.New()
	wantErr := errors.New("boom")
	grader := &fakeSubmissionGrader{err: wantErr}
	worker := writingjob.NewGradeSubmissionWorker(grader)

	job := &river.Job[writingjob.GradeSubmissionArgs]{
		Args: writingjob.GradeSubmissionArgs{
			AttemptID: attemptID,
		},
	}

	err := worker.Work(context.Background(), job)
	require.ErrorIs(t, err, wantErr)
}

// A rejected model reply cancels the job: the grader already failed the
// attempt, and a retry would pay for the same reply again.
func TestGradeSubmissionWorker_CancelsOnRejectedOutput(t *testing.T) {
	grader := &fakeSubmissionGrader{err: fmt.Errorf("grade: %w", ai.ErrRejectedOutput)}
	worker := writingjob.NewGradeSubmissionWorker(grader)
	job := &river.Job[writingjob.GradeSubmissionArgs]{
		JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3},
		Args:   writingjob.GradeSubmissionArgs{AttemptID: uuid.New()},
	}

	err := worker.Work(context.Background(), job)
	var cancel *river.JobCancelError
	require.True(t, errors.As(err, &cancel), "want a JobCancel, got %v", err)
}
