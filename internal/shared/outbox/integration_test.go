//go:build integration

package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/shared/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// newTestPool applies the real migrations to the database named by
// TEST_DATABASE_URL and returns a pool. The migrations are the ones shipped in
// the binary, so a schema change that breaks the publisher fails here.
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
		t.Fatalf("create goose provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	_ = provider.Close()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(), "TRUNCATE ops.outbox_events, ops.job_failures"); err != nil {
		t.Fatalf("reset tables: %v", err)
	}
	return pool
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

// TestWriter_RolledBackTransactionLeavesNoEvent is the guarantee the outbox
// exists for: no business write, no event.
func TestWriter_RolledBackTransactionLeavesNoEvent(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := outbox.NewWriter().Write(ctx, tx, "user", "user.created", map[string]string{"id": "u1"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if got := countRows(t, pool, "SELECT count(*) FROM ops.outbox_events"); got != 0 {
		t.Fatalf("outbox rows after rollback = %d, want 0", got)
	}
}

func TestPublisher_DispatchesCommittedEventExactlyOnce(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	eventID, err := outbox.NewWriter().Write(ctx, tx, "user", "user.created", map[string]string{"id": "u1"})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	dispatcher := &countingDispatcher{}
	publisher := outbox.NewPublisher(pool, dispatcher, 10, time.Second)

	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if len(dispatcher.seen) != 1 {
		t.Fatalf("dispatched %d events, want 1", len(dispatcher.seen))
	}
	if dispatcher.seen[0].ID != eventID {
		t.Errorf("event id = %s, want %s", dispatcher.seen[0].ID, eventID)
	}
	// "user.created", not "user.user.created".
	//
	// This assertion used to expect the doubled form, and expecting it is why
	// the bug survived from P1.2 until P1.5 wired the first consumer: the
	// writer stored the caller's already-qualified event constant verbatim and
	// Topic() prefixed it again, so nothing a module subscribed to could ever
	// match. See TestTopicRoundTripsContractNames.
	if dispatcher.seen[0].Topic() != "user.created" {
		t.Errorf("topic = %q, want %q", dispatcher.seen[0].Topic(), "user.created")
	}

	// A second pass must not redeliver a published event.
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("second batch: %v", err)
	}
	if len(dispatcher.seen) != 1 {
		t.Fatalf("published event was redelivered: %d dispatches", len(dispatcher.seen))
	}
	if got := countRows(t, pool, "SELECT count(*) FROM ops.outbox_events WHERE published_at IS NOT NULL"); got != 1 {
		t.Fatalf("published rows = %d, want 1", got)
	}
}

// TestPublisher_FailedDispatchBacksOffInsteadOfSpinning is the regression test
// for a dispatch failure that left the row untouched: it was retried every
// poll interval forever, and `attempts` was never written.
func TestPublisher_FailedDispatchBacksOffInsteadOfSpinning(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	writeEvent(t, pool, "user", "user.created")

	dispatcher := &countingDispatcher{err: errors.New("consumer down")}
	publisher := outbox.NewPublisher(pool, dispatcher, 10, time.Second)

	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("process batch: %v", err)
	}

	var attempts int
	var published, deadLettered *time.Time
	var nextAttempt time.Time
	err := pool.QueryRow(ctx, `SELECT attempts, published_at, dead_lettered_at, next_attempt_at FROM ops.outbox_events`).
		Scan(&attempts, &published, &deadLettered, &nextAttempt)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
	if published != nil || deadLettered != nil {
		t.Errorf("event should still be pending: published=%v dead=%v", published, deadLettered)
	}
	if !nextAttempt.After(time.Now()) {
		t.Errorf("next_attempt_at = %s, want a future time (backoff not applied)", nextAttempt)
	}

	// Immediately polling again must not pick it up: that is the backoff working.
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("second batch: %v", err)
	}
	if len(dispatcher.seen) != 1 {
		t.Fatalf("event was retried before its backoff elapsed: %d dispatches", len(dispatcher.seen))
	}
}

func TestPublisher_DeadLettersAfterMaxAttempts(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	writeEvent(t, pool, "user", "user.created")

	dispatcher := &countingDispatcher{err: errors.New("permanent failure")}
	publisher := outbox.NewPublisher(pool, dispatcher, 10, time.Second).WithMaxAttempts(3)

	for attempt := 1; attempt <= 3; attempt++ {
		if err := publisher.ProcessBatch(ctx); err != nil {
			t.Fatalf("batch %d: %v", attempt, err)
		}
		// Undo the backoff so the next pass is due.
		const dueNow = `UPDATE ops.outbox_events SET next_attempt_at = now() WHERE dead_lettered_at IS NULL`
		if _, err := pool.Exec(ctx, dueNow); err != nil {
			t.Fatalf("advance backoff: %v", err)
		}
	}

	if got := countRows(t, pool, "SELECT count(*) FROM ops.outbox_events WHERE dead_lettered_at IS NOT NULL"); got != 1 {
		t.Fatalf("dead-lettered rows = %d, want 1", got)
	}
	const failuresForKind = "SELECT count(*) FROM ops.job_failures WHERE kind = $1"
	// The dead-letter kind is "outbox." + the topic, so it carried the doubled
	// name too.
	if got := countRows(t, pool, failuresForKind, "outbox.user.created"); got != 1 {
		t.Fatalf("job_failures rows = %d, want 1", got)
	}

	// A dead-lettered event is never polled again.
	before := len(dispatcher.seen)
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("batch after dead-letter: %v", err)
	}
	if len(dispatcher.seen) != before {
		t.Fatalf("dead-lettered event was redelivered")
	}
}

func TestPublisher_EventIDIsUnique(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	writeEvent(t, pool, "user", "user.created")
	var eventID string
	if err := pool.QueryRow(ctx, `SELECT event_id FROM ops.outbox_events`).Scan(&eventID); err != nil {
		t.Fatalf("read id: %v", err)
	}
	const duplicate = `INSERT INTO ops.outbox_events (event_id, aggregate, event, payload)
		VALUES ($1, 'user', 'user.created', '{}')`
	_, err := pool.Exec(ctx, duplicate, eventID)
	if err == nil {
		t.Fatal("expected the unique index on event_id to reject a duplicate")
	}
}

func writeEvent(t *testing.T, pool *pgxpool.Pool, aggregate, name string) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := outbox.NewWriter().Write(ctx, tx, aggregate, name, map[string]string{"id": "u1"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
