// Package job holds the learning module's background tasks.
package job

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/platform/telemetry"
)

// Advisory lock id for the retention refresh, by the convention that a cron job
// takes the timestamp of the migration whose tables it reads.
const retentionLockID int64 = 1_700_000_211

const retentionInterval = 15 * time.Minute

// RetentionRefresher recomputes the two cohorts D1 retention is a ratio of.
//
// ROADMAP.md's Phase 2 exit criterion is "D1 retention measurable", and the trap
// the brief names is a dashboard of raw event counts that nobody can turn into
// that number without writing SQL — measurable in principle, which is the state
// the criterion exists to rule out. So the query runs here, on a schedule, and
// the answer is published as two gauges a Grafana panel divides.
//
// It is a gauge and not a counter because retention is a question about a window,
// not an event: "of the learners active yesterday, how many came back today".
type RetentionRefresher struct {
	pool     *pgxpool.Pool
	snapshot atomic.Pointer[telemetry.RetentionSnapshot]
}

// NewRetentionRefresher constructs the refresher over a pool.
func NewRetentionRefresher(pool *pgxpool.Pool) *RetentionRefresher {
	r := &RetentionRefresher{pool: pool}
	r.snapshot.Store(&telemetry.RetentionSnapshot{})
	return r
}

// Snapshot returns the last computed measurement.
//
// A scrape landing between refreshes reads the previous real answer rather than
// zero: a retention gauge that drops to zero between refreshes would look like
// every learner churning at once.
func (r *RetentionRefresher) Snapshot() telemetry.RetentionSnapshot {
	if snapshot := r.snapshot.Load(); snapshot != nil {
		return *snapshot
	}
	return telemetry.RetentionSnapshot{}
}

// CronJob returns the scheduled refresh.
func (r *RetentionRefresher) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "learning.refresh_retention",
		LockID:   retentionLockID,
		Interval: retentionInterval,
		Task:     r.Refresh,
	}
}

// retentionQuery counts the day-zero cohort and the part of it that returned.
//
// "Active" is an attempt, because that is the first thing a learner does that
// the product records — an enrolment with nothing after it is not a learner who
// came back. Both halves are counted from the same table in one statement so the
// ratio cannot be taken across two inconsistent reads.
//
// The window is the learner's day in UTC. Per-learner local midnight is what the
// due queue uses and would be more correct here too; it needs a join to
// core.users, which learn.attempts has no business making. Recorded in
// learning/TODO.md rather than approximated silently.
const retentionQuery = `
WITH active AS (
    SELECT DISTINCT user_id, (created_at AT TIME ZONE 'UTC')::date AS active_on
    FROM learn.attempts
    WHERE created_at >= now() - interval '2 days'
),
d0 AS (
    SELECT user_id FROM active
    WHERE active_on = ((now() AT TIME ZONE 'UTC')::date - 1)
),
d1 AS (
    SELECT user_id FROM active
    WHERE active_on = (now() AT TIME ZONE 'UTC')::date
)
SELECT
    (SELECT count(*) FROM d0)::bigint AS d0,
    (SELECT count(*) FROM d0 JOIN d1 USING (user_id))::bigint AS d1_returned`

// Refresh recomputes the cohorts and stores the result for the gauge callback.
func (r *RetentionRefresher) Refresh(ctx context.Context) error {
	if r.pool == nil {
		return fmt.Errorf("learning retention refresher has no pool")
	}

	var snapshot telemetry.RetentionSnapshot
	if err := r.pool.QueryRow(ctx, retentionQuery).Scan(&snapshot.D0, &snapshot.D1Returned); err != nil {
		return fmt.Errorf("compute retention cohorts: %w", err)
	}

	r.snapshot.Store(&snapshot)
	return nil
}
