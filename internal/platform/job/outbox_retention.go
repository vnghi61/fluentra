package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	jobsqlc "github.com/fluentra/fluentra/internal/generated/job/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// OutboxRetentionLockID is in the job-owned advisory-lock range reserved
	// for housekeeping. The task is safe to repeat, but one sweep at a time
	// avoids replicas competing to delete the same index range.
	OutboxRetentionLockID int64 = 1_700_000_050

	// OutboxRetentionInterval bounds how long an eligible payload can remain
	// after its configured retention window. Running more often would add write
	// churn without reducing the meaningful exposure window.
	OutboxRetentionInterval = 24 * time.Hour
)

// OutboxPruner removes dispatched outbox events after their configured
// retention window. It deliberately keeps dead letters: they have not been
// successfully published and are the evidence an operator needs to diagnose
// why, so treating them as ordinary delivered events would hide a failure.
type OutboxPruner struct {
	queries       *jobsqlc.Queries
	retentionDays int
	now           func() time.Time
}

// NewOutboxPruner builds the job-owned retention task. Days rather than a
// duration matches OUTBOX_PUBLISHED_RETENTION_DAYS and AddDate avoids a large
// configured integer overflowing a time.Duration into an immediate purge.
func NewOutboxPruner(pool *pgxpool.Pool, retentionDays int) (*OutboxPruner, error) {
	if retentionDays <= 0 {
		return nil, fmt.Errorf("outbox published retention days must be a positive integer")
	}
	return &OutboxPruner{
		queries:       jobsqlc.New(pool),
		retentionDays: retentionDays,
		now:           time.Now,
	}, nil
}

// CronJob returns the daily, lock-protected pruning task registered by the
// worker composition root.
func (p *OutboxPruner) CronJob() CronJob {
	return CronJob{
		Name:     "prune_published_outbox_events",
		LockID:   OutboxRetentionLockID,
		Interval: OutboxRetentionInterval,
		Task:     p.Prune,
	}
}

// Prune deletes only published, non-dead-lettered events that have outlived
// retention. The partial index introduced with this task matches the predicate,
// so the daily sweep does not turn a growing historical outbox into a table scan.
func (p *OutboxPruner) Prune(ctx context.Context) error {
	cutoff := p.now().UTC().AddDate(0, 0, -p.retentionDays)
	removed, err := p.queries.DeletePublishedOutboxEventsBefore(ctx, &cutoff)
	if err != nil {
		return fmt.Errorf("delete published outbox events before %s: %w", cutoff.Format(time.RFC3339), err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "pruned published outbox events",
			"module", "job", "removed", removed, "cutoff", cutoff)
	}
	return nil
}
