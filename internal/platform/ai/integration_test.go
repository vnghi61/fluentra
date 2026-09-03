//go:build integration

package ai_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// DBCache and DBUsageRecorder write raw SQL, and until this file nothing ran a
// line of it against a real schema.
//
// The unit tests cover the router with a fake recorder and an in-memory cache,
// which is the half that cannot break in production. The half that can is the
// SQL: `ai_requests` constrains `status`, `ai_usage` upserts on a four-column
// key, and `ai_cache_entries` is written on one path and read on another. A
// misspelled column in any of them compiles, passes every unit test, and then
// fails at runtime -- where `Set` discards its error and `Get` treats every
// error as a miss, so the visible symptom is not an error at all. It is a cache
// that never fills and a bill that keeps growing, which is exactly the cost the
// cache exists to prevent.
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
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE ai.ai_cache_entries, ai.ai_requests, ai.ai_usage"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	return pool
}

// TestClientWithPool_CachesInPostgresAndRecordsUsage is the acceptance a
// memory-only cache could not satisfy: the second identical request costs
// nothing, and it still costs nothing after the process that answered the first
// one is gone. On a sleeping free dyno that is every request.
func TestClientWithPool_CachesInPostgresAndRecordsUsage(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	client, err := ai.New(ai.Config{Provider: ai.ProviderMock, Pool: pool})
	require.NoError(t, err)

	req := ai.Request{
		Task: ai.TaskVerifyVocabulary,
		Vars: map[string]any{fieldTerm: termLeisure},
	}

	first, err := client.Complete(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, first.Text)

	// The row is in Postgres, not in a map that dies with the process.
	var cached int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM ai.ai_cache_entries WHERE expires_at > now()`).Scan(&cached))
	assert.Equal(t, 1, cached, "a completed request must leave a cache entry behind")

	// A second client, as a restarted worker would build. It shares no memory
	// with the first and must still be served from cache.
	restarted, err := ai.New(ai.Config{Provider: ai.ProviderMock, Pool: pool})
	require.NoError(t, err)

	second, err := restarted.Complete(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, first.Text, second.Text, "the cached answer must survive the process")

	// Two requests, and the second one spent nothing.
	var statuses []string
	rows, err := pool.Query(ctx, `SELECT status FROM ai.ai_requests ORDER BY created_at, status`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var status string
		require.NoError(t, rows.Scan(&status))
		statuses = append(statuses, status)
	}
	require.NoError(t, rows.Err())
	assert.ElementsMatch(t,
		[]string{string(ai.StatusSuccess), string(ai.StatusCached)}, statuses,
		"one provider call and one cache hit")

	// And the aggregate upsert -- the four-column conflict target no unit test
	// can exercise -- actually applied.
	var usageRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM ai.ai_usage`).Scan(&usageRows))
	assert.Positive(t, usageRows, "ai_usage must aggregate what ai_requests recorded")
}

// TestDBUsageRecorder_RejectsAStatusTheConstraintDoesNotAllow pins the CHECK
// from the Go side.
//
// TestRequestStatus_MatchesTheCheckConstraint reads the migration and compares
// strings, which catches a constant drifting from the constraint. This catches
// the other direction: that the insert really is subject to the constraint at
// all, and that a status invented at a call site is refused rather than stored.
func TestDBUsageRecorder_RejectsAStatusTheConstraintDoesNotAllow(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	err := ai.NewDBUsageRecorder(pool).Record(ctx, ai.RequestLog{
		Task:     ai.TaskVerifyVocabulary,
		Provider: "mock",
		Model:    "m",
		Status:   "ok", // The obvious wrong guess.
	})
	require.Error(t, err, "ck_ai_requests_status must refuse a status no constant names")

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM ai.ai_requests`).Scan(&count))
	assert.Equal(t, 0, count, "a refused insert must leave nothing behind")
}
