package job_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	writingjob "github.com/fluentra/fluentra/internal/modules/writing/job"
)

type fakeSubmissionGrader struct {
	graded []uuid.UUID
	err    error
}

func (f *fakeSubmissionGrader) GradeSubmission(_ context.Context, attemptID uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.graded = append(f.graded, attemptID)
	return nil
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
