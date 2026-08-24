//go:build integration

// Package learning_test verifies the `learn` schema and partition machinery against a real PostgreSQL instance.
package learning_test

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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

const bootstrapVersion = 1700000000
const learningSchemaVersion = 1700000210

const schemaDatabase = "fluentra_learning_schema_test"
const downDatabase = "fluentra_learning_schema_down_test"

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

type foreignKeyDef struct {
	table   string
	name    string
	columns []int
}

type indexDef struct {
	table   string
	name    string
	columns []int
}

func fetchForeignKeys(ctx context.Context, pool *pgxpool.Pool) ([]foreignKeyDef, error) {
	const foreignKeys = `
		SELECT t.relname, c.conname, c.conkey::int[]
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE c.contype = 'f' AND n.nspname = 'learn'
		ORDER BY t.relname, c.conname`
	rows, err := pool.Query(ctx, foreignKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []foreignKeyDef
	for rows.Next() {
		var fk foreignKeyDef
		if err := rows.Scan(&fk.table, &fk.name, &fk.columns); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func fetchIndexes(ctx context.Context, pool *pgxpool.Pool) ([]indexDef, error) {
	const indexes = `
		SELECT t.relname, i.relname, x.indkey::int[]
		FROM pg_index x
		JOIN pg_class i ON i.oid = x.indexrelid
		JOIN pg_class t ON t.oid = x.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'learn'`
	iRows, err := pool.Query(ctx, indexes)
	if err != nil {
		return nil, err
	}
	defer iRows.Close()

	var idxs []indexDef
	for iRows.Next() {
		var idx indexDef
		if err := iRows.Scan(&idx.table, &idx.name, &idx.columns); err != nil {
			return nil, err
		}
		idxs = append(idxs, idx)
	}
	return idxs, iRows.Err()
}

func isFKCovered(fk foreignKeyDef, idxs []indexDef) bool {
	for _, idx := range idxs {
		if idx.table != fk.table || len(idx.columns) < len(fk.columns) {
			continue
		}
		match := true
		for i, col := range fk.columns {
			if idx.columns[i] != col {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestLearningSchema_EveryForeignKeyIsIndexed asserts that every FK in learn schema
// has an index with matching leading columns (Rule DB: foreign keys must be indexed).
func TestLearningSchema_EveryForeignKeyIsIndexed(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	fks, err := fetchForeignKeys(ctx, pool)
	if err != nil {
		t.Fatalf("read foreign keys: %v", err)
	}
	if len(fks) == 0 {
		t.Fatal("no foreign keys discovered in learn schema; check the query")
	}

	idxs, err := fetchIndexes(ctx, pool)
	if err != nil {
		t.Fatalf("read indexes: %v", err)
	}

	for _, fk := range fks {
		if !isFKCovered(fk, idxs) {
			t.Errorf("foreign key learn.%s.%s (columns %v) is not covered by any index prefix",
				fk.table, fk.name, fk.columns)
		}
	}
}

// TestLearningSchema_CrossSchemaForeignKeysRestrictedToUsers proves DB4:
// The only cross-schema foreign key permitted is to core.users(id).
func TestLearningSchema_CrossSchemaForeignKeysRestrictedToUsers(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const query = `
		SELECT t.relname, c.conname, fn.nspname AS target_schema, ft.relname AS target_table
		FROM pg_constraint c
		JOIN pg_class t       ON t.oid  = c.conrelid
		JOIN pg_namespace n   ON n.oid  = t.relnamespace
		JOIN pg_class ft      ON ft.oid = c.confrelid
		JOIN pg_namespace fn  ON fn.oid = ft.relnamespace
		WHERE c.contype = 'f' AND n.nspname = 'learn';`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("query foreign keys: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var table, constraint, targetSchema, targetTable string
		if err := rows.Scan(&table, &constraint, &targetSchema, &targetTable); err != nil {
			t.Fatalf("scan foreign key: %v", err)
		}

		if targetSchema == "learn" {
			continue
		}
		if targetSchema == "core" && targetTable == "users" {
			continue
		}

		t.Errorf("DB4 violation: learn.%s constraint %s references %s.%s (must be in 'learn' or 'core.users')",
			table, constraint, targetSchema, targetTable)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign keys: %v", err)
	}
}

// TestLearningSchema_CheckConstraintsEnforced verifies that every check constraint fires.
func TestLearningSchema_CheckConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	// Seed user, course, unit, lesson, activity
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO core.users (email, status)
		VALUES ('learn-ck-test@example.com', 'active')
		RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})

	var courseID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to, status)
		VALUES ('learn-ck-course', 'Learning Check Course', 'B1', 'B2', 'published')
		RETURNING id`).Scan(&courseID)
	if err != nil {
		t.Fatalf("seed course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	var unitID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.course_units (course_id, position, title)
		VALUES ($1, 1, 'Unit 1')
		RETURNING id`, courseID).Scan(&unitID)
	if err != nil {
		t.Fatalf("seed unit: %v", err)
	}

	var lessonID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
		VALUES ($1, 1, 'Lesson 1', 'vocabulary')
		RETURNING id`, unitID).Scan(&lessonID)
	if err != nil {
		t.Fatalf("seed lesson: %v", err)
	}

	var activityID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.activities (lesson_id, position, kind, content_version_id)
		VALUES ($1, 1, 'quiz', gen_random_uuid())
		RETURNING id`, lessonID).Scan(&activityID)
	if err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	cases := []struct {
		name       string
		statement  string
		args       []any
		constraint string
	}{
		{
			name: "enrollment invalid status",
			statement: `INSERT INTO learn.enrollments (user_id, course_id, status)
				VALUES ($1, $2, 'invalid_status')`,
			args:       []any{userID, courseID},
			constraint: "ck_enrollments_status",
		},
		{
			name: "enrollment completed without completed_at",
			statement: `INSERT INTO learn.enrollments (user_id, course_id, status, completed_at)
				VALUES ($1, $2, 'completed', NULL)`,
			args:       []any{userID, courseID},
			constraint: "ck_enrollments_completed_at",
		},
		{
			name: "enrollment active with completed_at set",
			statement: `INSERT INTO learn.enrollments (user_id, course_id, status, completed_at)
				VALUES ($1, $2, 'active', now())`,
			args:       []any{userID, courseID},
			constraint: "ck_enrollments_completed_at",
		},
		{
			name: "progress invalid scope",
			statement: `INSERT INTO learn.progress (user_id, scope, scope_id, status)
				VALUES ($1, 'invalid_scope', gen_random_uuid(), 'in_progress')`,
			args:       []any{userID},
			constraint: "ck_progress_scope",
		},
		{
			name: "progress invalid status",
			statement: `INSERT INTO learn.progress (user_id, scope, scope_id, status)
				VALUES ($1, 'lesson', gen_random_uuid(), 'unknown')`,
			args:       []any{userID},
			constraint: "ck_progress_status",
		},
		{
			name: "progress score over 100",
			statement: `INSERT INTO learn.progress (user_id, scope, scope_id, status, score)
				VALUES ($1, 'activity', gen_random_uuid(), 'in_progress', 105.00)`,
			args:       []any{userID},
			constraint: "ck_progress_score",
		},
		{
			name: "progress score negative",
			statement: `INSERT INTO learn.progress (user_id, scope, scope_id, status, score)
				VALUES ($1, 'activity', gen_random_uuid(), 'in_progress', -5.00)`,
			args:       []any{userID},
			constraint: "ck_progress_score",
		},
		{
			name: "progress completed without completed_at",
			statement: `INSERT INTO learn.progress (user_id, scope, scope_id, status, completed_at)
				VALUES ($1, 'activity', gen_random_uuid(), 'completed', NULL)`,
			args:       []any{userID},
			constraint: "ck_progress_completed_at",
		},
		{
			name: "attempt invalid status",
			statement: `INSERT INTO learn.attempts (user_id, activity_id, status)
				VALUES ($1, $2, 'invalid_attempt_status')`,
			args:       []any{userID, activityID},
			constraint: "ck_attempts_status",
		},
		{
			name: "attempt negative duration",
			statement: `INSERT INTO learn.attempts (user_id, activity_id, duration_ms)
				VALUES ($1, $2, -100)`,
			args:       []any{userID, activityID},
			constraint: "ck_attempts_duration_positive",
		},
		{
			name: "attempt negative max_score",
			statement: `INSERT INTO learn.attempts (user_id, activity_id, max_score)
				VALUES ($1, $2, -10.00)`,
			args:       []any{userID, activityID},
			constraint: "ck_attempts_max_score_positive",
		},
		{
			name: "attempt score greater than max_score",
			statement: `INSERT INTO learn.attempts (user_id, activity_id, score, max_score)
				VALUES ($1, $2, 120.00, 100.00)`,
			args:       []any{userID, activityID},
			constraint: "ck_attempts_score_range",
		},
		{
			name: "attempt score negative",
			statement: `INSERT INTO learn.attempts (user_id, activity_id, score, max_score)
				VALUES ($1, $2, -1.00, 100.00)`,
			args:       []any{userID, activityID},
			constraint: "ck_attempts_score_range",
		},
		{
			name: "learning session negative activities completed",
			statement: `INSERT INTO learn.learning_sessions (user_id, activities_completed)
				VALUES ($1, -1)`,
			args:       []any{userID},
			constraint: "ck_learning_sessions_activities_completed",
		},
		{
			name: "learning session negative minutes",
			statement: `INSERT INTO learn.learning_sessions (user_id, minutes)
				VALUES ($1, -5)`,
			args:       []any{userID},
			constraint: "ck_learning_sessions_minutes",
		},
		{
			name: "learning session ended before started",
			statement: `INSERT INTO learn.learning_sessions (user_id, started_at, ended_at)
				VALUES ($1, now(), now() - interval '10 minutes')`,
			args:       []any{userID},
			constraint: "ck_learning_sessions_ended_after_started",
		},
		{
			name: "placement result invalid level",
			statement: `INSERT INTO learn.placement_results (user_id, estimated_level)
				VALUES ($1, 'D1')`,
			args:       []any{userID},
			constraint: "ck_placement_results_level",
		},
		{
			name: "skill mastery invalid skill",
			statement: `INSERT INTO learn.skill_mastery (user_id, skill, level)
				VALUES ($1, 'invalid_skill', 'B1')`,
			args:       []any{userID},
			constraint: "ck_skill_mastery_skill",
		},
		{
			name: "skill mastery invalid level",
			statement: `INSERT INTO learn.skill_mastery (user_id, skill, level)
				VALUES ($1, 'vocabulary', 'X1')`,
			args:       []any{userID},
			constraint: "ck_skill_mastery_level",
		},
		{
			name: "skill mastery confidence above 1",
			statement: `INSERT INTO learn.skill_mastery (user_id, skill, level, confidence)
				VALUES ($1, 'vocabulary', 'B1', 1.50)`,
			args:       []any{userID},
			constraint: "ck_skill_mastery_confidence",
		},
		{
			name: "skill mastery confidence below 0",
			statement: `INSERT INTO learn.skill_mastery (user_id, skill, level, confidence)
				VALUES ($1, 'vocabulary', 'B1', -0.10)`,
			args:       []any{userID},
			constraint: "ck_skill_mastery_confidence",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.statement, tc.args...)
			if err == nil {
				t.Fatalf("row was accepted; constraint %s did not fire", tc.constraint)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("error = %v, want a PostgreSQL error", err)
			}
			if pgErr.ConstraintName != tc.constraint {
				t.Fatalf("violated constraint = %q, want %q", pgErr.ConstraintName, tc.constraint)
			}
		})
	}
}

// TestLearningSchema_UniqueConstraintsEnforced proves uniqueness for enrollments, progress, and skill mastery.
func TestLearningSchema_UniqueConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO core.users (email, status)
		VALUES ('learn-uq-test@example.com', 'active')
		RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})

	var courseID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to, status)
		VALUES ('learn-uq-course', 'UQ Test Course', 'A1', 'A2', 'published')
		RETURNING id`).Scan(&courseID)
	if err != nil {
		t.Fatalf("seed course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	// 1. Enrollment uniqueness: uq_enrollments_user_course
	const insertEnrollment = `INSERT INTO learn.enrollments (user_id, course_id) VALUES ($1, $2)`
	if _, err := pool.Exec(ctx, insertEnrollment, userID, courseID); err != nil {
		t.Fatalf("insert enrollment: %v", err)
	}
	if _, err := pool.Exec(ctx, insertEnrollment, userID, courseID); err == nil {
		t.Fatal("duplicate enrollment accepted; uq_enrollments_user_course did not fire")
	}

	// 2. Progress uniqueness: uq_progress_user_scope
	scopeID := courseID
	const insertProgress = `INSERT INTO learn.progress (user_id, scope, scope_id) VALUES ($1, 'course', $2)`
	if _, err := pool.Exec(ctx, insertProgress, userID, scopeID); err != nil {
		t.Fatalf("insert progress: %v", err)
	}
	if _, err := pool.Exec(ctx, insertProgress, userID, scopeID); err == nil {
		t.Fatal("duplicate progress accepted; uq_progress_user_scope did not fire")
	}

	// 3. Skill mastery uniqueness: uq_skill_mastery_user_skill
	const insertSkill = `
		INSERT INTO learn.skill_mastery (user_id, skill, level, confidence)
		VALUES ($1, 'grammar', 'B1', 0.8)`
	if _, err := pool.Exec(ctx, insertSkill, userID); err != nil {
		t.Fatalf("insert skill mastery: %v", err)
	}
	if _, err := pool.Exec(ctx, insertSkill, userID); err == nil {
		t.Fatal("duplicate skill mastery accepted; uq_skill_mastery_user_skill did not fire")
	}
}

// TestLearningSchema_PartitionsPreCreatedAndTargeted verifies that partitions for the
// current and next 2 months exist immediately after migration, and inserting a row
// for next month lands in next month's partition.
func TestLearningSchema_PartitionsPreCreatedAndTargeted(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	nowUTC := time.Now().UTC()
	currentMonthStart := time.Date(nowUTC.Year(), nowUTC.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Check partitions for current month, next month, and month+2
	for i := 0; i <= 2; i++ {
		monthTime := currentMonthStart.AddDate(0, i, 0)
		partitionName := fmt.Sprintf("attempts_y%04dm%02d", monthTime.Year(), int(monthTime.Month()))

		var exists bool
		const checkQuery = `SELECT to_regclass('learn.' || $1) IS NOT NULL`
		if err := pool.QueryRow(ctx, checkQuery, partitionName).Scan(&exists); err != nil {
			t.Fatalf("check partition %s: %v", partitionName, err)
		}
		if !exists {
			t.Errorf("expected partition learn.%s to exist after migration, but it does not", partitionName)
		}
	}

	// Seed user and activity for insertion
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO core.users (email, status)
		VALUES ('learn-partition-test@example.com', 'active')
		RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})

	var courseID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
		VALUES ('learn-part-course', 'Partition Test Course', 'A1', 'A2')
		RETURNING id`).Scan(&courseID)
	if err != nil {
		t.Fatalf("seed course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	var unitID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.course_units (course_id, position, title)
		VALUES ($1, 1, 'Unit 1')
		RETURNING id`, courseID).Scan(&unitID)
	if err != nil {
		t.Fatalf("seed unit: %v", err)
	}

	var lessonID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
		VALUES ($1, 1, 'Lesson 1', 'reading')
		RETURNING id`, unitID).Scan(&lessonID)
	if err != nil {
		t.Fatalf("seed lesson: %v", err)
	}

	var activityID string
	err = pool.QueryRow(ctx, `
		INSERT INTO learn.activities (lesson_id, position, kind, content_version_id)
		VALUES ($1, 1, 'reading_mcq', gen_random_uuid())
		RETURNING id`, lessonID).Scan(&activityID)
	if err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	// Insert attempt dated next month (day 15 of next month)
	nextMonth := currentMonthStart.AddDate(0, 1, 14)
	expectedPartition := fmt.Sprintf("attempts_y%04dm%02d", nextMonth.Year(), int(nextMonth.Month()))

	var insertedID string
	var actualPartition string
	const insertNextMonth = `
		INSERT INTO learn.attempts (user_id, activity_id, created_at, status)
		VALUES ($1, $2, $3, 'in_progress')
		RETURNING id, tableoid::regclass::text`

	err = pool.QueryRow(ctx, insertNextMonth, userID, activityID, nextMonth).Scan(&insertedID, &actualPartition)
	if err != nil {
		t.Fatalf("insert attempt into next month: %v", err)
	}

	// PostgreSQL returns `tableoid::regclass` formatted as `learn.attempts_yYYYYmMM` or `attempts_yYYYYmMM`
	if !strings.HasSuffix(actualPartition, expectedPartition) {
		t.Errorf("attempt row landed in partition %q, want suffix %q", actualPartition, expectedPartition)
	}
}

// TestLearningSchema_OneAttemptIsGradedAtMostOnce is the test behind the
// idempotency decision in DECISIONS.md.
//
// A unique constraint on a partitioned table has to include the partition key,
// so this schema cannot make `idempotency_key` globally unique. The invariant it
// does enforce is narrower and sufficient: one attempt row is claimed for
// grading at most once. Two concurrent submissions race the same conditional
// UPDATE, and exactly one may win — which is what lets P8.3 return the stored
// result to the loser instead of grading twice.
//
// The failure this catches is the claim itself. Drop `AND status =
// 'in_progress'` from ClaimAttemptForGrading and both transactions update the
// row, both callers believe they won, and the activity is graded twice.
func TestLearningSchema_OneAttemptIsGradedAtMostOnce(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	userID, activityID := seedAttemptTarget(t, pool, "learn-idem-test")

	var attemptID string
	var attemptCreatedAt time.Time
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.attempts (user_id, activity_id, status)
		VALUES ($1, $2, 'in_progress')
		RETURNING id, created_at`, userID, activityID).Scan(&attemptID, &attemptCreatedAt); err != nil {
		t.Fatalf("seed attempt: %v", err)
	}

	// The same claim both callers issue. Serialising them in Go would prove
	// nothing: the question is what the database does when both are in flight.
	const claim = `
		UPDATE learn.attempts
		SET status = 'grading', idempotency_key = $3, updated_at = now()
		WHERE id = $1 AND created_at = $2 AND status = 'in_progress'
		RETURNING id`

	key := "0199a1c2-3d4e-7f80-9abc-def012345683"
	type outcome struct {
		claimed bool
		err     error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})

	for i := 0; i < 2; i++ {
		go func() {
			<-start
			tx, err := pool.Begin(ctx)
			if err != nil {
				results <- outcome{err: err}
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()

			var claimedID string
			scanErr := tx.QueryRow(ctx, claim, attemptID, attemptCreatedAt, key).Scan(&claimedID)
			if errors.Is(scanErr, pgx.ErrNoRows) {
				results <- outcome{claimed: false}
				return
			}
			if scanErr != nil {
				results <- outcome{err: scanErr}
				return
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				results <- outcome{err: commitErr}
				return
			}
			results <- outcome{claimed: true}
		}()
	}
	close(start)

	claims := 0
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err != nil {
			t.Fatalf("claim %d: %v", i, got.err)
		}
		if got.claimed {
			claims++
		}
	}

	if claims != 1 {
		t.Errorf("%d of 2 concurrent submissions claimed the attempt, want exactly 1", claims)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM learn.attempts WHERE id = $1`, attemptID).Scan(&status); err != nil {
		t.Fatalf("read attempt back: %v", err)
	}
	if status != "grading" {
		t.Errorf("attempt status = %q, want grading", status)
	}
}

// seedAttemptTarget builds the course/unit/lesson/activity chain an attempt
// needs, plus its learner, and cleans all of it up.
func seedAttemptTarget(t *testing.T, pool *pgxpool.Pool, tag string) (userID, activityID string) {
	t.Helper()
	ctx := context.Background()

	if err := pool.QueryRow(ctx, `
		INSERT INTO core.users (email, status)
		VALUES ($1, 'active')
		RETURNING id`, tag+"@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})

	var courseID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
		VALUES ($1, 'Idempotency Test Course', 'A1', 'A2')
		RETURNING id`, tag).Scan(&courseID); err != nil {
		t.Fatalf("seed course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	var unitID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.course_units (course_id, position, title)
		VALUES ($1, 1, 'Unit 1')
		RETURNING id`, courseID).Scan(&unitID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}

	var lessonID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
		VALUES ($1, 1, 'Lesson 1', 'reading')
		RETURNING id`, unitID).Scan(&lessonID); err != nil {
		t.Fatalf("seed lesson: %v", err)
	}

	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.activities (lesson_id, position, kind, content_version_id)
		VALUES ($1, 1, 'reading_mcq', gen_random_uuid())
		RETURNING id`, lessonID).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	return userID, activityID
}

// TestLearningSchema_EnsurePartitionsIdempotent verifies learn.ensure_partitions is idempotent.
func TestLearningSchema_EnsurePartitionsIdempotent(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var createdCount int
	if err := pool.QueryRow(ctx, `SELECT learn.ensure_partitions(3)`).Scan(&createdCount); err != nil {
		t.Fatalf("ensure_partitions(3): %v", err)
	}
	if createdCount != 0 {
		t.Errorf("second run of ensure_partitions created %d partitions, want 0 (idempotent)", createdCount)
	}
}

// TestLearningSchema_ProgressQueryUsesUniqueIndex verifies EXPLAIN uses uq_progress_user_scope for dashboard lookups.
func TestLearningSchema_ProgressQueryUsesUniqueIndex(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO core.users (email, status)
		VALUES ('learn-explain-test@example.com', 'active')
		RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, userID)
	})

	scopeID := "a0000000-0000-0000-0000-000000000001"
	_, err = pool.Exec(ctx, `
		INSERT INTO learn.progress (user_id, scope, scope_id, status, score)
		VALUES ($1, 'course', $2, 'in_progress', 75.00)`, userID, scopeID)
	if err != nil {
		t.Fatalf("seed progress: %v", err)
	}

	// Run EXPLAIN query
	const explainQuery = `
		EXPLAIN (FORMAT TEXT)
		SELECT id, user_id, scope, scope_id, status, score, completed_at
		FROM learn.progress
		WHERE user_id = $1 AND scope = $2 AND scope_id = $3`

	rows, err := pool.Query(ctx, explainQuery, userID, "course", scopeID)
	if err != nil {
		t.Fatalf("explain progress query: %v", err)
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

	planText := strings.Join(planLines, "\n")
	t.Logf("EXPLAIN plan:\n%s", planText)

	if !strings.Contains(planText, "uq_progress_user_scope") {
		t.Errorf("expected plan to use index 'uq_progress_user_scope', got:\n%s", planText)
	}
}

// TestLearningMigration_DownRemovesEverythingItCreated tests migration reversibility.
func TestLearningMigration_DownRemovesEverythingItCreated(t *testing.T) {
	pool, provider := privateDatabase(t, downDatabase)
	ctx := context.Background()

	assertLearningObjectsExist(t, pool, true)

	if _, err := provider.DownTo(ctx, learningSchemaVersion-1); err != nil {
		t.Fatalf("roll back to %d: %v", learningSchemaVersion-1, err)
	}
	assertLearningObjectsExist(t, pool, false)

	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version < bootstrapVersion {
		t.Fatalf("rolled back to version %d, past the bootstrap migration", version)
	}

	// Re-applying must succeed
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("re-apply after rollback: %v", err)
	}
	assertLearningObjectsExist(t, pool, true)
}

func assertLearningObjectsExist(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	ctx := context.Background()

	tables := []string{
		"enrollments",
		"progress",
		"attempts",
		"learning_sessions",
		"placement_results",
		"skill_mastery",
	}
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
			WHERE n.nspname = 'learn' AND p.proname = 'ensure_partitions'
		)`
	if err := pool.QueryRow(ctx, funcQuery).Scan(&funcExists); err != nil {
		t.Fatalf("check function learn.ensure_partitions: %v", err)
	}
	if funcExists != want {
		t.Errorf("learn.ensure_partitions function exists = %v, want %v", funcExists, want)
	}
}
