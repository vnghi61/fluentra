package job

import (
	"context"
	"time"

	platformjob "github.com/fluentra/fluentra/internal/platform/job"
)

const (
	// SweepExpiredLockID is the advisory lock for the 1-minute sweep (WO12 §6).
	SweepExpiredLockID int64 = 1_700_000_601
)

// ExamSweeper defines the interface for sweeping overdue sittings.
type ExamSweeper interface {
	SweepExpired(ctx context.Context) error
}

// SweepExpiredJob returns the platform scheduled cron job.
func SweepExpiredJob(service ExamSweeper) platformjob.CronJob {
	return platformjob.CronJob{
		Name:     "exam.sweep_expired",
		LockID:   SweepExpiredLockID,
		Interval: 1 * time.Minute,
		Task: func(ctx context.Context) error {
			return service.SweepExpired(ctx)
		},
	}
}
