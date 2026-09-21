package job

import (
	"context"
	"time"

	"github.com/fluentra/fluentra/internal/platform/job"
)

// SweepPendingResourcesLockID is the advisory lock ID for sweeping abandoned upload intents.
const SweepPendingResourcesLockID int64 = 1_700_000_790

// Sweeper sweeps expired pending upload intents.
type Sweeper interface {
	SweepExpiredPending(ctx context.Context) (int, error)
}

// SweepPendingJob returns a scheduled CronJob for sweeping abandoned upload intents past presign TTL.
func SweepPendingJob(sweeper Sweeper) job.CronJob {
	return job.CronJob{
		Name:     "resource.sweep_pending",
		LockID:   SweepPendingResourcesLockID,
		Interval: 10 * time.Minute,
		Task: func(ctx context.Context) error {
			_, err := sweeper.SweepExpiredPending(ctx)
			return err
		},
	}
}
