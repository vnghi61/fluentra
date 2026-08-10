//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// testUserID is the aggregate id these tests round-trip through the outbox.
const testUserID = "u-1"

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	sources, err := migrations.Flattened()
	if err != nil {
		t.Fatalf("flatten migrations: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	_ = provider.Close()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE ops.outbox_events, ops.job_failures"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	return pool
}

// TestOutboxReachesEventBus is the end-to-end proof for P0.10 + P0.12: a
// committed outbox row is delivered to a handler registered on the bus. The
// worker previously ran with a nil dispatcher, which marked every event
// published and delivered none of them.
func TestOutboxReachesEventBus(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	var received eventbus.Message
	var deliveries int
	if err := bus.Subscribe("user.created", func(_ context.Context, message eventbus.Message) error {
		deliveries++
		received = message
		return nil
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	eventID, err := outbox.NewWriter().Write(ctx, tx, "user", "user.created", map[string]string{"id": testUserID})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	publisher := outbox.NewPublisher(pool, busDispatcher{bus: bus}, 10, time.Second)
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch: %v", err)
	}

	if deliveries != 1 {
		t.Fatalf("handler ran %d times, want 1", deliveries)
	}
	if received.ID != eventID {
		t.Errorf("message id = %s, want the outbox event id %s", received.ID, eventID)
	}
	// jsonb normalises whitespace, so compare the decoded value.
	var payload map[string]string
	if err := json.Unmarshal(received.Payload, &payload); err != nil {
		t.Fatalf("payload is not JSON: %s", received.Payload)
	}
	if payload["id"] != testUserID {
		t.Errorf("payload = %s", received.Payload)
	}

	var published *time.Time
	const readPublished = `SELECT published_at FROM ops.outbox_events WHERE event_id = $1`
	if err := pool.QueryRow(ctx, readPublished, eventID).Scan(&published); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if published == nil {
		t.Error("event was delivered but not marked published")
	}
}

// TestFailingHandlerKeepsEventPending proves a consumer failure is not
// swallowed: the row stays pending so the outbox retries it.
func TestFailingHandlerKeepsEventPending(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	_ = bus.Subscribe("user.created", func(context.Context, eventbus.Message) error {
		return errors.New("consumer is down")
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := outbox.NewWriter().Write(
		ctx, tx, "user", "user.created", map[string]string{"id": testUserID}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	publisher := outbox.NewPublisher(pool, busDispatcher{bus: bus}, 10, time.Second)
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch: %v", err)
	}

	var attempts int
	var published *time.Time
	const readAttempts = `SELECT attempts, published_at FROM ops.outbox_events`
	if err := pool.QueryRow(ctx, readAttempts).Scan(&attempts, &published); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if attempts != 1 || published != nil {
		t.Fatalf("attempts = %d, published_at = %v; want the event still pending after one failure",
			attempts, published)
	}
}
