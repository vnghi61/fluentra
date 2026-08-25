//go:build integration

// Package vocabulary_test verifies the `skill` schema against a real PostgreSQL
// instance.
//
// It is the first test of any kind over `skill`, so unlike the srs file it does
// assert the schema-wide rules — every foreign key indexed, and no cross-schema
// key outside core.users — because nothing else does for this schema.
package vocabulary_test

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
const vocabularySchemaVersion = 1700000230

const schemaDatabase = "fluentra_vocabulary_schema_test"
const downDatabase = "fluentra_vocabulary_schema_down_test"

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
		WHERE c.contype = 'f' AND n.nspname = 'skill'
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
		WHERE n.nspname = 'skill'`
	rows, err := pool.Query(ctx, indexes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var idxs []indexDef
	for rows.Next() {
		var idx indexDef
		if err := rows.Scan(&idx.table, &idx.name, &idx.columns); err != nil {
			return nil, err
		}
		idxs = append(idxs, idx)
	}
	return idxs, rows.Err()
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

// TestVocabularySchema_EveryForeignKeyIsIndexed asserts the DB rule that every
// foreign key carries an index with matching leading columns. Without it a
// delete on the parent table degrades into a sequential scan of the child.
func TestVocabularySchema_EveryForeignKeyIsIndexed(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	fks, err := fetchForeignKeys(ctx, pool)
	if err != nil {
		t.Fatalf("read foreign keys: %v", err)
	}
	if len(fks) == 0 {
		t.Fatal("no foreign keys discovered in skill schema; check the query")
	}

	idxs, err := fetchIndexes(ctx, pool)
	if err != nil {
		t.Fatalf("read indexes: %v", err)
	}

	for _, fk := range fks {
		if !isFKCovered(fk, idxs) {
			t.Errorf("foreign key skill.%s.%s (columns %v) is not covered by any index prefix",
				fk.table, fk.name, fk.columns)
		}
	}
}

// TestVocabularySchema_CrossSchemaForeignKeysRestrictedToUsers proves DB4 for
// `skill`: the only key that may leave the schema is to core.users(id).
//
// This is the rule that stops `vocabulary` reaching into content.media_assets or
// learn.review_cards with a constraint instead of going through a contract, and
// no linter sees SQL — this test is the enforcement.
func TestVocabularySchema_CrossSchemaForeignKeysRestrictedToUsers(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const query = `
		SELECT t.relname, c.conname, fn.nspname AS target_schema, ft.relname AS target_table
		FROM pg_constraint c
		JOIN pg_class t       ON t.oid  = c.conrelid
		JOIN pg_namespace n   ON n.oid  = t.relnamespace
		JOIN pg_class ft      ON ft.oid = c.confrelid
		JOIN pg_namespace fn  ON fn.oid = ft.relnamespace
		WHERE c.contype = 'f' AND n.nspname = 'skill';`

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

		if targetSchema == "skill" {
			continue
		}
		if targetSchema == "core" && targetTable == "users" {
			continue
		}

		t.Errorf("DB4 violation: skill.%s constraint %s references %s.%s (must be in 'skill' or 'core.users')",
			table, constraint, targetSchema, targetTable)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign keys: %v", err)
	}
}

// TestVocabularySchema_NoAttemptTable is the review-blocking rule from P9.4.
//
// `vocabulary` records no attempts of its own — `learn.attempts` is the single
// attempt table, and a second one is exactly the duplication ADR-0015 exists to
// prevent. A table named for attempts appearing in `skill` is the cheapest
// possible signal that someone rebuilt the exercise engine inside a skill module.
func TestVocabularySchema_NoAttemptTable(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const query = `
		SELECT c.relname FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'skill' AND c.relkind = 'r'
		  AND (c.relname LIKE '%attempt%' OR c.relname LIKE '%review_card%')`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("query skill tables: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		t.Errorf("skill.%s looks like a second attempt or review-card table; "+
			"attempts belong to learn.attempts and cards to learn.review_cards", table)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate skill tables: %v", err)
	}
}

// TestVocabularySchema_CheckConstraintsEnforced proves each check constraint fires.
func TestVocabularySchema_CheckConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	userID := seedUser(ctx, t, pool, "vocab-checks-test@example.com")
	wordID := seedWord(ctx, t, pool, "meticulous", "adjective")
	senseID := seedSense(ctx, t, pool, wordID, "Showing great attention to detail.")

	cases := []struct {
		name       string
		constraint string
		statement  string
		args       []any
	}{
		{
			name:       "a CEFR level outside A1..C2",
			constraint: "ck_words_cefr",
			statement:  `INSERT INTO skill.words (lemma, pos, cefr_level) VALUES ('ineffable', 'adjective', 'D1')`,
		},
		{
			name:       "a relation type nobody defined",
			constraint: "ck_word_relations_type",
			statement: `INSERT INTO skill.word_relations (from_word_id, to_word_id, relation)
			            VALUES ($1, $1, 'rhymes_with')`,
			args: []any{wordID},
		},
		{
			name:       "a learner word status outside the four",
			constraint: "ck_user_word_state_status",
			statement: `INSERT INTO skill.user_word_state (user_id, word_sense_id, status)
			            VALUES ($1, $2, 'mastered')`,
			args: []any{userID, senseID},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.statement, tc.args...)
			assertConstraintViolation(t, err, tc.constraint)
		})
	}
}

// TestVocabularySchema_UniqueConstraintsEnforced covers the uniqueness rules,
// including the one that is easy to get wrong.
func TestVocabularySchema_UniqueConstraintsEnforced(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	userID := seedUser(ctx, t, pool, "vocab-unique-test@example.com")

	t.Run("one word per lemma and part of speech", func(t *testing.T) {
		const insert = `INSERT INTO skill.words (lemma, pos, cefr_level) VALUES ('bank', 'noun', 'A2')`
		if _, err := pool.Exec(ctx, insert); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		_, err := pool.Exec(ctx, insert)
		assertConstraintViolation(t, err, "uq_words_lemma_pos")
	})

	// Curated decks have no owner. Under the default NULLS DISTINCT, two curated
	// decks could share a slug, because NULL <> NULL — and the catalogue would
	// show duplicates that no unique constraint prevented. The migration says
	// NULLS NOT DISTINCT precisely to close that, so it is worth an assertion.
	t.Run("curated decks with no owner still have unique slugs", func(t *testing.T) {
		const insert = `INSERT INTO skill.decks (owner_id, slug, name, is_public)
		                VALUES (NULL, 'core-a1', 'Core A1', true)`
		if _, err := pool.Exec(ctx, insert); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		_, err := pool.Exec(ctx, insert)
		assertConstraintViolation(t, err, "uq_decks_owner_slug")
	})

	t.Run("a learner may reuse a slug a curated deck already took", func(t *testing.T) {
		const insert = `INSERT INTO skill.decks (owner_id, slug, name) VALUES ($1, 'core-a1', 'My Core A1')`
		if _, err := pool.Exec(ctx, insert, userID); err != nil {
			t.Fatalf("a learner deck must not collide with a curated one: %v", err)
		}
	})

	t.Run("one state row per learner per sense", func(t *testing.T) {
		wordID := seedWord(ctx, t, pool, "ephemeral", "adjective")
		senseID := seedSense(ctx, t, pool, wordID, "Lasting for a very short time.")

		const insert = `INSERT INTO skill.user_word_state (user_id, word_sense_id, status)
		                VALUES ($1, $2, 'learning')`
		if _, err := pool.Exec(ctx, insert, userID, senseID); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		_, err := pool.Exec(ctx, insert, userID, senseID)
		assertConstraintViolation(t, err, "uq_user_word_state")
	})
}

// TestVocabularyMigration_DownRemovesEverythingItCreated tests reversibility.
func TestVocabularyMigration_DownRemovesEverythingItCreated(t *testing.T) {
	pool, provider := privateDatabase(t, downDatabase)
	ctx := context.Background()

	assertVocabularyObjectsExist(t, pool, true)

	if _, err := provider.DownTo(ctx, vocabularySchemaVersion-1); err != nil {
		t.Fatalf("roll back to %d: %v", vocabularySchemaVersion-1, err)
	}
	assertVocabularyObjectsExist(t, pool, false)

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
	assertVocabularyObjectsExist(t, pool, true)
}

func assertVocabularyObjectsExist(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	ctx := context.Background()

	tables := []string{"words", "word_senses", "word_relations", "decks", "deck_items", "user_word_state"}
	for _, table := range tables {
		var exists bool
		const query = `SELECT to_regclass('skill.' || $1) IS NOT NULL`
		if err := pool.QueryRow(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("check table skill.%s: %v", table, err)
		}
		if exists != want {
			t.Errorf("skill.%s exists = %v, want %v", table, exists, want)
		}
	}
}

func seedUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()

	var userID string
	const insert = `INSERT INTO core.users (email, status) VALUES ($1, 'active') RETURNING id`
	if err := pool.QueryRow(ctx, insert, email).Scan(&userID); err != nil {
		t.Fatalf("seed user %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID)
	})
	return userID
}

func seedWord(ctx context.Context, t *testing.T, pool *pgxpool.Pool, lemma, pos string) string {
	t.Helper()

	var wordID string
	const insert = `INSERT INTO skill.words (lemma, pos, cefr_level) VALUES ($1, $2, 'B2') RETURNING id`
	if err := pool.QueryRow(ctx, insert, lemma, pos).Scan(&wordID); err != nil {
		t.Fatalf("seed word %s: %v", lemma, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM skill.words WHERE id = $1`, wordID)
	})
	return wordID
}

func seedSense(ctx context.Context, t *testing.T, pool *pgxpool.Pool, wordID, definition string) string {
	t.Helper()

	var senseID string
	const insert = `INSERT INTO skill.word_senses (word_id, definition) VALUES ($1, $2) RETURNING id`
	if err := pool.QueryRow(ctx, insert, wordID, definition).Scan(&senseID); err != nil {
		t.Fatalf("seed sense: %v", err)
	}
	return senseID
}

// assertConstraintViolation fails unless err is the named constraint firing.
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
