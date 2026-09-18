package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/platform/job"
)

func TestCronScheduler_TriggerDueDebounces(t *testing.T) {
	scheduler := job.NewCronScheduler(nil)
	scheduler.Register(job.CronJob{
		Name:     "test.hourly_job",
		Interval: 1 * time.Hour,
		LockID:   12345,
		Task: func(_ context.Context) error {
			return nil
		},
	})

	// Before any run, LastRun should report false
	if _, ok := scheduler.LastRun("test.hourly_job"); ok {
		t.Fatal("expected LastRun to be false before any execution")
	}

	// First TriggerDue should mark lastRun
	scheduler.TriggerDue(context.Background())
	firstRun, ok := scheduler.LastRun("test.hourly_job")
	if !ok {
		t.Fatal("expected LastRun to be true after TriggerDue")
	}

	// Subsequent immediate TriggerDue should be debounced and not change timestamp significantly
	scheduler.TriggerDue(context.Background())
	secondRun, ok := scheduler.LastRun("test.hourly_job")
	if !ok {
		t.Fatal("expected LastRun to be true after second TriggerDue")
	}

	if secondRun != firstRun {
		t.Fatalf("expected second TriggerDue to be debounced, got %v vs %v", secondRun, firstRun)
	}
}
