package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/fluentra/fluentra/internal/platform/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel/metric"
)

// Schema is where River's own tables live. It is the same schema as the outbox
// and the dead-letter table so the whole job subsystem is one unit of ownership
// (rule L3).
const Schema = "ops"

// DefaultMaxAttempts is River's attempt ceiling when config does not set one.
const DefaultMaxAttempts = 5

// ParseQueues reads the `WORKER_QUEUES` form `name:concurrency,name:concurrency`
// into River's queue map.
//
// Concurrency is configuration, not a constant: the media queue on a laptop and
// the media queue in production want different numbers, and a redeploy is the
// wrong unit of change for that.
func ParseQueues(spec string) (map[string]int, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, errors.New("job: queue specification is empty")
	}

	queues := make(map[string]int)
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, concurrency, found := strings.Cut(entry, ":")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, fmt.Errorf("job: queue entry %q must be name:concurrency", entry)
		}
		workers, err := strconv.Atoi(strings.TrimSpace(concurrency))
		if err != nil {
			return nil, fmt.Errorf("job: queue %q concurrency %q is not a number", name, concurrency)
		}
		if workers <= 0 {
			return nil, fmt.Errorf("job: queue %q concurrency must be positive, got %d", name, workers)
		}
		if _, duplicate := queues[name]; duplicate {
			return nil, fmt.Errorf("job: queue %q is declared twice", name)
		}
		queues[name] = workers
	}
	if len(queues) == 0 {
		return nil, errors.New("job: queue specification declared no queues")
	}
	return queues, nil
}

// WorkerOptions configures the River worker.
type WorkerOptions struct {
	Pool        *pgxpool.Pool
	Queues      map[string]int
	Workers     *river.Workers
	JobTimeout  time.Duration
	MaxAttempts int
	Instruments telemetry.Instruments
}

// Worker owns the River client that actually consumes queues, plus the
// instrumentation that makes the queues observable.
type Worker struct {
	client       *river.Client[pgx.Tx]
	pool         *pgxpool.Pool
	registration metric.Registration
}

// NewWorker builds the River client. It does not start it.
func NewWorker(opts WorkerOptions) (*Worker, error) {
	if opts.Pool == nil {
		return nil, errors.New("job: a database pool is required")
	}
	if len(opts.Queues) == 0 {
		return nil, errors.New("job: at least one queue is required")
	}
	if opts.Workers == nil {
		// River refuses to start without a worker bundle. An empty one is valid
		// and lets the worker boot before any module has registered a job kind.
		opts.Workers = river.NewWorkers()
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}

	queues := make(map[string]river.QueueConfig, len(opts.Queues))
	for name, concurrency := range opts.Queues {
		queues[name] = river.QueueConfig{MaxWorkers: concurrency}
	}

	client, err := river.NewClient(riverpgxv5.New(opts.Pool), &river.Config{
		Queues:      queues,
		Workers:     opts.Workers,
		Schema:      Schema,
		MaxAttempts: maxAttempts,
		// River's own timeout is the outer bound. The middleware applies the
		// same deadline one level in, where it can label the result and record
		// the span; this is the backstop if middleware is ever bypassed.
		JobTimeout:       opts.JobTimeout,
		WorkerMiddleware: []rivertype.WorkerMiddleware{NewMiddleware(opts.JobTimeout, opts.Instruments)},
		ErrorHandler:     &deadLetterHandler{pool: opts.Pool},
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return &Worker{client: client, pool: opts.Pool}, nil
}

// Client exposes the River client so callers can enqueue through job.Client.
func (w *Worker) Client() *river.Client[pgx.Tx] { return w.client }

// Start begins consuming queues and registers the queue-age gauge.
func (w *Worker) Start(ctx context.Context, instruments telemetry.Instruments) error {
	if err := w.client.Start(ctx); err != nil {
		return fmt.Errorf("start river client: %w", err)
	}
	if instruments.JobOldestPending != nil {
		registration, err := instruments.ObserveJobOldestPending(w.OldestPendingSeconds)
		if err != nil {
			// Losing the gauge is not a reason to refuse to process jobs.
			slog.WarnContext(ctx, "job queue age gauge not registered", "error", err)
		} else {
			w.registration = registration
		}
	}
	return nil
}

// Stop drains in-flight jobs within ctx's deadline.
func (w *Worker) Stop(ctx context.Context) error {
	var errs []error
	if w.registration != nil {
		// Unregister first: the callback reads the pool, and the pool is closed
		// once shutdown finishes.
		errs = append(errs, w.registration.Unregister())
		w.registration = nil
	}
	if err := w.client.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		errs = append(errs, fmt.Errorf("stop river client: %w", err))
	}
	return errors.Join(errs...)
}

// OldestPendingSeconds reports how long the oldest runnable job has been
// waiting. Zero means nothing is waiting — which is different from the gauge
// being absent, and is exactly the distinction an alert needs.
//
// `scheduled` belongs in the state list alongside `available` and `retryable`:
// River parks a job there until its scheduler promotes it, so a backlog that
// has come due sits in `scheduled` and would otherwise be invisible. The
// `scheduled_at <= now()` guard is what keeps genuinely future work out — work
// that is not yet due is not backlog.
func (w *Worker) OldestPendingSeconds(ctx context.Context) (int64, error) {
	const query = `
		SELECT COALESCE(EXTRACT(EPOCH FROM (now() - min(scheduled_at)))::bigint, 0)
		FROM ` + Schema + `.river_job
		WHERE state IN ('available', 'retryable', 'scheduled') AND scheduled_at <= now()
	`
	var seconds int64
	if err := w.pool.QueryRow(ctx, query).Scan(&seconds); err != nil {
		return 0, fmt.Errorf("read oldest pending job age: %w", err)
	}
	if seconds < 0 {
		seconds = 0
	}
	return seconds, nil
}

// MigrateUp applies River's own schema migrations into Schema. River owns these
// tables and versions them itself, so they are deliberately not goose files
// under db/migrations/job/.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("job: a database pool is required")
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: Schema})
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("apply river migrations: %w", err)
	}
	return nil
}

// deadLetterHandler records a job that has exhausted its attempts into
// ops.job_failures (BR-JOB-08). Without it a permanently failed job is only a
// row in River's table with a state nobody watches.
type deadLetterHandler struct {
	pool *pgxpool.Pool
}

func (h *deadLetterHandler) HandleError(
	ctx context.Context, jobRow *rivertype.JobRow, err error,
) *river.ErrorHandlerResult {
	h.recordIfFinal(ctx, jobRow, err.Error())
	return nil
}

func (h *deadLetterHandler) HandlePanic(
	ctx context.Context, jobRow *rivertype.JobRow, panicVal any, _ string,
) *river.ErrorHandlerResult {
	// The middleware normally converts a panic into an error before River sees
	// it, so this path is the backstop for a panic raised outside the
	// middleware's reach. The stack is deliberately not persisted: job_failures
	// is triage data, and the stack is already on the span.
	h.recordIfFinal(ctx, jobRow, fmt.Sprintf("%v: %v", ErrPanic, panicVal))
	return nil
}

// recordIfFinal writes only on the attempt that exhausts the budget. Writing on
// every attempt would turn the dead-letter table into a retry log.
func (h *deadLetterHandler) recordIfFinal(ctx context.Context, jobRow *rivertype.JobRow, cause string) {
	if jobRow.Attempt < jobRow.MaxAttempts {
		return
	}
	const insert = `
		INSERT INTO ` + Schema + `.job_failures (kind, args, last_error)
		VALUES ($1, $2, $3)
	`
	args := jobRow.EncodedArgs
	if len(args) == 0 {
		args = []byte("{}")
	}
	// The job's own context is already cancelled by the time a final failure is
	// handled, so this uses a fresh bounded context: the whole point of the row
	// is that it survives the failure.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if _, err := h.pool.Exec(writeCtx, insert, jobRow.Kind, args, cause); err != nil {
		slog.ErrorContext(ctx, "failed to dead-letter job",
			"kind", jobRow.Kind, "job_id", jobRow.ID, "error", err)
		return
	}
	slog.ErrorContext(ctx, "job failed permanently",
		"kind", jobRow.Kind, "job_id", jobRow.ID,
		"queue", jobRow.Queue, "attempt", jobRow.Attempt, "max_attempts", jobRow.MaxAttempts)
}

var _ river.ErrorHandler = (*deadLetterHandler)(nil)
