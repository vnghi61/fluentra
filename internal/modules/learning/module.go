package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/generated/learning/sqlc"
	"github.com/fluentra/fluentra/internal/platform/job"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// Advisory lock id for learning module partition maintenance.
// Value is unique across the repository based on migration timestamp 1700000210.
const rotatePartitionsLockID int64 = 1_700_000_210

// rotateInterval is how frequently partition creation is checked.
const rotateInterval = 6 * time.Hour

// Deps defines dependencies supplied by the composition root.
type Deps struct {
	Pool  *pgxpool.Pool
	Clock clock.Clock
}

// Module represents the learning module.
type Module struct {
	pool    *pgxpool.Pool
	clock   clock.Clock
	queries *sqlc.Queries
}

// New constructs and wires the learning module.
func New(deps Deps) *Module {
	timekeeper := deps.Clock
	if timekeeper == nil {
		timekeeper = clock.Real{}
	}

	var queries *sqlc.Queries
	if deps.Pool != nil {
		queries = sqlc.New(deps.Pool)
	}

	return &Module{
		pool:    deps.Pool,
		clock:   timekeeper,
		queries: queries,
	}
}

// CronJobs returns the scheduled partition maintenance job.
func (m *Module) CronJobs() []job.CronJob {
	return []job.CronJob{
		{
			Name:     "learning.rotate_partitions",
			LockID:   rotatePartitionsLockID,
			Interval: rotateInterval,
			Task:     m.RotatePartitions,
		},
	}
}

// RotatePartitions creates future partitions for the attempts table.
// Exported so cmd/worker can invoke it at start-up to avoid partition lapse outages.
func (m *Module) RotatePartitions(ctx context.Context) error {
	if m.queries == nil {
		return fmt.Errorf("learning module pool is nil")
	}
	_, err := m.queries.EnsurePartitions(ctx, 3)
	if err != nil {
		return fmt.Errorf("ensure learning partitions: %w", err)
	}
	return nil
}
