//go:build integration

package resource_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
)

const resourceSchemaVersion = 1700000790

const schemaDatabase = "fluentra_resource_schema_test"
const downDatabase = "fluentra_resource_schema_down_test"

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

func assertCheckViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected CHECK constraint violation %q, got nil", constraint)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected Postgres error, got %T: %v", err, err)
	}
	if pgErr.Code != "23514" {
		t.Fatalf("expected SQLSTATE 23514 (check_violation), got %s (%s)", pgErr.Code, pgErr.Message)
	}
	if pgErr.ConstraintName != constraint {
		t.Fatalf("expected constraint %q, got %q", constraint, pgErr.ConstraintName)
	}
}

func TestMigration_UpAndDown(t *testing.T) {
	ctx := context.Background()
	_, provider := privateDatabase(t, downDatabase)

	// Roll down past the resource migration
	res, err := provider.DownTo(ctx, resourceSchemaVersion-1)
	if err != nil {
		t.Fatalf("DownTo %d: %v", resourceSchemaVersion-1, err)
	}
	if len(res) == 0 {
		t.Fatalf("expected at least one down migration, got none")
	}

	// Migrate back up to latest
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
}

func TestSchema_CKShape(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "shape@example.com")

	// 1. File kind without object_key is refused
	_, err := pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, original_filename, declared_mime, object_key, source_url)
		VALUES ($1, 'file', 'File No Key', 'file.pdf', 'application/pdf', NULL, NULL)
	`, userID)
	assertCheckViolation(t, err, "ck_resources_shape")

	// 2. File kind with source_url is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, original_filename, declared_mime, object_key, source_url)
		VALUES ($1, 'file', 'File With URL', 'file.pdf', 'application/pdf', 'key-1', 'https://example.com')
	`, userID)
	assertCheckViolation(t, err, "ck_resources_shape")

	// 3. URL kind without source_url is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, source_url)
		VALUES ($1, 'url', 'URL No Source', NULL, NULL)
	`, userID)
	assertCheckViolation(t, err, "ck_resources_shape")

	// 4. URL kind with object_key is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, source_url)
		VALUES ($1, 'url', 'URL With Key', 'key-2', 'https://example.com')
	`, userID)
	assertCheckViolation(t, err, "ck_resources_shape")

	// 5. Valid file shape succeeds
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, original_filename, declared_mime, object_key, source_url)
		VALUES ($1, 'file', 'Valid File', 'file.pdf', 'application/pdf', 'valid-key-shape', NULL)
	`, userID)
	if err != nil {
		t.Fatalf("valid file insert failed: %v", err)
	}

	// 6. Valid URL shape succeeds
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, source_url, status)
		VALUES ($1, 'url', 'Valid URL', NULL, 'https://example.com/test', 'uploaded')
	`, userID)
	if err != nil {
		t.Fatalf("valid url insert failed: %v", err)
	}
}

func TestSchema_CKRejectedHasReason(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "rejected_reason@example.com")

	// Status rejected without failure_reason is refused
	_, err := pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, status, failure_reason)
		VALUES ($1, 'file', 'Rejected No Reason', 'key-no-reason', 'rejected', '')
	`, userID)
	assertCheckViolation(t, err, "ck_resources_rejected_has_reason")

	// Status rejected with whitespace only is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, status, failure_reason)
		VALUES ($1, 'file', 'Rejected WS Reason', 'key-ws-reason', 'rejected', '   ')
	`, userID)
	assertCheckViolation(t, err, "ck_resources_rejected_has_reason")

	// Status rejected with valid reason succeeds
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, status, failure_reason)
		VALUES ($1, 'file', 'Rejected Valid', 'key-valid-reason', 'rejected', 'Disallowed MIME type')
	`, userID)
	if err != nil {
		t.Fatalf("rejected with reason failed: %v", err)
	}
}

func TestSchema_CKStatusAndKind(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "status_kind@example.com")

	// Invalid status
	_, err := pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key, status)
		VALUES ($1, 'file', 'Bad Status', 'key-bad-status', 'invalid_status')
	`, userID)
	assertCheckViolation(t, err, "ck_resources_status")

	// Invalid kind
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key)
		VALUES ($1, 'audio', 'Bad Kind', 'key-bad-kind')
	`, userID)
	assertCheckViolation(t, err, "ck_resources_kind")
}

func TestSchema_UQObjectKey(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "uq_key@example.com")

	key := "shared-object-key"
	_, err := pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key)
		VALUES ($1, 'file', 'First Key', $2)
	`, userID, key)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key)
		VALUES ($1, 'file', 'Second Key', $2)
	`, userID, key)
	if err == nil {
		t.Fatalf("expected unique constraint violation on object_key, got nil")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("expected 23505 unique violation, got %v", err)
	}
}

func TestSchema_UserCascadeDelete(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "cascade@example.com")

	_, err := pool.Exec(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key)
		VALUES ($1, 'file', 'To Cascade', 'cascade-key')
	`, userID)
	if err != nil {
		t.Fatalf("insert resource: %v", err)
	}

	// Delete user
	_, err = pool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, userID)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM resource.resources WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		t.Fatalf("count resources: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 resources after cascade delete, got %d", count)
	}
}

// TestResourceSchema_AppRoleCanUseIt. The application connects as fluentra_app,
// and this suite connects as the owner, so without this test a schema created
// without grants passes every other test here and answers "permission denied
// for schema resource" to the first real request in production.
func TestResourceSchema_AppRoleCanUseIt(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const privilegeSQL = `SELECT
		has_schema_privilege('fluentra_app', 'resource', 'USAGE'),
		has_table_privilege('fluentra_app', 'resource.resources', 'SELECT'),
		has_table_privilege('fluentra_app', 'resource.resources', 'INSERT'),
		has_table_privilege('fluentra_app', 'resource.resources', 'UPDATE'),
		has_table_privilege('fluentra_app', 'resource.resources', 'DELETE'),
		has_table_privilege('fluentra_app', 'resource.renditions', 'SELECT'),
		has_table_privilege('fluentra_app', 'resource.renditions', 'INSERT'),
		has_table_privilege('fluentra_app', 'resource.renditions', 'UPDATE'),
		has_table_privilege('fluentra_app', 'resource.renditions', 'DELETE'),
		has_table_privilege('fluentra_app', 'resource.extractions', 'SELECT'),
		has_table_privilege('fluentra_app', 'resource.extractions', 'INSERT'),
		has_table_privilege('fluentra_app', 'resource.extractions', 'UPDATE'),
		has_table_privilege('fluentra_app', 'resource.extractions', 'DELETE'),
		has_table_privilege('fluentra_app', 'resource.classifications', 'SELECT'),
		has_table_privilege('fluentra_app', 'resource.classifications', 'INSERT'),
		has_table_privilege('fluentra_app', 'resource.classifications', 'UPDATE'),
		has_table_privilege('fluentra_app', 'resource.classifications', 'DELETE')`

	var usage, sel, ins, upd, del, rSel, rIns, rUpd, rDel, eSel, eIns, eUpd, eDel, cSel, cIns, cUpd, cDel bool
	if err := pool.QueryRow(ctx, privilegeSQL).Scan(
		&usage, &sel, &ins, &upd, &del, &rSel, &rIns, &rUpd, &rDel,
		&eSel, &eIns, &eUpd, &eDel, &cSel, &cIns, &cUpd, &cDel,
	); err != nil {
		t.Fatalf("query privileges: %v", err)
	}
	if !usage {
		t.Error("fluentra_app has no USAGE on schema resource")
	}
	if !sel || !ins || !upd || !del {
		t.Errorf("fluentra_app privileges on resource.resources: select=%v insert=%v update=%v delete=%v",
			sel, ins, upd, del)
	}
	if !rSel || !rIns || !rUpd || !rDel {
		t.Errorf("fluentra_app privileges on resource.renditions: select=%v insert=%v update=%v delete=%v",
			rSel, rIns, rUpd, rDel)
	}
	if !eSel || !eIns || !eUpd || !eDel {
		t.Errorf("fluentra_app privileges on resource.extractions: select=%v insert=%v update=%v delete=%v",
			eSel, eIns, eUpd, eDel)
	}
	if !cSel || !cIns || !cUpd || !cDel {
		t.Errorf("fluentra_app privileges on resource.classifications: select=%v insert=%v update=%v delete=%v",
			cSel, cIns, cUpd, cDel)
	}
}

func TestSchema_RenditionsConstraints(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "renditions_ck@example.com")

	var resID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key)
		VALUES ($1, 'file', 'File For Renditions', 'file-for-renditions-key')
		RETURNING id
	`, userID).Scan(&resID)
	if err != nil {
		t.Fatalf("insert resource: %v", err)
	}

	// 1. Invalid kind is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.renditions (resource_id, kind, status)
		VALUES ($1, 'invalid_kind', 'pending')
	`, resID)
	assertCheckViolation(t, err, "ck_renditions_kind")

	// 2. Invalid status is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.renditions (resource_id, kind, status)
		VALUES ($1, 'thumbnail', 'unknown_status')
	`, resID)
	assertCheckViolation(t, err, "ck_renditions_status")

	// 3. Status ready without object_key is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.renditions (resource_id, kind, status, object_key)
		VALUES ($1, 'thumbnail', 'ready', NULL)
	`, resID)
	assertCheckViolation(t, err, "ck_renditions_ready_has_object")

	// 4. Negative attempts is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.renditions (resource_id, kind, status, attempts)
		VALUES ($1, 'thumbnail', 'pending', -1)
	`, resID)
	assertCheckViolation(t, err, "ck_renditions_attempts")

	// 5. Valid rendition insert
	validKey := "renditions/" + resID.String() + "/thumbnail.png"
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.renditions (resource_id, kind, status, object_key, width, height)
		VALUES ($1, 'thumbnail', 'ready', $2, 320, 240)
	`, resID, validKey)
	if err != nil {
		t.Fatalf("insert valid rendition: %v", err)
	}

	// 6. Duplicate (resource_id, kind) is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.renditions (resource_id, kind, status, object_key)
		VALUES ($1, 'thumbnail', 'ready', 'another-key')
	`, resID)
	if err == nil {
		t.Fatalf("expected unique violation on (resource_id, kind)")
	}

	// 7. Cascade delete from resource to renditions
	_, err = pool.Exec(ctx, `DELETE FROM resource.resources WHERE id = $1`, resID)
	if err != nil {
		t.Fatalf("delete resource: %v", err)
	}
	var rCount int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM resource.renditions WHERE resource_id = $1`, resID).Scan(&rCount)
	if err != nil {
		t.Fatalf("count renditions after resource delete: %v", err)
	}
	if rCount != 0 {
		t.Fatalf("expected 0 renditions after resource delete, got %d", rCount)
	}
}

func TestSchema_ExtractionsAndClassificationsConstraints(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "stage_b_ck@example.com")

	var resID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO resource.resources (user_id, kind, title, object_key)
		VALUES ($1, 'file', 'File For Stage B', 'stage-b-key')
		RETURNING id
	`, userID).Scan(&resID)
	if err != nil {
		t.Fatalf("insert resource: %v", err)
	}

	// 1. Extractions: Invalid source is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.extractions (resource_id, source, text, char_count)
		VALUES ($1, 'invalid_source', 'hello', 5)
	`, resID)
	assertCheckViolation(t, err, "ck_extractions_source")

	// 2. Extractions: Negative char_count is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.extractions (resource_id, source, text, char_count)
		VALUES ($1, 'pdf_text', 'hello', -1)
	`, resID)
	assertCheckViolation(t, err, "ck_extractions_size")

	// 3. Extractions: char_count > 400,000 is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.extractions (resource_id, source, text, char_count)
		VALUES ($1, 'pdf_text', 'huge', 400001)
	`, resID)
	assertCheckViolation(t, err, "ck_extractions_size")

	// 4. Extractions: valid insert
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.extractions (resource_id, source, text, char_count, language, tool_version)
		VALUES ($1, 'pdf_text', 'Valid extracted text from PDF', 29, 'en', 'pdftotext 24.02.0')
	`, resID)
	if err != nil {
		t.Fatalf("insert valid extraction: %v", err)
	}

	// 5. Classifications: Invalid CEFR level is refused
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.classifications (resource_id, cefr_estimate, skill, prompt_version, model)
		VALUES ($1, 'INVALID', 'reading', '1', 'model')
	`, resID)
	assertCheckViolation(t, err, "ck_classifications_cefr")

	// 6. Classifications: valid insert
	_, err = pool.Exec(ctx, `
		INSERT INTO resource.classifications (resource_id, cefr_estimate, skill, node_codes, prompt_version, model)
		VALUES ($1, 'B1', 'reading', ARRAY['TENSES', 'MODALS'], '1', 'gpt-4o')
	`, resID)
	if err != nil {
		t.Fatalf("insert valid classification: %v", err)
	}

	// 7. Verify ai.ai_budgets has rows for resource_classify
	var budgetCount int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM ai.ai_budgets WHERE task = 'resource_classify'`).Scan(&budgetCount)
	if err != nil {
		t.Fatalf("count resource_classify budgets: %v", err)
	}
	if budgetCount == 0 {
		t.Fatalf("expected ai_budgets rows for resource_classify, got 0")
	}

	// 8. Cascade delete from resource to extractions and classifications
	_, err = pool.Exec(ctx, `DELETE FROM resource.resources WHERE id = $1`, resID)
	if err != nil {
		t.Fatalf("delete resource: %v", err)
	}

	var extCount, clsCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM resource.extractions WHERE resource_id = $1`, resID).Scan(&extCount)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM resource.classifications WHERE resource_id = $1`, resID).Scan(&clsCount)
	if extCount != 0 {
		t.Fatalf("expected 0 extractions after resource delete, got %d", extCount)
	}
	if clsCount != 0 {
		t.Fatalf("expected 0 classifications after resource delete, got %d", clsCount)
	}
}
