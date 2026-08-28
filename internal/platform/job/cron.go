package job

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/fluentra/fluentra/internal/platform/telemetry"
)

// cronQueue is the `queue` label value for scheduled jobs.
//
// Cron jobs emit the same job_attempts_total and job_duration_seconds as River
// jobs, under a queue of their own, so one alert covers both. Until they did,
// the only trace of a failed rotation was a log line: the partition job could
// fail every six hours for a month and no alert could see it, which is the
// outage P8.2 paid to learn about happening quietly.
const cronQueue = "cron"

// CronJob represents a scheduled background task.
type CronJob struct {
	Name     string
	LockID   int64
	Interval time.Duration
	Task     func(ctx context.Context) error
}

// CronScheduler manages periodic background jobs backed by Postgres advisory locks.
type CronScheduler struct {
	pool        *pgxpool.Pool
	instruments telemetry.Instruments
	jobs        []CronJob
	mu          sync.Mutex
}

// NewCronScheduler creates a cron scheduler facade.
//
// Instruments are optional so tests and the migrate path can build a scheduler
// without a meter; a zero Instruments records nothing.
func NewCronScheduler(pool *pgxpool.Pool) *CronScheduler {
	return &CronScheduler{pool: pool}
}

// WithInstruments returns the scheduler with metrics enabled.
func (s *CronScheduler) WithInstruments(instruments telemetry.Instruments) *CronScheduler {
	s.instruments = instruments
	return s
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
	started := time.Now()
	err = job.Task(ctx)
	result := "success"
	if err != nil {
		result = "error"
		slog.ErrorContext(ctx, "cron job failed", "job", job.Name, "error", err)
	}
	s.record(ctx, job.Name, result, time.Since(started))
}

// RecordForTest exposes record so the metric contract the alert depends on can
// be asserted without waiting out a ticker.
func (s *CronScheduler) RecordForTest(ctx context.Context, name, result string, duration time.Duration) {
	s.record(ctx, name, result, duration)
}

// record emits the same instruments the River middleware does, so a cron failure
// is visible to the same alert. The labels are a closed set: the job name is
// registered in code, never derived from input.
func (s *CronScheduler) record(ctx context.Context, name, result string, duration time.Duration) {
	if s.instruments.JobAttempts == nil || s.instruments.JobDuration == nil {
		return
	}
	attributes := metric.WithAttributes(
		attribute.String("queue", cronQueue),
		attribute.String("kind", name),
		attribute.String("result", result),
	)
	s.instruments.JobDuration.Record(ctx, duration.Seconds(), attributes)
	s.instruments.JobAttempts.Add(ctx, 1, attributes)
}
