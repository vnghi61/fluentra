package job

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CronJob represents a scheduled background task.
type CronJob struct {
	Name     string
	LockID   int64
	Interval time.Duration
	Task     func(ctx context.Context) error
}

// CronScheduler manages periodic background jobs backed by Postgres advisory locks.
type CronScheduler struct {
	pool *pgxpool.Pool
	jobs []CronJob
	mu   sync.Mutex
}

// NewCronScheduler creates a cron scheduler facade.
func NewCronScheduler(pool *pgxpool.Pool) *CronScheduler {
	return &CronScheduler{pool: pool}
}

// Register adds a periodic job to the scheduler.
func (s *CronScheduler) Register(job CronJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job)
}

// Start executes all registered cron loops until context is canceled.
func (s *CronScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	jobs := append([]CronJob{}, s.jobs...)
	s.mu.Unlock()

	for _, j := range jobs {
		go s.runJobLoop(ctx, j)
	}
}

func (s *CronScheduler) runJobLoop(ctx context.Context, job CronJob) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executeWithLock(ctx, job)
		}
	}
}

func (s *CronScheduler) executeWithLock(ctx context.Context, job CronJob) {
	if s.pool == nil {
		return
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to acquire connection for cron lock", "job", job.Name, "error", err)
		return
	}
	defer conn.Release()

	var acquired bool
	err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", job.LockID).Scan(&acquired)
	if err != nil || !acquired {
		// Lock held by another worker instance or query failed
		return
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", job.LockID)
	}()

	slog.InfoContext(ctx, "executing cron job", "job", job.Name)
	if err := job.Task(ctx); err != nil {
		slog.ErrorContext(ctx, "cron job failed", "job", job.Name, "error", err)
	}
}
