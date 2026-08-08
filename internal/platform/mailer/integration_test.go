//go:build integration

package mailer_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/platform/mailer"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	sources, err := migrations.Flattened()
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = provider.Close()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE comm.email_log, comm.email_suppressions"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	return pool
}

// TestPostgresSuppressionStore_SurvivesRestart is the P0.11 acceptance a
// memory-only store could never satisfy: a hard bounce must still be
// suppressed after the process that saw it is gone.
func TestPostgresSuppressionStore_SurvivesRestart(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	first := mailer.NewPostgresSuppressionStore(pool)
	if err := first.SuppressAddress(ctx, "bounced@example.com", "hard_bounce"); err != nil {
		t.Fatalf("suppress: %v", err)
	}

	// A different store value stands in for a restarted process.
	second := mailer.NewPostgresSuppressionStore(pool)
	suppressed, reason, err := second.IsSuppressed(ctx, "BOUNCED@Example.com")
	if err != nil {
		t.Fatalf("is suppressed: %v", err)
	}
	if !suppressed || reason != "hard_bounce" {
		t.Fatalf("suppressed = %v, reason = %q; want the suppression to persist", suppressed, reason)
	}

	clean, _, err := second.IsSuppressed(ctx, "fine@example.com")
	if err != nil {
		t.Fatalf("is suppressed: %v", err)
	}
	if clean {
		t.Error("an unrelated address was reported as suppressed")
	}
}

func TestPostgresSuppressionStore_SuppressingTwiceUpdatesTheReason(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	store := mailer.NewPostgresSuppressionStore(pool)

	if err := store.SuppressAddress(ctx, "x@example.com", "hard_bounce"); err != nil {
		t.Fatal(err)
	}
	if err := store.SuppressAddress(ctx, "x@example.com", "complaint"); err != nil {
		t.Fatalf("second suppression should upsert, not conflict: %v", err)
	}
	_, reason, err := store.IsSuppressed(ctx, "x@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if reason != "complaint" {
		t.Errorf("reason = %q, want complaint", reason)
	}
}

// TestPostgresRecorder_StoresOnlyHashedRecipients checks the delivery log
// carries no plaintext address.
func TestPostgresRecorder_StoresOnlyHashedRecipients(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	recorder := mailer.NewPostgresRecorder(pool)
	if err := recorder.Record(ctx, mailer.LogEntry{
		ToHash:   mailer.HashEmail("learner@example.com"),
		Template: "verify_email",
		Locale:   "vi",
		Status:   "sent",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	var toHash, template, locale, status string
	err := pool.QueryRow(ctx, `SELECT to_hash, template, locale, status FROM comm.email_log`).
		Scan(&toHash, &template, &locale, &status)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if toHash != mailer.HashEmail("learner@example.com") {
		t.Errorf("to_hash = %q", toHash)
	}
	if template != "verify_email" || locale != "vi" || status != "sent" {
		t.Errorf("row = %q/%q/%q", template, locale, status)
	}

	var plaintextRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM comm.email_log WHERE to_hash LIKE '%@%'`).Scan(&plaintextRows); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if plaintextRows != 0 {
		t.Errorf("%d log rows contain a plaintext address", plaintextRows)
	}
}
