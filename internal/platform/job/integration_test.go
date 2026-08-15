//go:build integration

package job_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/platform/job"
)

const jobDatabase = "fluentra_platform_job_test"

var sharedPool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, jobDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", jobDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", jobDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", jobDatabase, err)
		os.Exit(1)
	}
	if err := job.MigrateUp(context.Background(), created); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "apply river migrations for %s: %v\n", jobDatabase, err)
		os.Exit(1)
	}
	sharedPool = created

	code := m.Run()

	sharedPool.Close()
	dropDatabase()
	os.Exit(code)
}

func createDatabase(base, name string) (string, func(), error) {
	maintenance, err := replaceDatabase(base, "postgres")
	if err != nil {
		return "", nil, err
	}
	admin, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = admin.Close() }()

	ctx := context.Background()
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := admin.ExecContext(ctx, drop); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", name, err)
	}

	dsn, err := replaceDatabase(base, name)
	if err != nil {
		return "", nil, err
	}
	return dsn, func() {
		cleanup, err := sql.Open("pgx", maintenance)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), drop)
	}, nil
}

func migrateUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("create goose provider: %w", err)
	}
	defer func() { _ = provider.Close() }()
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// newTestPool returns the isolated package test pool and resets tables before each test.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	const truncateQuery = "TRUNCATE ops.river_job, ops.outbox_events, ops.job_failures"
	if _, err := sharedPool.Exec(context.Background(), truncateQuery); err != nil {
		t.Fatalf("reset tables: %v", err)
	}
	return sharedPool
}

// TestOutboxRetention_PrunesOnlyPublishedRowsPastTheWindow proves the privacy
// boundary against PostgreSQL itself. A pending event must remain deliverable and
// a dead letter must remain available for triage even when both are as old as a
// published payload that is safe to erase.
func TestOutboxRetention_PrunesOnlyPublishedRowsPastTheWindow(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	insert := func(t *testing.T, publishedAt, deadLetteredAt *time.Time) string {
		t.Helper()
		var eventID string
		err := pool.QueryRow(ctx, `
			INSERT INTO ops.outbox_events (aggregate, event, payload, published_at, dead_lettered_at)
			VALUES ('auth', 'verification_requested', '{}'::jsonb, $1, $2)
			RETURNING event_id`, publishedAt, deadLetteredAt).Scan(&eventID)
		if err != nil {
			t.Fatalf("insert outbox event: %v", err)
		}
		return eventID
	}

	old := now.AddDate(0, 0, -31)
	publishedID := insert(t, &old, nil)
	unpublishedID := insert(t, nil, nil)
	deadLetteredID := insert(t, &old, &old)

	pruner, err := job.NewOutboxPruner(pool, 30)
	if err != nil {
		t.Fatalf("new outbox pruner: %v", err)
	}
	if err := pruner.Prune(ctx); err != nil {
		t.Fatalf("prune: %v", err)
	}

	for name, event := range map[string]struct {
		id   string
		want int
	}{
		"published row older than retention is removed":      {publishedID, 0},
		"unpublished row of the same age stays deliverable":  {unpublishedID, 1},
		"dead-lettered row of the same age stays for triage": {deadLetteredID, 1},
	} {
		t.Run(name, func(t *testing.T) {
			got := countRows(t, pool, `SELECT count(*) FROM ops.outbox_events WHERE event_id = $1`, event.id)
			if got != event.want {
				t.Errorf("rows for %s = %d, want %d", event.id, got, event.want)
			}
		})
	}
}

// --- test job kinds -------------------------------------------------------

type panicArgs struct{}

func (panicArgs) Kind() string { return "test_panic" }

type panicWorker struct {
	river.WorkerDefaults[panicArgs]
}

func (*panicWorker) Work(context.Context, *river.Job[panicArgs]) error {
	panic("handler exploded on purpose")
}

type healthyArgs struct{}

func (healthyArgs) Kind() string { return "test_healthy" }

type healthyWorker struct {
	river.WorkerDefaults[healthyArgs]
	ran atomic.Int64
}

func (w *healthyWorker) Work(context.Context, *river.Job[healthyArgs]) error {
	w.ran.Add(1)
	return nil
}

// stuckArgs models the handler that ignores its context, which is the only case
// where a timeout has to actually cancel something.
type stuckArgs struct{}

func (stuckArgs) Kind() string { return "test_stuck" }

type stuckWorker struct {
	river.WorkerDefaults[stuckArgs]
	observed atomic.Int64
}

func (w *stuckWorker) Work(ctx context.Context, _ *river.Job[stuckArgs]) error {
	select {
	case <-ctx.Done():
		w.observed.Add(1)
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return nil
	}
}

type concurrentArgs struct{}

func (concurrentArgs) Kind() string { return "test_concurrent" }

type concurrentWorker struct {
	river.WorkerDefaults[concurrentArgs]
	active atomic.Int64
	peak   atomic.Int64
}

func (w *concurrentWorker) Work(context.Context, *river.Job[concurrentArgs]) error {
	current := w.active.Add(1)
	for {
		peak := w.peak.Load()
		if current <= peak || w.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	time.Sleep(300 * time.Millisecond)
	w.active.Add(-1)
	return nil
}

// --- helpers --------------------------------------------------------------

func startWorker(t *testing.T, pool *pgxpool.Pool, opts job.WorkerOptions) *job.Worker {
	t.Helper()
	opts.Pool = pool
	worker, err := job.NewWorker(opts)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err := worker.Start(context.Background(), opts.Instruments); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = worker.Stop(stopCtx)
	})
	return worker
}

// eventually polls until condition holds or the deadline passes. Job processing
// is asynchronous; a fixed sleep would be both slower and flakier.
func eventually(t *testing.T, timeout time.Duration, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

// --- tests ----------------------------------------------------------------

// TestWorker_PanicIsDeadLetteredAndWorkerKeepsRunning is the card's headline
// acceptance: a panicking handler is recorded and the worker survives to run
// the next job. Before this card there was no worker at all, so a panic in a
// handler had never been exercised.
func TestWorker_PanicIsDeadLetteredAndWorkerKeepsRunning(t *testing.T) {
	pool := newTestPool(t)

	healthy := &healthyWorker{}
	workers := river.NewWorkers()
	river.AddWorker(workers, &panicWorker{})
	river.AddWorker(workers, healthy)

	worker := startWorker(t, pool, job.WorkerOptions{
		Queues:     map[string]int{job.QueueDefault: 2},
		Workers:    workers,
		JobTimeout: 5 * time.Second,
	})

	ctx := context.Background()
	// MaxAttempts 1 makes the first failure the final one, so the dead-letter
	// path is reached without waiting out a retry schedule.
	if _, err := worker.Client().Insert(ctx, panicArgs{}, &river.InsertOpts{MaxAttempts: 1}); err != nil {
		t.Fatalf("insert panicking job: %v", err)
	}
	if _, err := worker.Client().Insert(ctx, healthyArgs{}, nil); err != nil {
		t.Fatalf("insert healthy job: %v", err)
	}

	eventually(t, 30*time.Second, "the panicking job to be dead-lettered", func() bool {
		return countRows(t, pool,
			`SELECT count(*) FROM ops.job_failures WHERE kind = $1`, "test_panic") == 1
	})

	eventually(t, 30*time.Second, "the worker to process the next job", func() bool {
		return healthy.ran.Load() == 1
	})

	var lastError string
	err := pool.QueryRow(ctx,
		`SELECT last_error FROM ops.job_failures WHERE kind = $1`, "test_panic").Scan(&lastError)
	if err != nil {
		t.Fatalf("read dead-letter row: %v", err)
	}
	if !strings.Contains(lastError, "panicked") {
		t.Errorf("last_error = %q, want it to name the panic", lastError)
	}
}

// TestWorker_TimedOutJobDoesNotHoldTheQueue proves the timeout cancels the
// handler and frees the slot, with the queue limited to one worker so a stuck
// job would block everything behind it.
func TestWorker_TimedOutJobDoesNotHoldTheQueue(t *testing.T) {
	pool := newTestPool(t)

	stuck := &stuckWorker{}
	healthy := &healthyWorker{}
	workers := river.NewWorkers()
	river.AddWorker(workers, stuck)
	river.AddWorker(workers, healthy)

	worker := startWorker(t, pool, job.WorkerOptions{
		Queues:     map[string]int{job.QueueDefault: 1},
		Workers:    workers,
		JobTimeout: 500 * time.Millisecond,
	})

	ctx := context.Background()
	if _, err := worker.Client().Insert(ctx, stuckArgs{}, &river.InsertOpts{MaxAttempts: 1}); err != nil {
		t.Fatalf("insert stuck job: %v", err)
	}
	if _, err := worker.Client().Insert(ctx, healthyArgs{}, nil); err != nil {
		t.Fatalf("insert healthy job: %v", err)
	}

	eventually(t, 20*time.Second, "the stuck handler to be cancelled", func() bool {
		return stuck.observed.Load() == 1
	})
	eventually(t, 20*time.Second, "the queue to keep moving behind the stuck job", func() bool {
		return healthy.ran.Load() == 1
	})
}

// TestWorker_OldestPendingGaugeReportsRealAge covers the instrument that was
// declared as an observable gauge with no callback, so it never emitted.
func TestWorker_OldestPendingGaugeReportsRealAge(t *testing.T) {
	pool := newTestPool(t)
	instruments, reader := newTestInstruments(t)

	// The worker consumes `default`; the jobs go to `batch`, so they stay
	// available and the gauge has something real to report.
	workers := river.NewWorkers()
	river.AddWorker(workers, &healthyWorker{})
	worker := startWorker(t, pool, job.WorkerOptions{
		Queues:      map[string]int{job.QueueDefault: 1},
		Workers:     workers,
		JobTimeout:  5 * time.Second,
		Instruments: instruments,
	})

	ctx := context.Background()
	_, err := worker.Client().Insert(ctx, healthyArgs{}, &river.InsertOpts{
		Queue:       job.QueueBatch,
		ScheduledAt: time.Now().Add(-90 * time.Second),
	})
	if err != nil {
		t.Fatalf("insert pending job: %v", err)
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatalf("collect: %v", err)
	}
	gauge := findMetric(collected, "job_oldest_pending_seconds")
	if gauge == nil {
		t.Fatal("job_oldest_pending_seconds was not exported; the callback is not registered")
	}
	data, ok := gauge.Data.(metricdata.Gauge[int64])
	if !ok || len(data.DataPoints) != 1 {
		t.Fatalf("gauge data = %#v", gauge.Data)
	}
	if age := data.DataPoints[0].Value; age < 60 {
		t.Errorf("oldest pending age = %ds, want at least 60s", age)
	}

	// And it must read zero rather than go missing once the queue drains.
	if _, err := pool.Exec(ctx, "TRUNCATE ops.river_job"); err != nil {
		t.Fatalf("drain queue: %v", err)
	}
	seconds, err := worker.OldestPendingSeconds(ctx)
	if err != nil {
		t.Fatalf("oldest pending: %v", err)
	}
	if seconds != 0 {
		t.Errorf("oldest pending on an empty queue = %d, want 0", seconds)
	}

	// Work deliberately scheduled for later is not backlog. Without this the
	// gauge would alarm on every job anyone scheduled for tomorrow.
	_, err = worker.Client().Insert(ctx, healthyArgs{}, &river.InsertOpts{
		Queue:       job.QueueBatch,
		ScheduledAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("insert future job: %v", err)
	}
	seconds, err = worker.OldestPendingSeconds(ctx)
	if err != nil {
		t.Fatalf("oldest pending: %v", err)
	}
	if seconds != 0 {
		t.Errorf("a job scheduled an hour out counted as %ds of backlog", seconds)
	}
}

// TestWorker_QueueConcurrencyComesFromConfiguration runs the same jobs under two
// configurations and asserts the observed parallelism differs. A hardcoded
// concurrency would make both runs identical.
func TestWorker_QueueConcurrencyComesFromConfiguration(t *testing.T) {
	for name, test := range map[string]struct {
		spec     string
		wantPeak int64
	}{
		"serial":   {"default:1", 1},
		"parallel": {"default:4", 4},
	} {
		t.Run(name, func(t *testing.T) {
			pool := newTestPool(t)

			queues, err := job.ParseQueues(test.spec)
			if err != nil {
				t.Fatalf("parse queues: %v", err)
			}

			counter := &concurrentWorker{}
			workers := river.NewWorkers()
			river.AddWorker(workers, counter)

			worker := startWorker(t, pool, job.WorkerOptions{
				Queues:     queues,
				Workers:    workers,
				JobTimeout: 10 * time.Second,
			})

			ctx := context.Background()
			for range 4 {
				if _, err := worker.Client().Insert(ctx, concurrentArgs{}, nil); err != nil {
					t.Fatalf("insert: %v", err)
				}
			}

			eventually(t, 30*time.Second, "all four jobs to finish", func() bool {
				return countRows(t, pool,
					`SELECT count(*) FROM ops.river_job WHERE state = 'completed'`) == 4
			})

			if peak := counter.peak.Load(); peak != test.wantPeak {
				t.Errorf("peak concurrency with %q = %d, want %d", test.spec, peak, test.wantPeak)
			}
		})
	}
}
