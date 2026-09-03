package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/platform/job"
)

const (
	// CacheRetentionLockID follows the convention of taking the timestamp of the
	// migration whose tables it manages (1700000310).
	CacheRetentionLockID int64 = 1_700_000_310

	// CacheRetentionInterval runs periodically to clean expired entries.
	CacheRetentionInterval = 1 * time.Hour
)

// CachePruner removes expired ai_cache_entries.
type CachePruner struct {
	cache *DBCache
}

// NewCachePruner builds the cache retention task over a database pool.
func NewCachePruner(pool *pgxpool.Pool) *CachePruner {
	return &CachePruner{
		cache: NewDBCache(pool),
	}
}

// CronJob returns the scheduled pruning task for the worker scheduler.
func (p *CachePruner) CronJob() job.CronJob {
	return job.CronJob{
		Name:     "ai.prune_expired_cache",
		LockID:   CacheRetentionLockID,
		Interval: CacheRetentionInterval,
		Task:     p.Prune,
	}
}

// Prune sweeps expired cache rows.
func (p *CachePruner) Prune(ctx context.Context) error {
	removed, err := p.cache.PruneExpired(ctx)
	if err != nil {
		return fmt.Errorf("prune expired ai cache entries: %w", err)
	}
	if removed > 0 {
		slog.InfoContext(ctx, "pruned expired ai cache entries",
			"module", "ai", "removed", removed)
	}
	return nil
}
