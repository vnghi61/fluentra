package job

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/platform/job"
)

// Advisory lock id for Gate 1 verification job (matches studio migration timestamp).
const Gate1VerificationLockID int64 = 1_700_000_750

const verificationInterval = 10 * time.Second

// SubmissionsRepository defines repo operations needed by the verification worker.
type SubmissionsRepository interface {
	ListPendingVerificationSubmissions(ctx context.Context, limit int32) ([]*domain.Submission, error)
}

// VerificationService defines the verification runner needed by the worker.
type VerificationService interface {
	RunGate1Verification(ctx context.Context, submissionID uuid.UUID) (*domain.Submission, error)
}

// VerificationWorker polls and processes submitted course drafts through Gate 1.
type VerificationWorker struct {
	repo    SubmissionsRepository
	service VerificationService
}

// NewVerificationWorker constructs the verification worker.
func NewVerificationWorker(repo SubmissionsRepository, service VerificationService) *VerificationWorker {
	return &VerificationWorker{
		repo:    repo,
		service: service,
	}
}

// CronJob returns the scheduled Gate 1 verification task.
func (w *VerificationWorker) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "studio.verify_submission",
		LockID:   Gate1VerificationLockID,
		Interval: verificationInterval,
		Task:     w.ProcessPending,
	}
}

// ProcessPending checks for submitted courses and executes Gate 1 checks.
func (w *VerificationWorker) ProcessPending(ctx context.Context) error {
	pending, err := w.repo.ListPendingVerificationSubmissions(ctx, 10)
	if err != nil {
		return err
	}

	for _, sub := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Run verification per submission
		_, _ = w.service.RunGate1Verification(ctx, sub.ID)
	}

	return nil
}
