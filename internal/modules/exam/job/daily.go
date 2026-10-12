package job

import (
	"context"
	"time"

	platformjob "github.com/fluentra/fluentra/internal/platform/job"
)

// DailyGenerationLockID is the advisory lock for the daily generation job
// (WO 22 Stage O).
const DailyGenerationLockID int64 = 1_700_000_950

// ComposeFixedTestsLockID is the advisory lock for the fixed-test sweep (WO 22
// Stage O).
const ComposeFixedTestsLockID int64 = 1_700_000_951

// FixedTestComposer composes the next numbered fixed tests every exam's bank can
// fill.
type FixedTestComposer interface {
	ComposeAllFixedTests(ctx context.Context) error
}

// ComposeFixedTestsJob returns the hourly sweep that composes a new numbered
// test once published questions complete one — after a batch approval as much
// as after the daily job. It generates nothing, so it runs whether or not daily
// generation is switched on.
func ComposeFixedTestsJob(service FixedTestComposer) platformjob.CronJob {
	return platformjob.CronJob{
		Name:     "exam.compose_fixed_tests",
		LockID:   ComposeFixedTestsLockID,
		Interval: time.Hour,
		Task: func(ctx context.Context) error {
			return service.ComposeAllFixedTests(ctx)
		},
	}
}

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
