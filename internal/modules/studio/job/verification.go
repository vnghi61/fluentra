// Package job implements background tasks for the studio module including automated Gate 1 verification.
package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/studio/domain"
	"github.com/fluentra/fluentra/internal/platform/job"
)

// Gate1VerificationLockID is the advisory lock id for the Gate 1 verification job (matches studio migration timestamp).
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
//
// A failure on one submission does not stop the others, and does not vanish:
// each is logged with its submission id (never the report or course body) and
// returned joined, so the scheduler records that the pass was not clean. The
// submission itself stays in `submitted`/`verifying`, which is exactly the set
// this worker polls, so the next tick retries it rather than losing it.
func (w *VerificationWorker) ProcessPending(ctx context.Context) error {
	pending, err := w.repo.ListPendingVerificationSubmissions(ctx, 10)
	if err != nil {
		return err
	}

	var failures []error
	for _, sub := range pending {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		if _, err := w.service.RunGate1Verification(ctx, sub.ID); err != nil {
			slog.ErrorContext(ctx, "Gate 1 verification failed; the submission stays pending and will be retried",
				"submission_id", sub.ID, "error", err)
			failures = append(failures, fmt.Errorf("verify submission %s: %w", sub.ID, err))
		}
	}

	return errors.Join(failures...)
}
