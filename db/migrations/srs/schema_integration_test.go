//go:build integration

// Package srs_test verifies the srs half of the `learn` schema — the review
// cards, the monthly-partitioned review logs, and the partition function — against
// a real PostgreSQL instance.
//
// The `learn`-wide rules (every foreign key indexed, no cross-schema key outside
// core.users) are asserted once, for the whole schema, in
// db/migrations/learning/schema_integration_test.go. Those tests query
// nspname = 'learn' and so already cover the tables created here; this file
// asserts only what is specific to srs.
package srs_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

const bootstrapVersion = 1700000000
const srsSchemaVersion = 1700000220

const schemaDatabase = "fluentra_srs_schema_test"
const downDatabase = "fluentra_srs_schema_down_test"

var packagePool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, schemaDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", schemaDatabase, err)
		os.Exit(1)
	}
	provider, err := migrateUp(dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", schemaDatabase, err)
		os.Exit(1)
	}
	_ = provider.Close()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", schemaDatabase, err)
		os.Exit(1)
	}
	packagePool = pool

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
}

func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return packagePool
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

func privateDatabase(t *testing.T, name string) (*pgxpool.Pool, *goose.Provider) {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	dsn, dropDatabase, err := createDatabase(base, name)
	if err != nil {
		t.Fatalf("prepare %s: %v", name, err)
	}
	t.Cleanup(dropDatabase)

	provider, err := migrateUp(dsn)
	if err != nil {
		t.Fatalf("migrate %s: %v", name, err)
	}
	t.Cleanup(func() { _ = provider.Close() })

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool for %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	return pool, provider
}

func migrateUp(dsn string) (*goose.Provider, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		_ = provider.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return provider, nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

// seedUser inserts a learner and removes it again when the test ends.
func seedUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	ctx := context.Background()

	var userID string
	const insert = `INSERT INTO core.users (email, status) VALUES ($1, 'active') RETURNING id`
	if err := pool.QueryRow(ctx, insert, email).Scan(&userID); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})
	return userID
}

// seedCard inserts a review card for a learner and returns its id.
func seedCard(t *testing.T, pool *pgxpool.Pool, userID string, dueAt time.Time) string {
	t.Helper()
	ctx := context.Background()

	var cardID string
	const insert = `
		INSERT INTO learn.review_cards (user_id, content_version_id, skill, stability, difficulty, due_at, state)
		VALUES ($1, gen_random_uuid(), 'vocabulary', 3.1262, 5.0, $2, 'review')
		RETURNING id`
	if err := pool.QueryRow(ctx, insert, userID, dueAt).Scan(&cardID); err != nil {
		t.Fatalf("seed review card: %v", err)
	}
	return cardID
}

// TestSRSSchema_PartitionsPreCreatedAndTargeted is the headline gate for P9.3.
//
// Monthly partitioning with no initial partitions fails on the very first
// insert, and a partition set that stops at the current month fails silently on
// the first of the next one. Both are production outages, so the migration
// pre-creates the current month and the three that follow, and this test proves
// it two ways: the partitions exist, and a log dated next month actually lands
// in next month's partition rather than erroring or falling somewhere else.
func TestSRSSchema_PartitionsPreCreatedAndTargeted(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	nowUTC := time.Now().UTC()
	currentMonthStart := time.Date(nowUTC.Year(), nowUTC.Month(), 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i <= 2; i++ {
		monthTime := currentMonthStart.AddDate(0, i, 0)
		partitionName := fmt.Sprintf("review_logs_y%04dm%02d", monthTime.Year(), int(monthTime.Month()))

		var exists bool
		const checkQuery = `SELECT to_regclass('learn.' || $1) IS NOT NULL`
		if err := pool.QueryRow(ctx, checkQuery, partitionName).Scan(&exists); err != nil {
			t.Fatalf("check partition %s: %v", partitionName, err)
		}
		if !exists {
			t.Errorf("expected partition learn.%s to exist after migration, but it does not", partitionName)
		}
	}

	userID := seedUser(t, pool, "srs-partition-test@example.com")
	cardID := seedCard(t, pool, userID, nowUTC)

	nextMonth := currentMonthStart.AddDate(0, 1, 14)
	expectedPartition := fmt.Sprintf("review_logs_y%04dm%02d", nextMonth.Year(), int(nextMonth.Month()))

	var insertedID, actualPartition string
	const insertNextMonth = `
		INSERT INTO learn.review_logs (
			card_id, user_id, grade, elapsed_ms,
			stability_before, stability_after, difficulty_before, difficulty_after,
			scheduled_days, scheduler_version, reviewed_at)
		VALUES ($1, $2, 'good', 2100, 3.1262, 8.4, 5.0, 4.8, 8, 'v4.5', $3)
		RETURNING id, tableoid::regclass::text`

	err := pool.QueryRow(ctx, insertNextMonth, cardID, userID, nextMonth).Scan(&insertedID, &actualPartition)
	if err != nil {
		t.Fatalf("insert review log dated next month: %v", err)
	}

	// regclass renders as `learn.review_logs_yYYYYmMM` or, when learn is on the
	// search path, as the bare relation name.
	if !strings.HasSuffix(actualPartition, expectedPartition) {
		t.Errorf("review log landed in partition %q, want suffix %q", actualPartition, expectedPartition)
	}
}

// TestSRSSchema_EnsureSRSPartitionsIdempotent proves the rotation job can run on
// every worker boot and on its six-hourly schedule without creating duplicates.
func TestSRSSchema_EnsureSRSPartitionsIdempotent(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var createdCount int
	if err := pool.QueryRow(ctx, `SELECT learn.ensure_srs_partitions(3)`).Scan(&createdCount); err != nil {
		t.Fatalf("ensure_srs_partitions(3): %v", err)
	}
	if createdCount != 0 {
		t.Errorf("second run of ensure_srs_partitions created %d partitions, want 0 (idempotent)", createdCount)
	}
}

// TestSRSSchema_EnsureSRSPartitionsRejectsAbsurdHorizons: the guard exists so a
// mistyped call cannot create years of empty partitions.
func TestSRSSchema_EnsureSRSPartitionsRejectsAbsurdHorizons(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	for _, monthsAhead := range []int{-1, 25} {
		var created int
		err := pool.QueryRow(ctx, `SELECT learn.ensure_srs_partitions($1)`, monthsAhead).Scan(&created)
		if err == nil {
			t.Errorf("ensure_srs_partitions(%d) succeeded, want an error", monthsAhead)
		}
	}
}

// TestSRSSchema_CheckConstraintsEnforced walks every check constraint the srs
// tables declare and proves each one actually fires. A constraint nobody has
// seen reject a row is a comment.
func TestSRSSchema_CheckConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	userID := seedUser(t, pool, "srs-checks-test@example.com")
	cardID := seedCard(t, pool, userID, time.Now().UTC())

	cases := []struct {
		name       string
		constraint string
		statement  string
		args       []any
	}{
		{
			name:       "skill outside the six skills",
			constraint: "ck_review_cards_skill",
			statement: `INSERT INTO learn.review_cards (user_id, content_version_id, skill, due_at)
			            VALUES ($1, gen_random_uuid(), 'telepathy', now())`,
			args: []any{userID},
		},
		{
			name:       "state outside the FSRS lifecycle",
			constraint: "ck_review_cards_state",
			statement: `INSERT INTO learn.review_cards (user_id, content_version_id, skill, state, due_at)
			            VALUES ($1, gen_random_uuid(), 'vocabulary', 'forgotten', now())`,
			args: []any{userID},
		},
		{
			name:       "negative stability",
			constraint: "ck_review_cards_stability_nonnegative",
			statement: `INSERT INTO learn.review_cards (user_id, content_version_id, skill, stability, due_at)
			            VALUES ($1, gen_random_uuid(), 'vocabulary', -1.0, now())`,
			args: []any{userID},
		},
		{
			name:       "difficulty above the FSRS ceiling",
			constraint: "ck_review_cards_difficulty_range",
			statement: `INSERT INTO learn.review_cards (user_id, content_version_id, skill, difficulty, due_at)
			            VALUES ($1, gen_random_uuid(), 'vocabulary', 11.0, now())`,
			args: []any{userID},
		},
		{
			name:       "negative reps",
			constraint: "ck_review_cards_reps_nonnegative",
			statement: `INSERT INTO learn.review_cards (user_id, content_version_id, skill, reps, due_at)
			            VALUES ($1, gen_random_uuid(), 'vocabulary', -1, now())`,
			args: []any{userID},
		},
		{
			name:       "negative lapses",
			constraint: "ck_review_cards_lapses_nonnegative",
			statement: `INSERT INTO learn.review_cards (user_id, content_version_id, skill, lapses, due_at)
			            VALUES ($1, gen_random_uuid(), 'vocabulary', -1, now())`,
			args: []any{userID},
		},
		{
			name:       "a fifth grade",
			constraint: "ck_review_logs_grade",
			statement: `INSERT INTO learn.review_logs (card_id, user_id, grade, reviewed_at)
			            VALUES ($1, $2, 'perfect', now())`,
			args: []any{cardID, userID},
		},
		{
			name:       "negative elapsed time",
			constraint: "ck_review_logs_elapsed_ms",
			statement: `INSERT INTO learn.review_logs (card_id, user_id, grade, elapsed_ms, reviewed_at)
			            VALUES ($1, $2, 'good', -1, now())`,
			args: []any{cardID, userID},
		},
		{
			name:       "negative scheduled days",
			constraint: "ck_review_logs_scheduled_days",
			statement: `INSERT INTO learn.review_logs (card_id, user_id, grade, scheduled_days, reviewed_at)
			            VALUES ($1, $2, 'good', -1, now())`,
			args: []any{cardID, userID},
		},
		{
			name:       "retention outside (0, 1)",
			constraint: "ck_srs_params_retention",
			statement: `INSERT INTO learn.srs_params (user_id, weights, request_retention)
			            VALUES ($1, '{}'::jsonb, 1.0)`,
			args: []any{userID},
		},
		{
			name:       "non-positive max interval",
			constraint: "ck_srs_params_max_interval",
			statement: `INSERT INTO learn.srs_params (user_id, weights, max_interval)
			            VALUES ($1, '{}'::jsonb, 0)`,
			args: []any{userID},
		},
		{
			name:       "negative reviews completed",
			constraint: "ck_review_daily_stats_reviews_completed",
			statement: `INSERT INTO learn.review_daily_stats (user_id, stat_date, reviews_completed)
			            VALUES ($1, current_date, -1)`,
			args: []any{userID},
		},
		{
			name:       "negative new cards learned",
			constraint: "ck_review_daily_stats_new_cards",
			statement: `INSERT INTO learn.review_daily_stats (user_id, stat_date, new_cards_learned)
			            VALUES ($1, current_date, -1)`,
			args: []any{userID},
		},
		{
			name:       "negative total minutes",
			constraint: "ck_review_daily_stats_total_minutes",
			statement: `INSERT INTO learn.review_daily_stats (user_id, stat_date, total_minutes)
			            VALUES ($1, current_date, -1)`,
			args: []any{userID},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.statement, tc.args...)
			assertConstraintViolation(t, err, tc.constraint)
		})
	}
}

// TestSRSSchema_UniqueConstraintsEnforced proves the two uniqueness rules the
// service relies on for its upserts.
func TestSRSSchema_UniqueConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	userID := seedUser(t, pool, "srs-unique-test@example.com")

	t.Run("one card per learner per content version", func(t *testing.T) {
		versionID := "b0000000-0000-0000-0000-000000000001"
		const insert = `
			INSERT INTO learn.review_cards (user_id, content_version_id, skill, due_at)
			VALUES ($1, $2, 'vocabulary', now())`

		if _, err := pool.Exec(ctx, insert, userID, versionID); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		_, err := pool.Exec(ctx, insert, userID, versionID)
		assertConstraintViolation(t, err, "uq_review_cards_user_content")
	})

	t.Run("one stats row per learner per day", func(t *testing.T) {
		const insert = `
			INSERT INTO learn.review_daily_stats (user_id, stat_date, reviews_completed)
			VALUES ($1, DATE '2026-01-15', 3)`

		if _, err := pool.Exec(ctx, insert, userID); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		_, err := pool.Exec(ctx, insert, userID)
		assertConstraintViolation(t, err, "uq_review_daily_stats_user_date")
	})

	t.Run("one parameter set per learner", func(t *testing.T) {
		const insert = `INSERT INTO learn.srs_params (user_id, weights) VALUES ($1, '{}'::jsonb)`

		if _, err := pool.Exec(ctx, insert, userID); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		_, err := pool.Exec(ctx, insert, userID)
		assertConstraintViolation(t, err, "uq_srs_params_user")
	})
}

// TestSRSSchema_DueQueueUsesThePartialIndex is what makes `GET /reviews/due-count`
// cheap enough to call on every app open.
//
// idx_review_cards_user_due is partial on `suspended_at IS NULL`, so the planner
// can only use it when the query carries that predicate. Drop `AND suspended_at
// IS NULL` from the due queries and this test goes red — before the sequential
// scan reaches production and the hottest query in the product starts reading
// every card in the table.
func TestSRSSchema_DueQueueUsesThePartialIndex(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	userID := seedUser(t, pool, "srs-explain-test@example.com")
	for i := range 50 {
		seedCard(t, pool, userID, time.Now().UTC().AddDate(0, 0, i))
	}
	if _, err := pool.Exec(ctx, `ANALYZE learn.review_cards`); err != nil {
		t.Fatalf("analyze review_cards: %v", err)
	}

	// The planner picks a sequential scan on a tiny table no matter what the
	// indexes say, so ask it to prefer the index when both are viable. The
	// question this test answers is whether the index is *usable*, which is a
	// property of the predicate, not of the row count.
	if _, err := pool.Exec(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `SET enable_seqscan = on`) })

	const explainQuery = `
		EXPLAIN (FORMAT TEXT)
		SELECT count(*) FROM learn.review_cards
		WHERE user_id = $1 AND suspended_at IS NULL AND due_at <= $2`

	rows, err := pool.Query(ctx, explainQuery, userID, time.Now().UTC().AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("explain due-count query: %v", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain output: %v", err)
	}

	planText := strings.Join(planLines, "\n")
	t.Logf("EXPLAIN plan:\n%s", planText)

	if !strings.Contains(planText, "idx_review_cards_user_due") {
		t.Errorf("expected the due-count plan to use idx_review_cards_user_due, got:\n%s", planText)
	}
}

// TestSRSMigration_DownRemovesEverythingItCreated tests migration reversibility.
func TestSRSMigration_DownRemovesEverythingItCreated(t *testing.T) {
	pool, provider := privateDatabase(t, downDatabase)
	ctx := context.Background()

	assertSRSObjectsExist(t, pool, true)

	if _, err := provider.DownTo(ctx, srsSchemaVersion-1); err != nil {
		t.Fatalf("roll back to %d: %v", srsSchemaVersion-1, err)
	}
	assertSRSObjectsExist(t, pool, false)

	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version < bootstrapVersion {
		t.Fatalf("rolled back to version %d, past the bootstrap migration", version)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("re-apply after rollback: %v", err)
	}
	assertSRSObjectsExist(t, pool, true)
}

func assertSRSObjectsExist(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	ctx := context.Background()

	tables := []string{"review_cards", "review_logs", "srs_params", "review_daily_stats"}
	for _, table := range tables {
		var exists bool
		const query = `SELECT to_regclass('learn.' || $1) IS NOT NULL`
		if err := pool.QueryRow(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("check table learn.%s: %v", table, err)
		}
		if exists != want {
			t.Errorf("learn.%s exists = %v, want %v", table, exists, want)
		}
	}

	var funcExists bool
	const funcQuery = `
		SELECT EXISTS (
			SELECT 1 FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = 'learn' AND p.proname = 'ensure_srs_partitions'
		)`
	if err := pool.QueryRow(ctx, funcQuery).Scan(&funcExists); err != nil {
		t.Fatalf("check function learn.ensure_srs_partitions: %v", err)
	}
	if funcExists != want {
		t.Errorf("learn.ensure_srs_partitions function exists = %v, want %v", funcExists, want)
	}

	// The down migration must take the partitions with it; a leftover
	// review_logs_yYYYYmMM would collide with the next apply.
	var partitionCount int
	const partitionQuery = `
		SELECT count(*) FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'learn' AND c.relname ~ '^review_logs_y\d{4}m\d{2}$'`
	if err := pool.QueryRow(ctx, partitionQuery).Scan(&partitionCount); err != nil {
		t.Fatalf("count review_logs partitions: %v", err)
	}
	if want && partitionCount == 0 {
		t.Error("no review_logs partitions exist after migrating up")
	}
	if !want && partitionCount != 0 {
		t.Errorf("%d review_logs partitions survived the down migration", partitionCount)
	}
}

// assertConstraintViolation fails unless err is the named PostgreSQL constraint
// firing. Matching on the name and not merely on "an error happened" is what
// stops the test passing for the wrong reason — a typo in the SQL, say.
func assertConstraintViolation(t *testing.T, err error, constraint string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected constraint %s to reject the row, but the insert succeeded", constraint)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a PostgreSQL error for %s, got %T: %v", constraint, err, err)
	}
	if pgErr.ConstraintName != constraint {
		t.Errorf("row was rejected by %q, want %q (%s)", pgErr.ConstraintName, constraint, pgErr.Message)
	}
}
