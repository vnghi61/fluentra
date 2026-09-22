package job

import (
	"context"
	"time"

	"github.com/fluentra/fluentra/internal/platform/job"
)

// Advisory lock id for daily generation job (WO 19 §I).
const dailyGenerationLockID int64 = 1_700_000_880

const dailyGenerationInterval = 24 * time.Hour

// DailyGenerator coordinates daily generation for learners' weak nodes.
type DailyGenerator interface {
	DailyGeneration(ctx context.Context) error
}

// DailyGenerationJob creates the scheduled daily generation cron job.
func DailyGenerationJob(generator DailyGenerator) job.CronJob {
	return job.CronJob{
		Name:     "learning.daily_generation",
		LockID:   dailyGenerationLockID,
		Interval: dailyGenerationInterval,
		Task:     generator.DailyGeneration,
	}
}
