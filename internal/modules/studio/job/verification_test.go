package job_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/modules/studio/job"
)

type fakeSubmissionsRepo struct {
	pending []*domain.Submission
	err     error
}

func (f *fakeSubmissionsRepo) ListPendingVerificationSubmissions(
	_ context.Context, _ int32,
) ([]*domain.Submission, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.pending, nil
}

type fakeVerificationService struct {
	calls []uuid.UUID
	errOn map[uuid.UUID]error
	err   error
}

func (f *fakeVerificationService) RunGate1Verification(
	_ context.Context, submissionID uuid.UUID,
) (*domain.Submission, error) {
	f.calls = append(f.calls, submissionID)
	if f.err != nil {
		return nil, f.err
	}
	if err := f.errOn[submissionID]; err != nil {
		return nil, err
	}
	return &domain.Submission{ID: submissionID, Status: domain.SubmissionStatusInReview}, nil
}

func TestVerificationWorker_ProcessesEveryPendingSubmission(t *testing.T) {
	ctx := context.Background()
	first, second := uuid.New(), uuid.New()
	repo := &fakeSubmissionsRepo{pending: []*domain.Submission{{ID: first}, {ID: second}}}
	svc := &fakeVerificationService{}
	worker := job.NewVerificationWorker(repo, svc)

	if err := worker.ProcessPending(ctx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if len(svc.calls) != 2 || svc.calls[0] != first || svc.calls[1] != second {
		t.Fatalf("expected both submissions verified in order, got %v", svc.calls)
	}
}

// A failing submission must not stop the rest, and must not disappear: the
// joined error names the submission so an operator can find it.
func TestVerificationWorker_SurfacesFailuresAndContinues(t *testing.T) {
	ctx := context.Background()
	failing, healthy := uuid.New(), uuid.New()
	repo := &fakeSubmissionsRepo{pending: []*domain.Submission{{ID: failing}, {ID: healthy}}}
	svc := &fakeVerificationService{errOn: map[uuid.UUID]error{
		failing: errors.New("item verifier unavailable"),
	}}
	worker := job.NewVerificationWorker(repo, svc)

	err := worker.ProcessPending(ctx)
	if err == nil {
		t.Fatal("a failed verification must be reported, not discarded")
	}
	if !strings.Contains(err.Error(), failing.String()) {
		t.Errorf("error should name the failed submission, got %v", err)
	}
	if len(svc.calls) != 2 {
		t.Fatalf("the healthy submission should still be verified, calls = %v", svc.calls)
	}
}

func TestVerificationWorker_RepositoryFailurePropagates(t *testing.T) {
	ctx := context.Background()
	repo := &fakeSubmissionsRepo{err: errors.New("database unavailable")}
	worker := job.NewVerificationWorker(repo, &fakeVerificationService{})

	if err := worker.ProcessPending(ctx); err == nil {
		t.Fatal("a failed pending-list read must propagate")
	}
}

func TestVerificationWorker_CancellationStopsBeforeVerify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := &fakeSubmissionsRepo{pending: []*domain.Submission{{ID: uuid.New()}}}
	svc := &fakeVerificationService{}
	worker := job.NewVerificationWorker(repo, svc)

	err := worker.ProcessPending(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(svc.calls) != 0 {
		t.Fatalf("a canceled run must not call the verifier, calls = %v", svc.calls)
	}
}

func TestVerificationWorker_CronJobPollsRetryably(t *testing.T) {
	worker := job.NewVerificationWorker(&fakeSubmissionsRepo{}, &fakeVerificationService{})
	cronJob := worker.CronJob()

	if cronJob.Interval.Seconds() != 10 {
		t.Errorf("verification interval = %v, want 10s", cronJob.Interval)
	}
	if cronJob.LockID != job.Gate1VerificationLockID {
		t.Errorf("lock id = %d, want %d", cronJob.LockID, job.Gate1VerificationLockID)
	}
}
