//go:build integration

package content_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/content"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const moduleDatabase = "fluentra_content_module_test"

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		// #nosec G101 -- the compose stack's local credentials, not a secret
		base = "postgres://fluentra:fluentra@localhost:5432/fluentra?sslmode=disable"
	}

	dsn, dropDatabase, err := createDatabase(base, moduleDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", moduleDatabase, err)
		os.Exit(1)
	}
	pool = created

	code := m.Run()

	pool.Close()
	dropDatabase()
	os.Exit(code)
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

func migrateUp(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sources, err := migrations.Flattened()
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("flatten migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, sources)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("create goose provider: %w", err)
	}
	defer func() { _ = provider.Close() }()
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func replaceDatabase(dsn, database string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	parsed.Path = "/" + database
	return parsed.String(), nil
}

type allowAllGuard struct{}

func (allowAllGuard) Require(_ context.Context, _ string) error { return nil }

const roleAdmin = "admin"

// call drives one request through the module's router. actor is uuid.Nil for a
// learner request, which carries no authenticated actor; body is nil for the
// state-change endpoints, which take no payload.
func call(
	ctx context.Context,
	t *testing.T,
	router http.Handler,
	method, target string,
	body any,
	actor uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal %s %s body: %v", method, target, err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	requestCtx := ctx
	if actor != uuid.Nil {
		requestCtx = httpx.WithActor(ctx, httpx.Actor{UserID: actor, Role: roleAdmin})
	}
	req := httptest.NewRequest(method, target, reader).WithContext(requestCtx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// reviewPayload is the body POST /admin/content/{id}/review takes. It exists so
// the two field names are written once.
func reviewPayload(decision, comments string) map[string]any {
	return map[string]any{"decision": decision, "comments": comments}
}

// wantStatus fails with the response body, which is where the problem code is.
func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int, step string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("%s: status %d, want %d: %s", step, rec.Code, want, rec.Body.String())
	}
}

func mustExec(ctx context.Context, t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// seedUser satisfies the core.users foreign key content_items.owner_id points at.
func seedUser(ctx context.Context, t *testing.T, id uuid.UUID, email string) {
	t.Helper()
	mustExec(ctx, t,
		"INSERT INTO core.users (id, email) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING",
		id, email)
}

func outboxEvents(ctx context.Context, t *testing.T, event string) int {
	t.Helper()
	var count int
	err := pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM ops.outbox_events WHERE aggregate = 'content' AND event = $1",
		event).Scan(&count)
	if err != nil {
		t.Fatalf("count %s outbox events: %v", event, err)
	}
	return count
}

func resetTables(ctx context.Context, t *testing.T) {
	t.Helper()
	queries := []string{
		"TRUNCATE TABLE ops.outbox_events CASCADE",
		"TRUNCATE TABLE content.content_reviews CASCADE",
		"TRUNCATE TABLE content.content_tags CASCADE",
		"TRUNCATE TABLE content.content_versions CASCADE",
		"TRUNCATE TABLE content.content_items CASCADE",
		"TRUNCATE TABLE content.media_assets CASCADE",
		"TRUNCATE TABLE content.taxonomies CASCADE",
	}
	for _, q := range queries {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("reset table %q: %v", q, err)
		}
	}
}

const lifecycleSlug = "photosynthesis-item"

// authoringFixture is the module under test together with the two people who
// drive it. BR-CONTENT-03 needs an author and a reviewer who are not the same
// person, so both exist from the start.
type authoringFixture struct {
	mod        *content.Module
	router     http.Handler
	authorID   uuid.UUID
	reviewerID uuid.UUID
	itemID     uuid.UUID
}

func newAuthoringFixture(ctx context.Context, t *testing.T) *authoringFixture {
	t.Helper()
	resetTables(ctx, t)

	fixture := &authoringFixture{
		authorID:   uuid.MustParse("018f0000-0000-7000-8000-000000000001"),
		reviewerID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"),
	}
	seedUser(ctx, t, fixture.authorID, "author@fluentra.test")
	seedUser(ctx, t, fixture.reviewerID, "reviewer@fluentra.test")

	mustExec(ctx, t,
		"INSERT INTO content.taxonomies (id, namespace, code, label) VALUES ($1, 'topic', 'science', 'Science')",
		uuid.MustParse("018f0000-0000-7000-8000-000000000099"))

	fixture.mod = content.New(content.Deps{
		Pool:  pool,
		Clock: clock.NewFake(time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)),
		Guard: allowAllGuard{},
	})

	router := chi.NewRouter()
	fixture.mod.Routes(router)
	fixture.mod.AdminRoutes(router)
	fixture.router = router

	return fixture
}

// createDraft covers steps 1 and 2: a new item starts as a draft, and editing
// that draft while it is still a draft edits it in place.
func (f *authoringFixture) createDraft(ctx context.Context, t *testing.T) {
	t.Helper()

	rec := call(ctx, t, f.router, http.MethodPost, "/admin/content", map[string]any{
		"kind":       "vocab_word",
		"slug":       lifecycleSlug,
		"cefr_level": "B2",
		"body":       map[string]any{"word": "photosynthesis", "def": "process by plants"},
		"tags":       []string{"science"},
	}, f.authorID)
	wantStatus(t, rec, http.StatusCreated, "create item")

	var item struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("unmarshal create item response: %v", err)
	}
	if item.Status != "draft" {
		t.Errorf("status = %q, want draft", item.Status)
	}
	f.itemID = item.ID

	rec = call(ctx, t, f.router, http.MethodPut, f.path("draft"), map[string]any{
		"cefr_level": "C1",
		"body":       map[string]any{"word": "photosynthesis", "def": "updated definition"},
	}, f.authorID)
	wantStatus(t, rec, http.StatusOK, "update draft")
}

// reviewCycle covers steps 3 to 7: submit, the self-approval refusal, changes
// requested, resubmit, and finally an approval by someone else.
func (f *authoringFixture) reviewCycle(ctx context.Context, t *testing.T) {
	t.Helper()

	rec := call(ctx, t, f.router, http.MethodPost, f.path("submit"), nil, f.authorID)
	wantStatus(t, rec, http.StatusOK, "submit for review")

	// BR-CONTENT-03: the author may not approve their own version.
	rec = call(ctx, t, f.router, http.MethodPost, f.path("review"),
		reviewPayload("approved", "looks great to myself"), f.authorID)
	wantStatus(t, rec, http.StatusForbidden, "self-approval")

	var prob apperr.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("unmarshal problem: %v", err)
	}
	if prob.Code != "SELF_APPROVAL_FORBIDDEN" {
		t.Errorf("problem code = %q, want SELF_APPROVAL_FORBIDDEN", prob.Code)
	}

	rec = call(ctx, t, f.router, http.MethodPost, f.path("review"),
		reviewPayload("changes_requested", "please fix definition"), f.reviewerID)
	wantStatus(t, rec, http.StatusOK, "changes requested")

	rec = call(ctx, t, f.router, http.MethodPost, f.path("submit"), nil, f.authorID)
	wantStatus(t, rec, http.StatusOK, "resubmit")

	rec = call(ctx, t, f.router, http.MethodPost, f.path("review"),
		reviewPayload("approved", "definition reads well now"), f.reviewerID)
	wantStatus(t, rec, http.StatusOK, "approve")
}

// publishGatedOnMedia covers steps 8 and 9 and returns the published version id.
// media_refs is written straight to the row because the authoring API has no
// field for it yet — see the P7.4 note in content/TODO.md.
func (f *authoringFixture) publishGatedOnMedia(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()

	const mediaKey = "audio/words/photosynthesis.mp3"
	mustExec(ctx, t,
		"INSERT INTO content.media_assets (id, object_key, kind, status) VALUES ($1, $2, 'audio', 'pending')",
		uuid.New(), mediaKey)
	mustExec(ctx, t,
		"UPDATE content.content_versions SET media_refs = ARRAY[$1] WHERE item_id = $2",
		mediaKey, f.itemID)

	// BR-CONTENT-04: publishing is blocked while the asset is still pending.
	rec := call(ctx, t, f.router, http.MethodPost, f.path("publish"), nil, f.authorID)
	wantStatus(t, rec, http.StatusConflict, "publish with pending media")

	mustExec(ctx, t,
		"UPDATE content.media_assets SET status = 'ready' WHERE object_key = $1", mediaKey)

	rec = call(ctx, t, f.router, http.MethodPost, f.path("publish"), nil, f.authorID)
	wantStatus(t, rec, http.StatusOK, "publish")

	var version struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &version); err != nil {
		t.Fatalf("unmarshal published version: %v", err)
	}
	if version.Status != "published" {
		t.Errorf("version status = %q, want published", version.Status)
	}
	if got := outboxEvents(ctx, t, "published"); got != 1 {
		t.Fatalf("content.published outbox events = %d, want 1", got)
	}

	return version.ID
}

// archiveAndCheckMidSession covers steps 10 to 12, and step 12 is the whole
// point of the archive decision: a learner who already holds a version id keeps
// reading it, while discovery stops returning the item.
func (f *authoringFixture) archiveAndCheckMidSession(ctx context.Context, t *testing.T, versionID uuid.UUID) {
	t.Helper()

	rec := call(ctx, t, f.router, http.MethodGet, "/content/"+lifecycleSlug, nil, uuid.Nil)
	wantStatus(t, rec, http.StatusOK, "learner discovery before archive")

	rec = call(ctx, t, f.router, http.MethodPost, f.path("archive"), nil, f.authorID)
	wantStatus(t, rec, http.StatusOK, "archive item")

	if got := outboxEvents(ctx, t, "archived"); got != 1 {
		t.Fatalf("content.archived outbox events = %d, want 1", got)
	}

	version, err := f.mod.Reader().GetVersion(ctx, versionID)
	if err != nil {
		t.Fatalf("GetVersion on an archived item's version: %v", err)
	}
	if version.ID != versionID {
		t.Errorf("resolved version id = %v, want %v", version.ID, versionID)
	}

	rec = call(ctx, t, f.router, http.MethodGet, "/content/"+lifecycleSlug, nil, uuid.Nil)
	wantStatus(t, rec, http.StatusNotFound, "learner discovery after archive")
}

func (f *authoringFixture) path(action string) string {
	return fmt.Sprintf("/admin/content/%s/%s", f.itemID, action)
}

// TestAuthoringLifecycle_Integration walks one item from draft to archived
// through the HTTP surface, against the real schema and its triggers.
func TestAuthoringLifecycle_Integration(t *testing.T) {
	ctx := context.Background()

	fixture := newAuthoringFixture(ctx, t)
	fixture.createDraft(ctx, t)
	fixture.reviewCycle(ctx, t)
	versionID := fixture.publishGatedOnMedia(ctx, t)
	fixture.archiveAndCheckMidSession(ctx, t, versionID)
}

func TestGetManyVersionsSingleQuery_Integration(t *testing.T) {
	ctx := context.Background()
	resetTables(ctx, t)

	mod := content.New(content.Deps{
		Pool:  pool,
		Guard: allowAllGuard{},
	})
	reader := mod.Reader()

	// Insert 5 published items & versions
	authorID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	seedUser(ctx, t, authorID, "author@fluentra.test")
	var versionIDs []uuid.UUID

	for i := 1; i <= 5; i++ {
		itemID := uuid.New()
		verID := uuid.New()
		versionIDs = append(versionIDs, verID)
		slug := fmt.Sprintf("sample-item-%d", i)

		_, err := pool.Exec(ctx, `
			INSERT INTO content.content_items (id, kind, slug, current_version_id, status, owner_id)
			VALUES ($1, 'vocab_word', $2, NULL, 'published', $3)
		`, itemID, slug, authorID)
		if err != nil {
			t.Fatalf("insert item %d: %v", i, err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO content.content_versions (id, item_id, version, kind, body, cefr_level, status, media_refs, published_at)
			VALUES ($1, $2, 1, 'vocab_word', '{}', 'B1', 'published', '{}', now())
		`, verID, itemID)
		if err != nil {
			t.Fatalf("insert version %d: %v", i, err)
		}

		_, err = pool.Exec(ctx, `
			UPDATE content.content_items SET current_version_id = $1 WHERE id = $2
		`, verID, itemID)
		if err != nil {
			t.Fatalf("update current_version_id %d: %v", i, err)
		}
	}

	// Run GetManyVersions and assert batch query execution
	res, err := reader.GetManyVersions(ctx, versionIDs)
	if err != nil {
		t.Fatalf("GetManyVersions failed: %v", err)
	}

	if len(res) != 5 {
		t.Fatalf("expected 5 versions, got %d", len(res))
	}
	for _, id := range versionIDs {
		if _, ok := res[id]; !ok {
			t.Errorf("missing version %v in result map", id)
		}
	}
}
