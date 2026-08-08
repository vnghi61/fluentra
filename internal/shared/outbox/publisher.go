package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EventDispatcher delivers outbox events to subscribers.
type EventDispatcher interface {
	Dispatch(ctx context.Context, aggregate, event string, payload jsonbMessage) error
}

type jsonbMessage json.RawMessage

// Publisher polls unpublished events and dispatches them.
type Publisher struct {
	pool       *pgxpool.Pool
	dispatcher EventDispatcher
	batchSize  int
	pollFreq   time.Duration
}

// NewPublisher creates an outbox publisher daemon.
func NewPublisher(pool *pgxpool.Pool, dispatcher EventDispatcher, batchSize int, pollFreq time.Duration) *Publisher {
	if batchSize <= 0 {
		batchSize = 50
	}
	if pollFreq <= 0 {
		pollFreq = 500 * time.Millisecond
	}
	return &Publisher{
		pool:       pool,
		dispatcher: dispatcher,
		batchSize:  batchSize,
		pollFreq:   pollFreq,
	}
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
				slog.ErrorContext(ctx, "outbox batch processing error", "error", err)
			}
		}
	}
}

// ProcessBatch locks and processes a batch of unpublished outbox events.
func (p *Publisher) ProcessBatch(ctx context.Context) error {
	if p.pool == nil {
		return nil
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin outbox transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const selectQuery = `
		SELECT id, aggregate, event, payload
		FROM ops.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(ctx, selectQuery, p.batchSize)
	if err != nil {
		return fmt.Errorf("query outbox events: %w", err)
	}

	type record struct {
		id        string
		aggregate string
		event     string
		payload   json.RawMessage
	}

	var batch []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.id, &r.aggregate, &r.event, &r.payload); err != nil {
			rows.Close()
			return fmt.Errorf("scan outbox event: %w", err)
		}
		batch = append(batch, r)
	}
	rows.Close()

	if len(batch) == 0 {
		return nil
	}

	const updateQuery = `
		UPDATE ops.outbox_events
		SET published_at = now()
		WHERE id = $1
	`

	for _, r := range batch {
		if p.dispatcher != nil {
			if err := p.dispatcher.Dispatch(ctx, r.aggregate, r.event, jsonbMessage(r.payload)); err != nil {
				slog.WarnContext(ctx, "outbox event dispatch failed", "id", r.id, "event", r.event, "error", err)
				continue
			}
		}
		if _, err := tx.Exec(ctx, updateQuery, r.id); err != nil {
			return fmt.Errorf("mark outbox event published: %w", err)
		}
	}

	return tx.Commit(ctx)
}
