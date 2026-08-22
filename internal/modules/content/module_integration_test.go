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

func (allowAllGuard) Require(ctx context.Context, permission string) error { return nil }

func resetTables(t *testing.T) {
	t.Helper()
	ctx := context.Background()
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

func TestAuthoringLifecycle_Integration(t *testing.T) {
	resetTables(t)
	ctx := context.Background()

	authorID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	reviewerID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")

	// Ensure core.users exists for FK
	_, _ = pool.Exec(ctx, "INSERT INTO core.users (id, email) VALUES ($1, 'author@fluentra.test') ON CONFLICT (id) DO NOTHING", authorID)
	_, _ = pool.Exec(ctx, "INSERT INTO core.users (id, email) VALUES ($1, 'reviewer@fluentra.test') ON CONFLICT (id) DO NOTHING", reviewerID)

	// Prepopulate taxonomy
	taxID := uuid.MustParse("018f0000-0000-7000-8000-000000000099")
	_, err := pool.Exec(ctx, "INSERT INTO content.taxonomies (id, namespace, code, label) VALUES ($1, 'topic', 'science', 'Science')", taxID)
	if err != nil {
		t.Fatalf("insert taxonomy: %v", err)
	}

	fixedTime := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	mod := content.New(content.Deps{
		Pool:  pool,
		Clock: clock.NewFake(fixedTime),
		Guard: allowAllGuard{},
	})

	router := chi.NewRouter()
	mod.Routes(router)
	mod.AdminRoutes(router)

	// 1. Create content item (draft)
	createReq := map[string]any{
		"kind":       "vocab_word",
		"slug":       "photosynthesis-item",
		"cefr_level": "B2",
		"body":       map[string]any{"word": "photosynthesis", "def": "process by plants"},
		"tags":       []string{"science"},
	}
	createBody, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/admin/content", bytes.NewReader(createBody))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create item failed with status %d: %s", rec.Code, rec.Body.String())
	}

	var itemResp struct {
		ID               uuid.UUID  `json:"id"`
		Kind             string     `json:"kind"`
		Slug             string     `json:"slug"`
		CurrentVersionID *uuid.UUID `json:"current_version_id"`
		Status           string     `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &itemResp); err != nil {
		t.Fatalf("unmarshal create item response: %v", err)
	}
	if itemResp.Status != "draft" {
		t.Errorf("status = %q, want draft", itemResp.Status)
	}
	itemID := itemResp.ID

	// 2. Update draft
	updateReq := map[string]any{
		"cefr_level": "C1",
		"body":       map[string]any{"word": "photosynthesis", "def": "updated definition"},
	}
	updateBody, _ := json.Marshal(updateReq)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/content/%s/draft", itemID), bytes.NewReader(updateBody))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update draft failed with status %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Submit for review
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/submit", itemID), nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("submit for review failed with status %d: %s", rec.Code, rec.Body.String())
	}

	// 4. BR-CONTENT-03 Self-approval rejection
	reviewReq := map[string]any{
		"decision": "approved",
		"comments": "looks great to myself",
	}
	reviewBody, _ := json.Marshal(reviewReq)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/review", itemID), bytes.NewReader(reviewBody))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for self-approval, got %d: %s", rec.Code, rec.Body.String())
	}
	var prob apperr.Problem
	_ = json.Unmarshal(rec.Body.Bytes(), &prob)
	if prob.Code != "SELF_APPROVAL_FORBIDDEN" {
		t.Errorf("problem code = %q, want SELF_APPROVAL_FORBIDDEN", prob.Code)
	}

	// 5. Reviewer requests changes
	reviewReq["decision"] = "changes_requested"
	reviewReq["comments"] = "please fix definition"
	reviewBody, _ = json.Marshal(reviewReq)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/review", itemID), bytes.NewReader(reviewBody))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: reviewerID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("review changes_requested failed with status %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Author resubmits
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/submit", itemID), nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resubmit failed with status %d: %s", rec.Code, rec.Body.String())
	}

	// 7. Reviewer approves
	reviewReq["decision"] = "approved"
	reviewBody, _ = json.Marshal(reviewReq)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/review", itemID), bytes.NewReader(reviewBody))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: reviewerID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("review approve failed with status %d: %s", rec.Code, rec.Body.String())
	}

	// 8. BR-CONTENT-04 Check: Add pending media asset ref directly to draft version
	mediaKey := "audio/words/photosynthesis.mp3"
	_, err = pool.Exec(ctx, "INSERT INTO content.media_assets (id, object_key, kind, status) VALUES ($1, $2, 'audio', 'pending')", uuid.New(), mediaKey)
	if err != nil {
		t.Fatalf("insert pending media asset: %v", err)
	}

	_, err = pool.Exec(ctx, "UPDATE content.content_versions SET media_refs = ARRAY[$1] WHERE item_id = $2", mediaKey, itemID)
	if err != nil {
		t.Fatalf("update version media_refs: %v", err)
	}

	// Attempt publish -> should fail with MEDIA_NOT_READY
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/publish", itemID), nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for non-ready media publish, got %d: %s", rec.Code, rec.Body.String())
	}

	// Mark media as ready
	_, err = pool.Exec(ctx, "UPDATE content.media_assets SET status = 'ready' WHERE object_key = $1", mediaKey)
	if err != nil {
		t.Fatalf("mark media ready: %v", err)
	}

	// 9. Publish succeeds
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/publish", itemID), nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("publish failed with status %d: %s", rec.Code, rec.Body.String())
	}

	var pubVerResp struct {
		ID          uuid.UUID `json:"id"`
		Status      string    `json:"status"`
		PublishedAt string    `json:"published_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pubVerResp); err != nil {
		t.Fatalf("unmarshal pub ver response: %v", err)
	}
	if pubVerResp.Status != "published" {
		t.Errorf("version status = %q, want published", pubVerResp.Status)
	}

	// Verify outbox has content.published
	var outboxCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM ops.outbox_events WHERE aggregate = 'content' AND event = 'published'").Scan(&outboxCount)
	if err != nil || outboxCount != 1 {
		t.Fatalf("expected 1 content.published outbox event, got %d (err: %v)", outboxCount, err)
	}

	// 10. Learner discovery
	req = httptest.NewRequest(http.MethodGet, "/content/photosynthesis-item", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("learner get by slug failed: %d: %s", rec.Code, rec.Body.String())
	}

	// 11. Archive item
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/content/%s/archive", itemID), nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: authorID, Role: "admin"}))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("archive item failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Verify outbox has content.archived
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM ops.outbox_events WHERE aggregate = 'content' AND event = 'archived'").Scan(&outboxCount)
	if err != nil || outboxCount != 1 {
		t.Fatalf("expected 1 content.archived outbox event, got %d (err: %v)", outboxCount, err)
	}

	// 12. Archive-mid-session check:
	// Direct GetVersion by ID still succeeds (does NOT 404)
	reader := mod.Reader()
	v, err := reader.GetVersion(ctx, pubVerResp.ID)
	if err != nil {
		t.Fatalf("direct GetVersion on archived item failed: %v", err)
	}
	if v.ID != pubVerResp.ID {
		t.Errorf("resolved version ID = %v, want %v", v.ID, pubVerResp.ID)
	}

	// Discovery by slug fails (404)
	req = httptest.NewRequest(http.MethodGet, "/content/photosynthesis-item", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for archived item discovery, got %d", rec.Code)
	}
}

func TestGetManyVersionsSingleQuery_Integration(t *testing.T) {
	resetTables(t)
	ctx := context.Background()

	mod := content.New(content.Deps{
		Pool:  pool,
		Guard: allowAllGuard{},
	})
	reader := mod.Reader()

	// Insert 5 published items & versions
	authorID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	_, _ = pool.Exec(ctx, "INSERT INTO core.users (id, email) VALUES ($1, 'author@fluentra.test') ON CONFLICT (id) DO NOTHING", authorID)
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
