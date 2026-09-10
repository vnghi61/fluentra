package vocabulary_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/vocabulary"
)

// TestCronJobsAreAllRegistered.
//
// `cmd/worker` registers whatever `CronJobs()` returns and asks no questions, so
// a job that is written, exported and left out of this slice runs never — with
// nothing failing, and nothing in the logs to say so. That is the "built and
// never wired" fault this repository has caught six times; for a scheduled job
// it is invisible rather than merely broken, because there is no screen to open.
//
// Lock ids are asserted alongside the names: two jobs sharing one advisory lock
// means the second waits on the first for ever, and the symptom is the same
// silence.
func TestCronJobsAreAllRegistered(t *testing.T) {
	t.Parallel()

	module := vocabulary.New(vocabulary.Deps{})

	want := map[string]bool{
		"vocabulary.generate_exercises": false,
		"vocabulary.verify_uploads":     false,
		"vocabulary.enrich_queued":      false,
		"vocabulary.enrich_examples":    false,
	}

	locks := map[int64]string{}
	for _, scheduled := range module.CronJobs() {
		seen, known := want[scheduled.Name]
		if !known {
			t.Errorf("unexpected cron job %q; add it to this test deliberately", scheduled.Name)
			continue
		}
		if seen {
			t.Errorf("cron job %q is registered twice", scheduled.Name)
		}
		want[scheduled.Name] = true

		if scheduled.Interval <= 0 {
			t.Errorf("cron job %q has interval %v; it would never run", scheduled.Name, scheduled.Interval)
		}
		if scheduled.Task == nil {
			t.Errorf("cron job %q has no task", scheduled.Name)
		}
		if other, clash := locks[scheduled.LockID]; clash {
			t.Errorf("cron jobs %q and %q share advisory lock %d", scheduled.Name, other, scheduled.LockID)
		}
		locks[scheduled.LockID] = scheduled.Name
	}

	for name, registered := range want {
		if !registered {
			t.Errorf("cron job %q is not registered; the worker will never run it", name)
		}
	}
}
