package job

import (
	"context"
	"time"

	"github.com/fluentra/fluentra/internal/platform/job"
)

// PurgeRecordingsLockID is the advisory lock ID for the 90-day recording retention sweep.
const PurgeRecordingsLockID int64 = 1_700_000_215

// Purger sweeps recordings older than retention limit.
type Purger interface {
	PurgeRecordings(ctx context.Context, retention time.Duration) (int, error)
}

// PurgeJob returns a scheduled CronJob for purging audio older than 90 days.
func PurgeJob(purger Purger) job.CronJob {
	return job.CronJob{
		Name:     "speaking.purge_recordings",
		LockID:   PurgeRecordingsLockID,
		Interval: 24 * time.Hour,
		Task: func(ctx context.Context) error {
			_, err := purger.PurgeRecordings(ctx, 90*24*time.Hour)
			return err
		},
	}
}
