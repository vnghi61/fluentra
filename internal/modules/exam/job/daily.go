package job

import (
	"context"
	"time"

	platformjob "github.com/fluentra/fluentra/internal/platform/job"
)

// DailyGenerationLockID is the advisory lock for the daily generation job
// (WO 22 Stage O).
const DailyGenerationLockID int64 = 1_700_000_950

// DailyGenerator generates one day's exam content and composes the next test.
type DailyGenerator interface {
	GenerateDaily(ctx context.Context) error
}

// DailyGenerationJob returns the daily generation cron job. It runs once a day;
// the service itself is a no-op without a bank author, so the switch that
// decides whether it runs at all lives at registration.
func DailyGenerationJob(service DailyGenerator) platformjob.CronJob {
	return platformjob.CronJob{
		Name:     "questionbank.generate_daily",
		LockID:   DailyGenerationLockID,
		Interval: 24 * time.Hour,
		Task: func(ctx context.Context) error {
			return service.GenerateDaily(ctx)
		},
	}
}
