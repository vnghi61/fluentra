package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Partition management constants.
const (
	// PartitionsAhead is how many months are created beyond the current one.
	// Three is what the P1.4 card specifies, and it is also the margin that
	// lets the rotation job miss two runs without writes starting to fail.
	PartitionsAhead = 3

	// RetentionPeriod is BR-AUDIT-05. Two years of trail, then the partition
	// leaves the table.
	RetentionPeriod = 2 * 365 * 24 * time.Hour
)

// RotatePartitions creates the partitions the coming months will need.
//
// It is idempotent — the function it calls skips any partition that already
// exists — so running it hourly, daily, or twice at once is harmless. That
// matters because the cron scheduler guards with an advisory lock that a
// second worker can lose without anybody noticing.
func (s *Service) RotatePartitions(ctx context.Context) error {
	created, err := s.repo.EnsurePartitions(ctx, PartitionsAhead)
	if err != nil {
		return fmt.Errorf("rotate audit partitions: %w", err)
	}
	if created > 0 {
		slog.InfoContext(ctx, "created audit partitions",
			"module", "audit", "op", "RotatePartitions", "count", created)
	}
	return nil
}

// EnforceRetention detaches every partition whose whole month has passed out
// of the retention period.
//
// Detached, not dropped. Detaching is instant and reversible; dropping is
// neither, and the archival step that should follow needs object storage,
// which this module does not depend on. A partition left detached is
// unreachable through the parent table and has no grants, so it cannot be read
// or written by the application — it is waiting to be archived, not in use.
func (s *Service) EnforceRetention(ctx context.Context) error {
	detached, err := s.repo.DetachExpiredPartitions(ctx, RetentionPeriod)
	if err != nil {
		return fmt.Errorf("enforce audit retention: %w", err)
	}
	if len(detached) > 0 {
		slog.InfoContext(ctx, "detached expired audit partitions; they await archival",
			"module", "audit", "op", "EnforceRetention",
			"count", len(detached), "partitions", detached)
	}
	return nil
}
