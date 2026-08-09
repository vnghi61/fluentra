//go:build integration

// Package user_test verifies the `core` schema against a real PostgreSQL, not
// against the text of the migration. A regex over SQL proves the file says
// something; only the server proves the constraint holds.
package user_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

// bootstrapVersion is the migration that creates the schemas and the two
// database roles. Those roles are cluster-wide, so no test may roll back past
// this point: doing so would drop roles that every other integration package
// running against the same server depends on.
const bootstrapVersion = 1700000000

// userSchemaVersion is the migration under test.
const userSchemaVersion = 1700000010

// schemaDatabase is created once for this package and dropped when it
// finishes. Nothing here runs against the database TEST_DATABASE_URL names.
//
// That is not tidiness. The shared database is also used by the outbox, job
// and worker suites, which truncate `ops` tables and then assert exact row
// counts. Adding a fifth package to the pool changed how those three
// interleave, and the suite started failing in two runs out of six — in
// packages this one never touches. An integration package that only reads its
// own schema has no reason to be in that pool at all.
const schemaDatabase = "fluentra_user_schema_test"

// downDatabase is separate again: that test rolls the schema back, so it must
// not share even with the read-only tests here.
const downDatabase = "fluentra_user_schema_down_test"

// packagePool is the pool over schemaDatabase, or nil when TEST_DATABASE_URL
// is unset and every test skips.
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

// migratedPool returns the pool over this package's own database.
func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if packagePool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return packagePool
}

// createDatabase makes a throwaway database and returns its DSN together with
// the function that removes it.
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

// privateDatabase gives one test a database of its own, migrated and dropped
// on cleanup, and hands back the provider so the test can drive migrations.
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

// migrateUp applies every embedded migration and returns the provider still
// attached to the database, so a caller can roll back through it.
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

// insertUser creates a user and removes it again, so the tests in this file
// stay independent of each other regardless of the order they run in.
func insertUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	const insert = `INSERT INTO core.users (email) VALUES ($1) RETURNING id`
	if err := pool.QueryRow(ctx, insert, email).Scan(&id); err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, id)
	})
	return id
}

// TestUsersTable_HoldsIdentityOnly is the trap from the P1.1 card made
// executable: every schema in the system foreign-keys to core.users, and that
// is only defensible while the table stays narrow. A profile column added here
// fails this test rather than being noticed in review, or not.
func TestUsersTable_HoldsIdentityOnly(t *testing.T) {
	pool := migratedPool(t)

	const columns = `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'core' AND table_name = 'users'
		ORDER BY column_name`
	rows, err := pool.Query(context.Background(), columns)
	if err != nil {
		t.Fatalf("read columns: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns: %v", err)
	}

	want := []string{"created_at", "email", "email_verified_at", "id", "status", "updated_at"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("core.users columns = %v, want exactly %v\n"+
			"Descriptive data belongs in core.profiles, settings in core.user_preferences.", got, want)
	}
}

// TestUsersEmail_IsUniqueCaseInsensitively is BR-USER-01. The card names the
// exact pair to try.
func TestUsersEmail_IsUniqueCaseInsensitively(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	insertUser(t, pool, "A@b.com")

	_, err := pool.Exec(ctx, `INSERT INTO core.users (email) VALUES ($1)`, "a@b.com")
	if err == nil {
		t.Fatal("a@b.com was accepted alongside A@b.com: email uniqueness is case-sensitive")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("insert error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_users_email" {
		t.Errorf("violated constraint = %q, want uq_users_email", pgErr.ConstraintName)
	}

	// The other half of citext: a lookup written in any case finds the row.
	var found string
	const byEmail = `SELECT email FROM core.users WHERE email = $1`
	if err := pool.QueryRow(ctx, byEmail, "a@B.COM").Scan(&found); err != nil {
		t.Fatalf("case-insensitive lookup failed: %v", err)
	}
	if found != "A@b.com" {
		t.Errorf("stored email = %q, want the original casing preserved", found)
	}
}

// TestCoreSchema_EveryForeignKeyIsIndexed is a Definition-of-Done item that is
// easy to satisfy by accident and easy to break by accident. PostgreSQL indexes
// the referenced side automatically and the referencing side never.
func TestCoreSchema_EveryForeignKeyIsIndexed(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const foreignKeys = `
		SELECT t.relname, c.conname, c.conkey::int[]
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE c.contype = 'f' AND n.nspname = 'core'
		ORDER BY t.relname, c.conname`
	rows, err := pool.Query(ctx, foreignKeys)
	if err != nil {
		t.Fatalf("read foreign keys: %v", err)
	}
	defer rows.Close()

	type foreignKey struct {
		table   string
		name    string
		columns []int32
	}
	var keys []foreignKey
	for rows.Next() {
		var key foreignKey
		if err := rows.Scan(&key.table, &key.name, &key.columns); err != nil {
			t.Fatalf("scan foreign key: %v", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign keys: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("no foreign keys found in schema core — the migration did not run")
	}

	const indexes = `
		SELECT t.relname, string_to_array(i.indkey::text, ' ')::int[]
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'core'`
	indexRows, err := pool.Query(ctx, indexes)
	if err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	defer indexRows.Close()

	byTable := map[string][][]int32{}
	for indexRows.Next() {
		var table string
		var keyColumns []int32
		if err := indexRows.Scan(&table, &keyColumns); err != nil {
			t.Fatalf("scan index: %v", err)
		}
		byTable[table] = append(byTable[table], keyColumns)
	}
	if err := indexRows.Err(); err != nil {
		t.Fatalf("iterate indexes: %v", err)
	}

	for _, key := range keys {
		if !hasLeadingIndex(byTable[key.table], key.columns) {
			t.Errorf("core.%s: foreign key %s has no index whose leading columns match it",
				key.table, key.name)
		}
	}
}

// hasLeadingIndex reports whether any index starts with exactly the foreign
// key's columns, in order. A trailing-position match does not help the planner
// on a cascade or a join, so only the prefix counts.
func hasLeadingIndex(indexes [][]int32, columns []int32) bool {
	for _, index := range indexes {
		if len(index) < len(columns) {
			continue
		}
		match := true
		for position, column := range columns {
			if index[position] != column {
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

// TestCoreSchema_CheckConstraintsRejectInvalidRows proves the invariants are in
// the database rather than only in the service that happens to write today.
func TestCoreSchema_CheckConstraintsRejectInvalidRows(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "constraints@fluentra.test")

	cases := []struct {
		name       string
		statement  string
		args       []any
		constraint string
	}{
		{
			name:       "display name over 50 characters",
			statement:  `INSERT INTO core.profiles (user_id, display_name) VALUES ($1, $2)`,
			args:       []any{userID, strings.Repeat("x", 51)},
			constraint: "ck_profiles_display_name_length",
		},
		{
			name:       "country is not ISO 3166-1 alpha-2",
			statement:  `INSERT INTO core.profiles (user_id, display_name, country) VALUES ($1, 'Nghi', $2)`,
			args:       []any{userID, "vnm"},
			constraint: "ck_profiles_country_format",
		},
		{
			name:       "daily goal outside 5..480 minutes",
			statement:  `INSERT INTO core.user_preferences (user_id, daily_goal_minutes) VALUES ($1, $2)`,
			args:       []any{userID, 1},
			constraint: "ck_user_preferences_daily_goal",
		},
		{
			name:       "unknown notification channel",
			statement:  `INSERT INTO core.user_preferences (user_id, notification_channels) VALUES ($1, $2)`,
			args:       []any{userID, []string{"carrier_pigeon"}},
			constraint: "ck_user_preferences_channels",
		},
		{
			name:       "half a quiet-hours window",
			statement:  `INSERT INTO core.user_preferences (user_id, quiet_hours_start) VALUES ($1, TIME '22:00')`,
			args:       []any{userID},
			constraint: "ck_user_preferences_quiet_hours",
		},
		{
			name:       "weekly goal below the floor",
			statement:  `INSERT INTO core.learning_profiles (user_id, weekly_minutes_goal) VALUES ($1, $2)`,
			args:       []any{userID, 1},
			constraint: "ck_learning_profiles_weekly_goal",
		},
		{
			name:       "email with no local part",
			statement:  `INSERT INTO core.users (email) VALUES ($1)`,
			args:       []any{"@fluentra.test"},
			constraint: "ck_users_email_shape",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, testCase.statement, testCase.args...)
			if err == nil {
				t.Fatalf("row was accepted; %s did not fire", testCase.constraint)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("error = %v, want a PostgreSQL error", err)
			}
			if pgErr.ConstraintName != testCase.constraint {
				t.Fatalf("violated constraint = %q, want %q", pgErr.ConstraintName, testCase.constraint)
			}
		})
	}
}

// TestUserMigration_DownRemovesEverythingItCreated runs in a database of its
// own: rolling the schema back inside the shared one would break whatever
// package is running beside it.
func TestUserMigration_DownRemovesEverythingItCreated(t *testing.T) {
	pool, provider := privateDatabase(t, downDatabase)
	ctx := context.Background()

	assertObjectsExist(t, pool, true)

	// Down to just below this migration, never past bootstrap: that migration
	// drops cluster-wide roles other databases on this server are using.
	if _, err := provider.DownTo(ctx, userSchemaVersion-1); err != nil {
		t.Fatalf("roll back to %d: %v", userSchemaVersion-1, err)
	}
	assertObjectsExist(t, pool, false)

	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version < bootstrapVersion {
		t.Fatalf("rolled back to version %d, past the bootstrap migration", version)
	}

	// Re-applying must work: a down-migration that leaves debris behind passes
	// the assertions above and fails here.
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("re-apply after rollback: %v", err)
	}
	assertObjectsExist(t, pool, true)
}

func assertObjectsExist(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	ctx := context.Background()

	for _, table := range []string{"users", "profiles", "user_preferences", "learning_profiles"} {
		var exists bool
		const query = `SELECT to_regclass('core.' || $1) IS NOT NULL`
		if err := pool.QueryRow(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("check table core.%s: %v", table, err)
		}
		if exists != want {
			t.Errorf("core.%s exists = %v, want %v", table, exists, want)
		}
	}

	for _, typeName := range []string{"user_status", "ui_theme", "cefr_level", "target_exam"} {
		var exists bool
		const query = `
			SELECT EXISTS (
				SELECT 1 FROM pg_type t
				JOIN pg_namespace n ON n.oid = t.typnamespace
				WHERE n.nspname = 'core' AND t.typname = $1
			)`
		if err := pool.QueryRow(ctx, query, typeName).Scan(&exists); err != nil {
			t.Fatalf("check type core.%s: %v", typeName, err)
		}
		if exists != want {
			t.Errorf("type core.%s exists = %v, want %v", typeName, exists, want)
		}
	}
}
