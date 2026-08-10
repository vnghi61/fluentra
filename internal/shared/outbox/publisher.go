package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
)

const (
	defaultBatchSize   = 50
	defaultPollFreq    = 500 * time.Millisecond
	defaultMaxAttempts = 5
	backoffBase        = time.Second
	backoffCeiling     = 5 * time.Minute
)

// Event is one outbox row as delivered to a consumer.
type Event struct {
	// ID is the deduplication key. Delivery is at-least-once, so a consumer
	// that is not idempotent on this value will double-apply an effect.
	ID        uuid.UUID
	Aggregate string
	Name      string
	Payload   json.RawMessage
	Attempt   int

	// TraceParent is the W3C traceparent of the transaction that wrote the
	// event, or "" when it was written outside a span.
	//
	// Consumers do not read it. The publisher restores it into the context it
	// dispatches with, so a handler that asks the context for its trace gets
	// the producer's — which is the whole point, and is why nothing downstream
	// had to change to get it.
	TraceParent string
}

// Topic is the event name a broker would route on.
func (e Event) Topic() string { return e.Aggregate + "." + e.Name }

// DispatchContext returns ctx with the event's originating span restored, so
// the delivery continues the trace that produced it instead of starting a new
// one.
//
// An event with no traceparent, or one the propagator will not parse, returns
// ctx unchanged: a delivery that loses its trace is worth less than one that
// keeps it, and far more than one that fails.
//
// The Publisher calls this for every event. It is exported because a broker
// implementation replacing the Publisher would have to do the same thing, and
// having to reimplement it is how the trace would go missing again.
func (e Event) DispatchContext(ctx context.Context) context.Context {
	if e.TraceParent == "" {
		return ctx
	}
	return tracePropagator.Extract(ctx, propagation.MapCarrier{traceParentHeader: e.TraceParent})
}

// EventDispatcher delivers one outbox event. Every type in the signature is
// exported, so an implementation can live in any package.
type EventDispatcher interface {
	Dispatch(ctx context.Context, event Event) error
}

// Publisher polls unpublished events and dispatches them.
type Publisher struct {
	pool        *pgxpool.Pool
	dispatcher  EventDispatcher
	batchSize   int
	pollFreq    time.Duration
	maxAttempts int
	now         func() time.Time
}

// NewPublisher creates an outbox publisher daemon and registers the
// `outbox_lag_seconds` gauge against it.
func NewPublisher(pool *pgxpool.Pool, dispatcher EventDispatcher, batchSize int, pollFreq time.Duration) *Publisher {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if pollFreq <= 0 {
		pollFreq = defaultPollFreq
	}
	publisher := &Publisher{
		pool:        pool,
		dispatcher:  dispatcher,
		batchSize:   batchSize,
		pollFreq:    pollFreq,
		maxAttempts: defaultMaxAttempts,
		now:         time.Now,
	}
	if err := publisher.registerLagGauge(); err != nil {
		slog.Warn("outbox lag gauge unavailable", "error", err)
	}
	return publisher
}

// WithMaxAttempts sets how many dispatch attempts an event gets before it is
// dead-lettered.
func (p *Publisher) WithMaxAttempts(attempts int) *Publisher {
	if attempts > 0 {
		p.maxAttempts = attempts
	}
	return p
}

// registerLagGauge publishes the age of the oldest event still waiting for
// dispatch. Without it a stalled publisher is invisible until someone notices
// the effects are missing.
func (p *Publisher) registerLagGauge() error {
	meter := otel.Meter("fluentra.shared.outbox")
	gauge, err := meter.Float64ObservableGauge(
		"outbox_lag_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("Age of the oldest outbox event that has not been dispatched"),
	)
	if err != nil {
		return fmt.Errorf("create outbox lag gauge: %w", err)
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		lag, err := p.oldestPendingAge(ctx)
		if err != nil {
			return err
		}
		observer.ObserveFloat64(gauge, lag.Seconds())
		return nil
	}, gauge)
	if err != nil {
		return fmt.Errorf("register outbox lag callback: %w", err)
	}
	return nil
}

func (p *Publisher) oldestPendingAge(ctx context.Context) (time.Duration, error) {
	if p.pool == nil {
		return 0, nil
	}
	const query = `
		SELECT COALESCE(EXTRACT(EPOCH FROM (now() - MIN(created_at))), 0)
		FROM ops.outbox_events
		WHERE published_at IS NULL AND dead_lettered_at IS NULL
	`
	var seconds float64
	if err := p.pool.QueryRow(ctx, query).Scan(&seconds); err != nil {
		return 0, fmt.Errorf("query outbox lag: %w", err)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

// Run executes the polling loop until context is canceled.
func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.pollFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.ProcessBatch(ctx); err != nil {
				slog.ErrorContext(ctx, "outbox batch processing failed", "error", err)
			}
		}
	}
}

// ProcessBatch dispatches one batch of due events. Dispatch happens inside the
// transaction that holds the row locks, so an event is never marked published
// unless the consumer accepted it, and a crash mid-batch redelivers rather
// than drops.
func (p *Publisher) ProcessBatch(ctx context.Context) error {
	if p.pool == nil {
		return nil
	}
	if p.dispatcher == nil {
		// A publisher with nowhere to deliver must not mark anything published;
		// doing so would discard every event the application ever wrote.
		return fmt.Errorf("outbox publisher has no dispatcher configured")
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin outbox transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const selectQuery = `
		SELECT event_id, aggregate, event, payload, attempts, COALESCE(traceparent, '')
		FROM ops.outbox_events
		WHERE published_at IS NULL
		  AND dead_lettered_at IS NULL
		  AND next_attempt_at <= now()
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, selectQuery, p.batchSize)
	if err != nil {
		return fmt.Errorf("query outbox events: %w", err)
	}

	batch := make([]Event, 0, p.batchSize)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Aggregate, &event.Name,
			&event.Payload, &event.Attempt, &event.TraceParent); err != nil {
			rows.Close()
			return fmt.Errorf("scan outbox event: %w", err)
		}
		batch = append(batch, event)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read outbox events: %w", err)
	}
	if len(batch) == 0 {
		return nil
	}

	for _, event := range batch {
		// Dispatched in the trace the event was written in, not the trace of
		// this polling loop. The bookkeeping below stays on ctx: marking a row
		// published belongs to the publisher, not to the request that caused
		// the event months of partitions ago.
		dispatchErr := p.dispatcher.Dispatch(event.DispatchContext(ctx), event)
		if dispatchErr == nil {
			const markPublished = `UPDATE ops.outbox_events SET published_at = now() WHERE event_id = $1`
			if _, err := tx.Exec(ctx, markPublished, event.ID); err != nil {
				return fmt.Errorf("mark outbox event published: %w", err)
			}
			continue
		}
		if err := p.recordFailure(ctx, tx, event, dispatchErr); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// recordFailure applies backoff, or dead-letters the event once it has used up
// its attempts. Without this an event that can never succeed is retried at the
// poll interval forever and starves the queue behind it.
func (p *Publisher) recordFailure(ctx context.Context, tx DBTx, event Event, cause error) error {
	attempt := event.Attempt + 1
	if attempt >= p.maxAttempts {
		const deadLetter = `
			INSERT INTO ops.job_failures (kind, args, last_error)
			VALUES ($1, $2, $3)
		`
		if _, err := tx.Exec(ctx, deadLetter, "outbox."+event.Topic(), []byte(event.Payload), cause.Error()); err != nil {
			return fmt.Errorf("dead-letter outbox event: %w", err)
		}
		const markDead = `
			UPDATE ops.outbox_events
			SET attempts = $2, dead_lettered_at = now(), last_error = $3
			WHERE event_id = $1
		`
		if _, err := tx.Exec(ctx, markDead, event.ID, attempt, cause.Error()); err != nil {
			return fmt.Errorf("mark outbox event dead-lettered: %w", err)
		}
		slog.ErrorContext(ctx, "outbox event dead-lettered",
			"event_id", event.ID.String(), "attempt", attempt, "error", cause)
		return nil
	}

	const retry = `
		UPDATE ops.outbox_events
		SET attempts = $2, next_attempt_at = now() + $3::interval, last_error = $4
		WHERE event_id = $1
	`
	delay := BackoffFor(attempt)
	if _, err := tx.Exec(ctx, retry, event.ID, attempt, delay.String(), cause.Error()); err != nil {
		return fmt.Errorf("schedule outbox retry: %w", err)
	}
	slog.WarnContext(ctx, "outbox event dispatch failed",
		"event_id", event.ID.String(), "attempt", attempt,
		"max_attempts", p.maxAttempts, "error", cause)
	return nil
}

// BackoffFor returns the delay before the given attempt is retried: 1s, 2s,
// 4s… capped at five minutes.
func BackoffFor(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := backoffBase << (attempt - 1)
	if delay > backoffCeiling || delay <= 0 {
		return backoffCeiling
	}
	return delay
}
