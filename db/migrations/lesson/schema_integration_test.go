//go:build integration

// Package lesson_test verifies the `learn` schema against a real PostgreSQL instance.
package lesson_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

const bootstrapVersion = 1700000000
const lessonSchemaVersion = 1700000200

const schemaDatabase = "fluentra_lesson_schema_test"
const downDatabase = "fluentra_lesson_schema_down_test"

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

// TestLessonSchema_EveryForeignKeyIsIndexed asserts that every FK in learn schema
// has an index with matching leading columns (Rule DB: foreign keys must be indexed).
func TestLessonSchema_EveryForeignKeyIsIndexed(t *testing.T) {
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

// TestLessonSchema_CrossSchemaForeignKeysRestrictedToUsers proves DB4:
// The only cross-schema foreign key permitted is to core.users(id).
func TestLessonSchema_CrossSchemaForeignKeysRestrictedToUsers(t *testing.T) {
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

// TestLessonSchema_CheckConstraintsEnforced verifies that every check constraint fires.
func TestLessonSchema_CheckConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	// Seed course, unit, lesson
	var courseID string
	err := pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to, status)
		VALUES ('ck-test-course', 'Check Test Course', 'B1', 'B2', 'published')
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

	cases := []struct {
		name       string
		statement  string
		args       []any
		constraint string
	}{
		{
			name: "course slug uppercase",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
				VALUES ('INVALID_SLUG', 'Title', 'B1', 'B2')`,
			constraint: "ck_courses_slug_format",
		},
		{
			name: "course slug spaces",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
				VALUES ('invalid slug', 'Title', 'B1', 'B2')`,
			constraint: "ck_courses_slug_format",
		},
		{
			name: "course empty title",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
				VALUES ('empty-title', '', 'B1', 'B2')`,
			constraint: "ck_courses_title_length",
		},
		{
			name: "course invalid cefr_from",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
				VALUES ('cefr-from-test', 'Title', 'X1', 'B2')`,
			constraint: "ck_courses_cefr_from",
		},
		{
			name: "course lowercase cefr_to",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
				VALUES ('cefr-to-test', 'Title', 'B1', 'b2')`,
			constraint: "ck_courses_cefr_to",
		},
		{
			name: "course invalid status",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to, status)
				VALUES ('status-test', 'Title', 'B1', 'B2', 'unknown')`,
			constraint: "ck_courses_status",
		},
		{
			name: "course negative estimated_hours",
			statement: `INSERT INTO learn.courses (slug, title, cefr_from, cefr_to, estimated_hours)
				VALUES ('hours-test', 'Title', 'B1', 'B2', -5)`,
			constraint: "ck_courses_estimated_hours",
		},
		{
			name: "unit non-positive position",
			statement: `INSERT INTO learn.course_units (course_id, position, title)
				VALUES ($1, 0, 'Unit zero')`,
			args:       []any{courseID},
			constraint: "ck_course_units_position_positive",
		},
		{
			name: "unit empty title",
			statement: `INSERT INTO learn.course_units (course_id, position, title)
				VALUES ($1, 2, '')`,
			args:       []any{courseID},
			constraint: "ck_course_units_title_length",
		},
		{
			name: "lesson non-positive position",
			statement: `INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
				VALUES ($1, 0, 'Lesson zero', 'grammar')`,
			args:       []any{unitID},
			constraint: "ck_lessons_position_positive",
		},
		{
			name: "lesson empty title",
			statement: `INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
				VALUES ($1, 2, '', 'grammar')`,
			args:       []any{unitID},
			constraint: "ck_lessons_title_length",
		},
		{
			name: "lesson negative estimated_minutes",
			statement: `INSERT INTO learn.lessons (unit_id, position, title, skill_focus, estimated_minutes)
				VALUES ($1, 2, 'Lesson neg', 'grammar', -10)`,
			args:       []any{unitID},
			constraint: "ck_lessons_estimated_minutes",
		},
		{
			name: "lesson invalid status",
			statement: `INSERT INTO learn.lessons (unit_id, position, title, skill_focus, status)
				VALUES ($1, 2, 'Lesson stat', 'grammar', 'unknown')`,
			args:       []any{unitID},
			constraint: "ck_lessons_status",
		},
		{
			name: "activity non-positive position",
			statement: `INSERT INTO learn.activities (lesson_id, position, kind, content_version_id)
				VALUES ($1, 0, 'quiz', gen_random_uuid())`,
			args:       []any{lessonID},
			constraint: "ck_activities_position_positive",
		},
		{
			name: "activity negative weight",
			statement: `INSERT INTO learn.activities (lesson_id, position, kind, content_version_id, weight)
				VALUES ($1, 2, 'quiz', gen_random_uuid(), -1)`,
			args:       []any{lessonID},
			constraint: "ck_activities_weight",
		},
		{
			name: "lesson prerequisite self-reference",
			statement: `INSERT INTO learn.lesson_prerequisites (lesson_id, requires_lesson_id)
				VALUES ($1, $1)`,
			args:       []any{lessonID},
			constraint: "ck_lesson_prerequisites_no_self_ref",
		},
		{
			name: "lesson prerequisite invalid min_score",
			statement: `INSERT INTO learn.lesson_prerequisites (lesson_id, requires_lesson_id, min_score)
				VALUES ($1, gen_random_uuid(), 150)`,
			args:       []any{lessonID},
			constraint: "ck_lesson_prerequisites_min_score",
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

// TestCourses_SlugIsUnique proves slug uniqueness constraint uq_courses_slug.
func TestCourses_SlugIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const insert = `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
		VALUES ('uniq-course-slug', 'Course One', 'B1', 'B2')
		RETURNING id`
	var id string
	if err := pool.QueryRow(ctx, insert).Scan(&id); err != nil {
		t.Fatalf("insert course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, id)
	})

	_, err := pool.Exec(ctx, insert)
	if err == nil {
		t.Fatal("duplicate slug accepted; uq_courses_slug did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_courses_slug" {
		t.Errorf("violated constraint = %q, want uq_courses_slug", pgErr.ConstraintName)
	}
}

// TestCourseUnits_PositionIsUnique proves uq_course_units_course_position.
func TestCourseUnits_PositionIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var courseID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
		VALUES ('unit-pos-test', 'Unit Pos Course', 'A1', 'A2')
		RETURNING id`).Scan(&courseID); err != nil {
		t.Fatalf("insert course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	const insertUnit = `INSERT INTO learn.course_units (course_id, position, title) VALUES ($1, 1, 'Unit 1')`
	if _, err := pool.Exec(ctx, insertUnit, courseID); err != nil {
		t.Fatalf("insert unit: %v", err)
	}

	_, err := pool.Exec(ctx, insertUnit, courseID)
	if err == nil {
		t.Fatal("duplicate unit position accepted; uq_course_units_course_position did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_course_units_course_position" {
		t.Errorf("violated constraint = %q, want uq_course_units_course_position", pgErr.ConstraintName)
	}
}

// TestLessons_PositionIsUnique proves uq_lessons_unit_position.
func TestLessons_PositionIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var courseID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
		VALUES ('lesson-pos-test', 'Lesson Pos Course', 'A1', 'A2')
		RETURNING id`).Scan(&courseID); err != nil {
		t.Fatalf("insert course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	var unitID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.course_units (course_id, position, title)
		VALUES ($1, 1, 'Unit 1')
		RETURNING id`, courseID).Scan(&unitID); err != nil {
		t.Fatalf("insert unit: %v", err)
	}

	const insertLesson = `
		INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
		VALUES ($1, 1, 'Lesson 1', 'reading')`
	if _, err := pool.Exec(ctx, insertLesson, unitID); err != nil {
		t.Fatalf("insert lesson: %v", err)
	}

	_, err := pool.Exec(ctx, insertLesson, unitID)
	if err == nil {
		t.Fatal("duplicate lesson position accepted; uq_lessons_unit_position did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_lessons_unit_position" {
		t.Errorf("violated constraint = %q, want uq_lessons_unit_position", pgErr.ConstraintName)
	}
}

// TestActivities_PositionIsUnique proves uq_activities_lesson_position.
func TestActivities_PositionIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	var courseID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.courses (slug, title, cefr_from, cefr_to)
		VALUES ('act-pos-test', 'Act Pos Course', 'A1', 'A2')
		RETURNING id`).Scan(&courseID); err != nil {
		t.Fatalf("insert course: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learn.courses WHERE id = $1`, courseID)
	})

	var unitID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.course_units (course_id, position, title)
		VALUES ($1, 1, 'Unit 1')
		RETURNING id`, courseID).Scan(&unitID); err != nil {
		t.Fatalf("insert unit: %v", err)
	}

	var lessonID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO learn.lessons (unit_id, position, title, skill_focus)
		VALUES ($1, 1, 'Lesson 1', 'reading')
		RETURNING id`, unitID).Scan(&lessonID); err != nil {
		t.Fatalf("insert lesson: %v", err)
	}

	const insertAct = `
		INSERT INTO learn.activities (lesson_id, position, kind, content_version_id)
		VALUES ($1, 1, 'multiple_choice', gen_random_uuid())`
	if _, err := pool.Exec(ctx, insertAct, lessonID); err != nil {
		t.Fatalf("insert activity: %v", err)
	}

	_, err := pool.Exec(ctx, insertAct, lessonID)
	if err == nil {
		t.Fatal("duplicate activity position accepted; uq_activities_lesson_position did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_activities_lesson_position" {
		t.Errorf("violated constraint = %q, want uq_activities_lesson_position", pgErr.ConstraintName)
	}
}

// TestLessonMigration_DownRemovesEverythingItCreated tests migration reversibility.
func TestLessonMigration_DownRemovesEverythingItCreated(t *testing.T) {
	pool, provider := privateDatabase(t, downDatabase)
	ctx := context.Background()

	assertLessonObjectsExist(t, pool, true)

	if _, err := provider.DownTo(ctx, lessonSchemaVersion-1); err != nil {
		t.Fatalf("roll back to %d: %v", lessonSchemaVersion-1, err)
	}
	assertLessonObjectsExist(t, pool, false)

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
	assertLessonObjectsExist(t, pool, true)
}

func assertLessonObjectsExist(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	ctx := context.Background()

	tables := []string{
		"courses",
		"course_units",
		"lessons",
		"activities",
		"lesson_prerequisites",
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
}
