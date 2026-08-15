package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/fluentra/fluentra/internal/platform/telemetry"
)

var tracer = otel.Tracer("fluentra.platform.job")

// ErrPanic wraps a value recovered from a panicking job handler. It is an
// ordinary error by the time River sees it, so the job follows the normal retry
// and dead-letter path instead of taking the process down.
var ErrPanic = errors.New("job: handler panicked")

// DefaultJobTimeout bounds a handler that never returns. BR-JOB-04 requires
// every job to declare a timeout; this is the ceiling applied when it does not.
const DefaultJobTimeout = 5 * time.Minute

// Middleware wraps every job handler with the five things a handler must never
// have to remember: a span, a structured log line, panic recovery, a timeout,
// and metrics.
//
// Panic recovery lives here rather than in each handler on purpose. A handler
// that forgets to recover would otherwise kill the worker goroutine and take
// every other queue down with it, and "every handler remembers" is not a
// property anyone can enforce in review.
type Middleware struct {
	river.MiddlewareDefaults

	timeout     time.Duration
	instruments telemetry.Instruments
	metering    bool
}

// NewMiddleware builds the worker middleware. A non-positive timeout falls back
// to DefaultJobTimeout.
func NewMiddleware(timeout time.Duration, instruments telemetry.Instruments) *Middleware {
	if timeout <= 0 {
		timeout = DefaultJobTimeout
	}
	return &Middleware{
		timeout:     timeout,
		instruments: instruments,
		metering:    instruments.JobDuration != nil && instruments.JobAttempts != nil,
	}
}

// Work runs one job handler under the middleware's guarantees.
func (m *Middleware) Work(
	ctx context.Context, jobRow *rivertype.JobRow, doInner func(context.Context) error,
) (err error) {
	started := time.Now()

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	// The span name is the job kind, never the job ID: a span name carrying an
	// identifier makes every execution its own operation and the trace backend
	// can no longer aggregate them.
	ctx, span := tracer.Start(ctx, "job."+jobRow.Kind, trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("job.kind", jobRow.Kind),
			attribute.String("job.queue", jobRow.Queue),
			attribute.Int64("job.id", jobRow.ID),
			attribute.Int("job.attempt", jobRow.Attempt),
		))
	defer span.End()

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", ErrPanic, recovered)
			// The stack goes on the span, not into the log: the log allowlist is
			// fail-closed by design (LG6) and a stack trace is unbounded text
			// that routinely quotes arguments.
			span.RecordError(err, trace.WithStackTrace(true))
		}

		duration := time.Since(started)
		result := resultLabel(err)
		if err != nil {
			span.SetStatus(codes.Error, result)
		}
		m.record(ctx, jobRow, result, duration)
		m.log(ctx, jobRow, result, duration, err)
	}()

	return doInner(ctx)
}

// record emits duration and attempt counts with bounded labels. Queue and kind
// are both closed sets; job ID deliberately is not a label.
func (m *Middleware) record(
	ctx context.Context, jobRow *rivertype.JobRow, result string, duration time.Duration,
) {
	if !m.metering {
		return
	}
	attributes := metric.WithAttributes(
		attribute.String("queue", jobRow.Queue),
		attribute.String("kind", jobRow.Kind),
		attribute.String("result", result),
	)
	m.instruments.JobDuration.Record(ctx, duration.Seconds(), attributes)
	m.instruments.JobAttempts.Add(ctx, 1, attributes)
}

func (m *Middleware) log(
	ctx context.Context, jobRow *rivertype.JobRow, result string, duration time.Duration, err error,
) {
	attrs := []any{
		"kind", jobRow.Kind,
		"queue", jobRow.Queue,
		"job_id", jobRow.ID,
		"attempt", jobRow.Attempt,
		"max_attempts", jobRow.MaxAttempts,
		"result", result,
		"duration_ms", duration.Milliseconds(),
	}
	if err != nil {
		slog.ErrorContext(ctx, "job failed", append(attrs, "error", err)...)
		return
	}
	slog.InfoContext(ctx, "job completed", attrs...)
}

// resultLabel keeps the metric label a closed set. A timeout is separated from
// an ordinary failure because the two need different operational responses:
// one means the handler is too slow, the other that it is broken.
func resultLabel(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, ErrPanic):
		return "panic"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "error"
	}
}

var _ rivertype.WorkerMiddleware = (*Middleware)(nil)
