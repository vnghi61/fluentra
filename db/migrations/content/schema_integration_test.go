//go:build integration

// Package content_test verifies the `content` schema against a real PostgreSQL instance.
package content_test

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
const contentSchemaVersion = 1700000190

const schemaDatabase = "fluentra_content_schema_test"
const downDatabase = "fluentra_content_schema_down_test"

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

// TestContentSchema_EveryForeignKeyIsIndexed asserts that every FK in content schema
// has an index with matching leading columns (Rule DB: foreign keys must be indexed).
func TestContentSchema_EveryForeignKeyIsIndexed(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const foreignKeys = `
		SELECT t.relname, c.conname, c.conkey::int[]
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE c.contype = 'f' AND n.nspname = 'content'
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
		t.Fatal("no foreign keys found in schema content — the migration did not run")
	}

	const indexes = `
		SELECT t.relname, string_to_array(i.indkey::text, ' ')::int[]
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'content'`
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
			t.Errorf("content.%s: foreign key %s has no index whose leading columns match it",
				key.table, key.name)
		}
	}
}

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

// TestContentVersions_PublishedRowIsImmutable proves BR-CONTENT-01:
// An UPDATE on a published content version is rejected by the database trigger.
func TestContentVersions_PublishedRowIsImmutable(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	ownerID := insertUser(t, pool, "content-author@fluentra.test")

	// 1. Create content item
	var itemID string
	const insertItem = `
		INSERT INTO content.content_items (kind, slug, owner_id)
		VALUES ('grammar_rule', 'present-perfect-rule', $1)
		RETURNING id`
	if err := pool.QueryRow(ctx, insertItem, ownerID).Scan(&itemID); err != nil {
		t.Fatalf("insert content item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.content_items WHERE id = $1`, itemID)
	})

	// 2. Create draft content version
	var versionID string
	const insertVersion = `
		INSERT INTO content.content_versions (item_id, version, kind, body, cefr_level, status)
		VALUES ($1, 1, 'grammar_rule', '{"rule":"initial draft"}'::jsonb, 'B1', 'draft')
		RETURNING id`
	if err := pool.QueryRow(ctx, insertVersion, itemID).Scan(&versionID); err != nil {
		t.Fatalf("insert content version draft: %v", err)
	}

	// 3. Modifying a draft version should succeed
	const updateDraft = `
		UPDATE content.content_versions
		SET body = '{"rule":"updated draft"}'::jsonb
		WHERE id = $1`
	if _, err := pool.Exec(ctx, updateDraft, versionID); err != nil {
		t.Fatalf("update draft version failed unexpectedly: %v", err)
	}

	// 4. Publish the version
	const publishVersion = `
		UPDATE content.content_versions
		SET status = 'published', published_at = now()
		WHERE id = $1`
	if _, err := pool.Exec(ctx, publishVersion, versionID); err != nil {
		t.Fatalf("publish version failed: %v", err)
	}

	// 5. Modifying any field on a published version MUST fail
	const attemptUpdateBody = `
		UPDATE content.content_versions
		SET body = '{"rule":"malicious change after publish"}'::jsonb
		WHERE id = $1`
	_, err := pool.Exec(ctx, attemptUpdateBody, versionID)
	if err == nil {
		t.Fatal("UPDATE on published content version succeeded, but BR-CONTENT-01 requires immutability")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want a PostgreSQL error", err)
	}
	if pgErr.Code != "23514" {
		t.Errorf("error code = %q, want 23514 (check_violation)", pgErr.Code)
	}

	// 6. Attempting to change status of published version also fails
	const attemptUpdateStatus = `
		UPDATE content.content_versions
		SET status = 'archived'
		WHERE id = $1`
	_, err = pool.Exec(ctx, attemptUpdateStatus, versionID)
	if err == nil {
		t.Fatal("UPDATE status on published content version succeeded, but published versions are immutable")
	}
}

// TestContentVersions_RepublishIsRejected documents the guard P7.3 must honour:
// the database rejects a second PublishContentVersion on an already-published
// version (BR-CONTENT-01 immutability trigger). P7.3's service must check the
// current status before calling the publish query — "publishing twice is
// idempotent" lives in the service, not in a DB UPDATE that would silently
// bump published_at.
func TestContentVersions_RepublishIsRejected(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	ownerID := insertUser(t, pool, "republish@fluentra.test")

	var itemID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO content.content_items (kind, slug, owner_id)
		VALUES ('grammar_rule', 'republish-guard', $1)
		RETURNING id`, ownerID).Scan(&itemID); err != nil {
		t.Fatalf("insert content item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.content_items WHERE id = $1`, itemID)
	})

	var versionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO content.content_versions (item_id, version, kind, body, cefr_level, status)
		VALUES ($1, 1, 'grammar_rule', '{"rule":"draft"}'::jsonb, 'B1', 'approved')
		RETURNING id`, itemID).Scan(&versionID); err != nil {
		t.Fatalf("insert content version: %v", err)
	}

	// First publish succeeds.
	const publish = `
		UPDATE content.content_versions
		SET status = 'published', published_at = COALESCE(published_at, now())
		WHERE id = $1`
	if _, err := pool.Exec(ctx, publish, versionID); err != nil {
		t.Fatalf("first publish failed: %v", err)
	}

	// Second publish is rejected by the immutability trigger.
	_, err := pool.Exec(ctx, publish, versionID)
	if err == nil {
		t.Fatal("re-publishing a published version succeeded; the trigger must reject it so P7.3 guards before calling")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("error = %v, want check_violation (23514) from the immutability trigger", err)
	}
}

// TestContentSchema_CheckConstraintsRejectInvalidRows verifies table invariants.
func TestContentSchema_CheckConstraintsRejectInvalidRows(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	ownerID := insertUser(t, pool, "content-checks@fluentra.test")

	// Insert valid content item for version FK tests
	var validItemID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO content.content_items (kind, slug, owner_id)
		VALUES ('vocab_word', 'sample-valid-slug', $1)
		RETURNING id`, ownerID).Scan(&validItemID); err != nil {
		t.Fatalf("insert valid item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.content_items WHERE id = $1`, validItemID)
	})

	cases := []struct {
		name       string
		statement  string
		args       []any
		constraint string
	}{
		{
			name:       "item slug with uppercase letters",
			statement:  `INSERT INTO content.content_items (kind, slug, owner_id) VALUES ('vocab_word', 'Invalid_Slug', $1)`,
			args:       []any{ownerID},
			constraint: "ck_content_items_slug_format",
		},
		{
			name:       "item slug with double hyphens",
			statement:  `INSERT INTO content.content_items (kind, slug, owner_id) VALUES ('vocab_word', 'invalid--slug', $1)`,
			args:       []any{ownerID},
			constraint: "ck_content_items_slug_format",
		},
		{
			name: "version number zero",
			statement: `INSERT INTO content.content_versions (item_id, version, kind, cefr_level)
				VALUES ($1, 0, 'vocab_word', 'B1')`,
			args:       []any{validItemID},
			constraint: "ck_content_versions_version_positive",
		},
		{
			name: "invalid CEFR level",
			statement: `INSERT INTO content.content_versions (item_id, version, kind, cefr_level)
				VALUES ($1, 2, 'vocab_word', 'X9')`,
			args:       []any{validItemID},
			constraint: "ck_content_versions_cefr_level",
		},
		{
			// The CEFRLevel schema is enum [A1..C2]. A lowercase row would round-trip
			// out of the API as a value the published contract does not allow, so the
			// constraint is case-sensitive and this is what proves it.
			name: "lowercase CEFR level",
			statement: `INSERT INTO content.content_versions (item_id, version, kind, cefr_level)
				VALUES ($1, 4, 'vocab_word', 'b1')`,
			args:       []any{validItemID},
			constraint: "ck_content_versions_cefr_level",
		},
		{
			name: "published status without published_at",
			statement: `INSERT INTO content.content_versions (item_id, version, kind, cefr_level, status, published_at)
				VALUES ($1, 3, 'vocab_word', 'B1', 'published', NULL)`,
			args:       []any{validItemID},
			constraint: "ck_content_versions_published_at",
		},
		{
			name: "media asset negative duration",
			statement: `INSERT INTO content.media_assets (object_key, kind, duration_ms)
				VALUES ('audio/lesson1.mp3', 'audio', -500)`,
			args:       nil,
			constraint: "ck_media_assets_duration_ms",
		},
		{
			name: "media asset negative byte size",
			statement: `INSERT INTO content.media_assets (object_key, kind, byte_size)
				VALUES ('audio/lesson2.mp3', 'audio', -10)`,
			args:       nil,
			constraint: "ck_media_assets_byte_size",
		},
		{
			name:       "taxonomy empty namespace",
			statement:  `INSERT INTO content.taxonomies (namespace, code, label) VALUES ('', 'code1', 'Label 1')`,
			args:       nil,
			constraint: "ck_taxonomies_namespace_length",
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

// TestContentItems_SlugIsUnique proves slug uniqueness constraint uq_content_items_slug.
func TestContentItems_SlugIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	ownerID := insertUser(t, pool, "slug-uniq@fluentra.test")

	const insert = `
		INSERT INTO content.content_items (kind, slug, owner_id)
		VALUES ('grammar_rule', 'unique-slug-test', $1)
		RETURNING id`
	var itemID string
	if err := pool.QueryRow(ctx, insert, ownerID).Scan(&itemID); err != nil {
		t.Fatalf("insert first item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.content_items WHERE id = $1`, itemID)
	})

	_, err := pool.Exec(ctx, insert, ownerID)
	if err == nil {
		t.Fatal("duplicate slug was accepted; uq_content_items_slug did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_content_items_slug" {
		t.Errorf("violated constraint = %q, want uq_content_items_slug", pgErr.ConstraintName)
	}
}

// TestContentVersions_ItemVersionIsUnique proves compound uniqueness uq_content_versions_item_version.
func TestContentVersions_ItemVersionIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	ownerID := insertUser(t, pool, "version-uniq@fluentra.test")

	var itemID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO content.content_items (kind, slug, owner_id)
		VALUES ('vocab_word', 'version-uniq-item', $1)
		RETURNING id`, ownerID).Scan(&itemID); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.content_items WHERE id = $1`, itemID)
	})

	const insertVersion = `
		INSERT INTO content.content_versions (item_id, version, kind, cefr_level)
		VALUES ($1, 1, 'vocab_word', 'B1')`
	if _, err := pool.Exec(ctx, insertVersion, itemID); err != nil {
		t.Fatalf("insert version 1: %v", err)
	}

	_, err := pool.Exec(ctx, insertVersion, itemID)
	if err == nil {
		t.Fatal("duplicate version 1 was accepted; uq_content_versions_item_version did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_content_versions_item_version" {
		t.Errorf("violated constraint = %q, want uq_content_versions_item_version", pgErr.ConstraintName)
	}
}

// TestTaxonomies_NamespaceCodeIsUnique proves compound uniqueness uq_taxonomies_namespace_code.
func TestTaxonomies_NamespaceCodeIsUnique(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const insertTaxonomy = `
		INSERT INTO content.taxonomies (namespace, code, label)
		VALUES ('topic', 'travel', 'Travel & Tourism')
		RETURNING id`
	var id string
	if err := pool.QueryRow(ctx, insertTaxonomy).Scan(&id); err != nil {
		t.Fatalf("insert taxonomy: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM content.taxonomies WHERE id = $1`, id)
	})

	_, err := pool.Exec(ctx, insertTaxonomy)
	if err == nil {
		t.Fatal("duplicate (namespace, code) accepted; uq_taxonomies_namespace_code did not fire")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("error = %v, want unique_violation (23505)", err)
	}
	if pgErr.ConstraintName != "uq_taxonomies_namespace_code" {
		t.Errorf("violated constraint = %q, want uq_taxonomies_namespace_code", pgErr.ConstraintName)
	}
}

// TestContentMigration_DownRemovesEverythingItCreated tests migration reversibility.
func TestContentMigration_DownRemovesEverythingItCreated(t *testing.T) {
	pool, provider := privateDatabase(t, downDatabase)
	ctx := context.Background()

	assertContentObjectsExist(t, pool, true)

	if _, err := provider.DownTo(ctx, contentSchemaVersion-1); err != nil {
		t.Fatalf("roll back to %d: %v", contentSchemaVersion-1, err)
	}
	assertContentObjectsExist(t, pool, false)

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
	assertContentObjectsExist(t, pool, true)
}

func assertContentObjectsExist(t *testing.T, pool *pgxpool.Pool, want bool) {
	t.Helper()
	ctx := context.Background()

	tables := []string{
		"content_items",
		"content_versions",
		"media_assets",
		"taxonomies",
		"content_tags",
		"content_reviews",
	}
	for _, table := range tables {
		var exists bool
		const query = `SELECT to_regclass('content.' || $1) IS NOT NULL`
		if err := pool.QueryRow(ctx, query, table).Scan(&exists); err != nil {
			t.Fatalf("check table content.%s: %v", table, err)
		}
		if exists != want {
			t.Errorf("content.%s exists = %v, want %v", table, exists, want)
		}
	}

	types := []string{
		"authoring_status",
		"review_decision",
		"media_status",
	}
	for _, typeName := range types {
		var exists bool
		const query = `
			SELECT EXISTS (
				SELECT 1 FROM pg_type t
				JOIN pg_namespace n ON n.oid = t.typnamespace
				WHERE n.nspname = 'content' AND t.typname = $1
			)`
		if err := pool.QueryRow(ctx, query, typeName).Scan(&exists); err != nil {
			t.Fatalf("check type content.%s: %v", typeName, err)
		}
		if exists != want {
			t.Errorf("type content.%s exists = %v, want %v", typeName, exists, want)
		}
	}
}
