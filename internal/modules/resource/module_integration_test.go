//go:build integration

package resource_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/resource"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	resourcejob "github.com/fluentra/fluentra/internal/modules/resource/job"
	"github.com/fluentra/fluentra/internal/modules/resource/service"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const moduleDatabase = "fluentra_resource_module_test"

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
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

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	const insert = `INSERT INTO core.users (email) VALUES ($1) RETURNING id`
	if err := pool.QueryRow(ctx, insert, email).Scan(&id); err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.users WHERE id = $1`, id)
	})
	return id
}

type inMemoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{objects: make(map[string][]byte)}
}

func (s *inMemoryStore) PresignPut(
	_ context.Context, bucket, key, contentType string, maxBytes int64, expiry time.Duration,
) (storage.UploadIntent, error) {
	return storage.UploadIntent{
		URL:         "http://storage.local/" + bucket + "/" + key,
		Method:      "PUT",
		ObjectKey:   key,
		ExpiresAt:   time.Now().Add(expiry),
		MaxBytes:    maxBytes,
		ContentType: contentType,
	}, nil
}

func (s *inMemoryStore) PresignGet(
	_ context.Context, bucket, key string, _ time.Duration,
) (string, error) {
	return "http://storage.local/download/" + bucket + "/" + key, nil
}

func (s *inMemoryStore) Stat(_ context.Context, _, key string) (storage.ObjectStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return storage.ObjectStat{}, storage.ErrObjectNotFound
	}
	return storage.ObjectStat{
		Key:  key,
		Size: int64(len(data)),
	}, nil
}

func (s *inMemoryStore) Get(_ context.Context, _, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *inMemoryStore) Put(
	_ context.Context, _, key string, reader io.Reader, _ int64, _ string,
) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	return nil
}

func (s *inMemoryStore) Copy(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (s *inMemoryStore) Delete(_ context.Context, _, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func (s *inMemoryStore) VerifyUpload(
	ctx context.Context, bucket, key, _ string, _ int64,
) (storage.ObjectStat, error) {
	return s.Stat(ctx, bucket, key)
}

func (s *inMemoryStore) hasObject(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.objects[key]
	return ok
}

type fakeURLFetcher struct {
	meta *service.URLMetadata
	err  error
}

func (f *fakeURLFetcher) FetchMetadata(_ context.Context, _ string) (*service.URLMetadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.meta, nil
}

func doRequest(
	handler http.Handler, method, path, body string, userID uuid.UUID,
) *httptest.ResponseRecorder {
	var reader io.Reader = http.NoBody
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := httpx.WithActor(req.Context(), httpx.Actor{
		UserID: userID,
		Role:   "user",
	})
	ctx = httpx.WithRequestID(ctx, "01J8XQ7Z9K3M4N5P6Q7R8S9T0V")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestModule_WorkOrder17Gate executes the end-to-end lifecycle gate specified in WO-17 §11:
// 1. POST /me/resources/upload-intent -> row at pending
// 2. PUT bytes to storage
// 3. POST /me/resources {resource_id} -> row at uploaded
// 4. validation job runs
// 5. GET /me/resources/{id} -> status validated, detected_mime "application/pdf"
func TestModule_WorkOrder17Gate(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	userA := insertUser(t, pool, "wo17_gate@example.com")
	store := newInMemoryStore()
	fetcher := &fakeURLFetcher{
		meta: &service.URLMetadata{FinalURL: "https://example.com/guide", Title: "Guide"},
	}

	mod := resource.New(resource.Deps{
		Pool:       pool,
		Storage:    store,
		URLFetcher: fetcher,
	})

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		mod.Routes(r)
	})

	resourceID, objectKey := createGateIntent(t, router, userA)

	reader := mod.Reader()
	res, err := reader.GetResource(context.Background(), resourceID, userA)
	if err != nil {
		t.Fatalf("get resource before upload: %v", err)
	}
	if res.Status != domain.StatusPending {
		t.Fatalf("expected pending status, got %s", res.Status)
	}

	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF")
	err = store.Put(
		context.Background(),
		storage.BucketUploads,
		objectKey,
		bytes.NewReader(pdfContent),
		int64(len(pdfContent)),
		"application/pdf",
	)
	if err != nil {
		t.Fatalf("put object to storage: %v", err)
	}

	submitGateUpload(t, router, userA, resourceID)

	worker := mod.ValidateWorker()
	riverJob := &river.Job[resourcejob.ValidateResourceArgs]{
		Args: resourcejob.ValidateResourceArgs{ResourceID: resourceID},
	}
	if err := worker.Work(context.Background(), riverJob); err != nil {
		t.Fatalf("validation worker failed: %v", err)
	}

	verifyGateValidated(t, router, userA, resourceID)
}

func createGateIntent(t *testing.T, router http.Handler, user uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	intentBody := `{"filename":"grammar.pdf","content_type":"application/pdf"}`
	rec := doRequest(router, http.MethodPost, "/api/v1/me/resources/upload-intent", intentBody, user)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload-intent failed: code %d, body %s", rec.Code, rec.Body)
	}

	var resp struct {
		ID        uuid.UUID `json:"id"`
		UploadURL string    `json:"upload_url"`
		ObjectKey string    `json:"object_key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal intent response: %v", err)
	}
	if resp.ID == uuid.Nil || resp.ObjectKey == "" {
		t.Fatalf("invalid intent response: %+v", resp)
	}
	return resp.ID, resp.ObjectKey
}

func submitGateUpload(t *testing.T, router http.Handler, user, resourceID uuid.UUID) {
	t.Helper()
	submitBody := fmt.Sprintf(`{"resource_id":"%s"}`, resourceID)
	rec := doRequest(router, http.MethodPost, "/api/v1/me/resources", submitBody, user)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit file failed: code %d, body %s", rec.Code, rec.Body)
	}

	var submitResp struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &submitResp); err != nil {
		t.Fatalf("unmarshal submit response: %v", err)
	}
	if submitResp.Status != domain.StatusUploaded {
		t.Fatalf("expected status 'uploaded', got %q", submitResp.Status)
	}
}

func verifyGateValidated(t *testing.T, router http.Handler, user, resourceID uuid.UUID) {
	t.Helper()
	getURL := fmt.Sprintf("/api/v1/me/resources/%s", resourceID)
	rec := doRequest(router, http.MethodGet, getURL, "", user)
	if rec.Code != http.StatusOK {
		t.Fatalf("get resource after validation failed: code %d, body %s", rec.Code, rec.Body)
	}

	var finalRes struct {
		ID           uuid.UUID `json:"id"`
		Status       string    `json:"status"`
		DetectedMIME string    `json:"detected_mime"`
		DownloadURL  *string   `json:"download_url"`
		Checksum     *string   `json:"checksum"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &finalRes); err != nil {
		t.Fatalf("unmarshal final resource: %v", err)
	}
	if finalRes.Status != domain.StatusValidated {
		t.Fatalf("expected status %q, got %q", domain.StatusValidated, finalRes.Status)
	}
	if finalRes.DetectedMIME != "application/pdf" {
		t.Fatalf("expected detected_mime 'application/pdf', got %q", finalRes.DetectedMIME)
	}
	if finalRes.DownloadURL == nil || *finalRes.DownloadURL == "" {
		t.Fatalf("expected download_url to be present for validated file resource")
	}
	if finalRes.Checksum == nil || *finalRes.Checksum == "" {
		t.Fatalf("expected checksum to be recorded")
	}
}

// TestModule_OwnershipIsolation verifies BR-RESOURCE-01 and D17-7:
// Accessing another user's resource returns 404 (not 403) from GET and DELETE.
func TestModule_OwnershipIsolation(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	userA := insertUser(t, pool, "owner_a@example.com")
	userB := insertUser(t, pool, "owner_b@example.com")
	store := newInMemoryStore()

	mod := resource.New(resource.Deps{
		Pool:    pool,
		Storage: store,
	})

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		mod.Routes(r)
	})

	// User A creates an intent
	rec := doRequest(
		router,
		http.MethodPost,
		"/api/v1/me/resources/upload-intent",
		`{"filename":"a.pdf","content_type":"application/pdf"}`,
		userA,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("intent creation: %d", rec.Code)
	}
	var intent struct {
		ID uuid.UUID `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &intent)

	path := fmt.Sprintf("/api/v1/me/resources/%s", intent.ID)

	// User B attempts to GET User A's resource -> 404
	recB := doRequest(router, http.MethodGet, path, "", userB)
	if recB.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unowned resource GET, got %d (body %s)", recB.Code, recB.Body)
	}

	// User B attempts to DELETE User A's resource -> 404
	recBDel := doRequest(router, http.MethodDelete, path, "", userB)
	if recBDel.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unowned resource DELETE, got %d (body %s)", recBDel.Code, recBDel.Body)
	}

	// User A still has their resource -> 200
	recA := doRequest(router, http.MethodGet, path, "", userA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner GET, got %d", recA.Code)
	}
}

// TestModule_DeleteResourceRemovesStorageObject verifies BR-RESOURCE-06 / D17-6:
// Deleting a resource deletes the object from storage as well as the database row.
func TestModule_DeleteResourceRemovesStorageObject(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	userA := insertUser(t, pool, "delete_store@example.com")
	store := newInMemoryStore()

	mod := resource.New(resource.Deps{
		Pool:    pool,
		Storage: store,
	})

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		mod.Routes(r)
	})

	// Intent + Put
	rec := doRequest(
		router,
		http.MethodPost,
		"/api/v1/me/resources/upload-intent",
		`{"filename":"del.pdf","content_type":"application/pdf"}`,
		userA,
	)
	var intent struct {
		ID        uuid.UUID `json:"id"`
		ObjectKey string    `json:"object_key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &intent)

	pdfBytes := []byte("%PDF-1.4\ntest")
	_ = store.Put(
		context.Background(),
		storage.BucketUploads,
		intent.ObjectKey,
		bytes.NewReader(pdfBytes),
		int64(len(pdfBytes)),
		"application/pdf",
	)
	if !store.hasObject(intent.ObjectKey) {
		t.Fatalf("object was not stored")
	}

	// Confirm
	rec = doRequest(router, http.MethodPost, "/api/v1/me/resources", fmt.Sprintf(`{"resource_id":"%s"}`, intent.ID), userA)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("confirm: %d", rec.Code)
	}

	// Delete
	path := fmt.Sprintf("/api/v1/me/resources/%s", intent.ID)
	recDel := doRequest(router, http.MethodDelete, path, "", userA)
	if recDel.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from delete, got %d", recDel.Code)
	}

	// Verify object was deleted from storage
	if store.hasObject(intent.ObjectKey) {
		t.Fatalf("storage object %s was not deleted after resource removal", intent.ObjectKey)
	}

	// Verify resource is gone from database
	recGet := doRequest(router, http.MethodGet, path, "", userA)
	if recGet.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after deletion, got %d", recGet.Code)
	}
}

// TestModule_SweeperExpiresPendingIntents verifies BR-RESOURCE-07:
// Abandoned intents older than PendingIntentTTL are marked failed and orphan objects cleaned up.
func TestModule_SweeperExpiresPendingIntents(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	userA := insertUser(t, pool, "sweeper@example.com")
	store := newInMemoryStore()

	mod := resource.New(resource.Deps{
		Pool:    pool,
		Storage: store,
	})

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		mod.Routes(r)
	})

	// Create an intent
	rec := doRequest(
		router,
		http.MethodPost,
		"/api/v1/me/resources/upload-intent",
		`{"filename":"stale.pdf","content_type":"application/pdf"}`,
		userA,
	)
	var intent struct {
		ID        uuid.UUID `json:"id"`
		ObjectKey string    `json:"object_key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &intent)

	// Simulate client uploaded bytes but never confirmed
	_ = store.Put(
		context.Background(),
		storage.BucketUploads,
		intent.ObjectKey,
		bytes.NewReader([]byte("bytes")),
		5,
		"application/pdf",
	)

	// Backdate the created_at timestamp to 30 minutes ago
	_, err := pool.Exec(context.Background(), `
		UPDATE resource.resources
		SET created_at = now() - interval '30 minutes'
		WHERE id = $1
	`, intent.ID)
	if err != nil {
		t.Fatalf("backdate intent: %v", err)
	}

	// Run sweeper job
	sweepJob := mod.SweepJob()
	if err := sweepJob.Task(context.Background()); err != nil {
		t.Fatalf("sweeper task: %v", err)
	}

	// Verify status is now 'failed'
	reader := mod.Reader()
	res, err := reader.GetResource(context.Background(), intent.ID, userA)
	if err != nil {
		t.Fatalf("get swept resource: %v", err)
	}
	if res.Status != domain.StatusFailed {
		t.Fatalf("expected status 'failed', got %q", res.Status)
	}

	// Verify orphan storage object was cleaned up
	if store.hasObject(intent.ObjectKey) {
		t.Fatalf("orphan storage object %s was not cleaned up by sweeper", intent.ObjectKey)
	}
}

// TestModule_ValidationRejectionDisagreement verifies BR-RESOURCE-04 / D17-4:
// Declaring image/png but uploading an executable or PDF is rejected with failure reason.
func TestModule_ValidationRejectionDisagreement(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	userA := insertUser(t, pool, "disagree@example.com")
	store := newInMemoryStore()

	mod := resource.New(resource.Deps{
		Pool:    pool,
		Storage: store,
	})

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		mod.Routes(r)
	})

	// Intent: declares image/png
	rec := doRequest(
		router,
		http.MethodPost,
		"/api/v1/me/resources/upload-intent",
		`{"filename":"pic.png","content_type":"image/png"}`,
		userA,
	)
	var intent struct {
		ID        uuid.UUID `json:"id"`
		ObjectKey string    `json:"object_key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &intent)

	// Put PDF bytes instead of PNG
	pdfBytes := []byte("%PDF-1.4\nfake image")
	_ = store.Put(
		context.Background(),
		storage.BucketUploads,
		intent.ObjectKey,
		bytes.NewReader(pdfBytes),
		int64(len(pdfBytes)),
		"image/png",
	)

	// Confirm
	rec = doRequest(router, http.MethodPost, "/api/v1/me/resources", fmt.Sprintf(`{"resource_id":"%s"}`, intent.ID), userA)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("confirm: %d", rec.Code)
	}

	// Run validation worker
	worker := mod.ValidateWorker()
	riverJob := &river.Job[resourcejob.ValidateResourceArgs]{
		Args: resourcejob.ValidateResourceArgs{ResourceID: intent.ID},
	}
	_ = worker.Work(context.Background(), riverJob)

	// Check status -> rejected
	reader := mod.Reader()
	res, err := reader.GetResource(context.Background(), intent.ID, userA)
	if err != nil {
		t.Fatalf("get rejected resource: %v", err)
	}
	if res.Status != domain.StatusRejected {
		t.Fatalf("expected status 'rejected', got %q", res.Status)
	}
	if res.FailureReason == "" {
		t.Fatalf("expected non-empty failure_reason for rejected resource")
	}
}

// TestModule_URLIntakeAndValidation verifies URL submission, metadata extraction, and storage of reference only.
func TestModule_URLIntakeAndValidation(t *testing.T) {
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	userA := insertUser(t, pool, "url_intake@example.com")
	store := newInMemoryStore()
	fetcher := &fakeURLFetcher{
		meta: &service.URLMetadata{
			FinalURL: "https://example.com/article",
			Title:    "Extracted Article Title",
		},
	}

	mod := resource.New(resource.Deps{
		Pool:       pool,
		Storage:    store,
		URLFetcher: fetcher,
	})

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		mod.Routes(r)
	})

	// Submit URL
	submitBody := `{"url":"https://example.com/article","title":"My Custom Title"}`
	rec := doRequest(router, http.MethodPost, "/api/v1/me/resources", submitBody, userA)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit url failed: code %d, body %s", rec.Code, rec.Body)
	}

	var submitResp struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &submitResp)

	// Run validation worker
	worker := mod.ValidateWorker()
	riverJob := &river.Job[resourcejob.ValidateResourceArgs]{
		Args: resourcejob.ValidateResourceArgs{ResourceID: submitResp.ID},
	}
	if err := worker.Work(context.Background(), riverJob); err != nil {
		t.Fatalf("url validation failed: %v", err)
	}

	// Verify validated state
	reader := mod.Reader()
	res, err := reader.GetResource(context.Background(), submitResp.ID, userA)
	if err != nil {
		t.Fatalf("get url resource: %v", err)
	}
	if res.Status != domain.StatusValidated {
		t.Fatalf("expected status 'validated', got %q", res.Status)
	}
	if res.Kind != domain.KindURL {
		t.Fatalf("expected kind 'url', got %q", res.Kind)
	}
	if res.ObjectKey != nil {
		t.Fatalf("URL resource must not have object_key")
	}
	if res.SourceURL == nil || *res.SourceURL != "https://example.com/article" {
		t.Fatalf("expected source_url https://example.com/article, got %v", res.SourceURL)
	}
}
