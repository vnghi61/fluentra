//go:build integration

package user_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/user"
	"github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/platform/storage"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// This file exercises the module as the composition root will: real pool, real
// repository, real outbox, real router. The layers each have their own tests;
// what only this level can check is the wiring between them — the two adapters
// in module.go that bridge interfaces the service declares to the packages that
// satisfy them. Those are the kind of glue that compiles and does nothing.

// moduleDatabase is this package's own database. The `ops` tables it asserts on
// are the same ones the outbox and worker suites truncate, so sharing the
// database TEST_DATABASE_URL names would make all three flaky.
const moduleDatabase = "fluentra_user_module_test"

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
		Method:      "POST",
		ObjectKey:   key,
		ExpiresAt:   time.Now().Add(expiry),
		MaxBytes:    maxBytes,
		ContentType: contentType,
	}, nil
}

func (s *inMemoryStore) VerifyUpload(
	_ context.Context, _, key, _ string, maxBytes int64,
) (storage.ObjectStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return storage.ObjectStat{}, storage.ErrObjectNotFound
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return storage.ObjectStat{}, storage.ErrSizeMismatch
	}
	sniffed := http.DetectContentType(data)
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		sniffed = "application/x-dosexec"
	}
	return storage.ObjectStat{
		Key:                key,
		Size:               int64(len(data)),
		SniffedContentType: sniffed,
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
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.objects[key] = data
	return nil
}

func (s *inMemoryStore) Delete(_ context.Context, _, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
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

func (s *inMemoryStore) Copy(_ context.Context, _, srcKey, _, destKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[srcKey]
	if !ok {
		return storage.ErrObjectNotFound
	}
	copied := make([]byte, len(data))
	copy(copied, data)
	s.objects[destKey] = copied
	return nil
}

func (s *inMemoryStore) PresignGet(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	return "http://storage.local/" + bucket + "/" + key, nil
}

func newTestStorage(t *testing.T) storage.Store {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		return newInMemoryStore()
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("TEST_S3_ACCESS_KEY"), os.Getenv("TEST_S3_SECRET_KEY"), ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	ctx := context.Background()
	for _, bucket := range []string{storage.BucketAvatars, storage.BucketExports} {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			t.Fatalf("bucket %s exists check: %v", bucket, err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				t.Fatalf("make bucket %s: %v", bucket, err)
			}
		}
	}
	return storage.NewMinIOStore(client)
}

// newModuleWithStorage builds the module over the real pool and test storage.
func newModuleWithStorage(t *testing.T) (*user.Module, http.Handler, storage.Store) {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	const reset = `TRUNCATE core.users CASCADE; TRUNCATE ops.outbox_events`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	store := newTestStorage(t)
	module := user.New(user.Deps{Pool: pool, Storage: store})
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) { module.Routes(api) })
	return module, router, store
}

// newModule builds the module over the real pool and mounts it the way the
// composition root will, then clears the tables so each test starts empty.
func newModule(t *testing.T) (*user.Module, http.Handler) {
	t.Helper()
	module, router, _ := newModuleWithStorage(t)
	return module, router
}

func request(
	t *testing.T, handler http.Handler, method, path string, actor uuid.UUID, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	ctx := httpx.WithRequestID(req.Context(), "01KZGA1FXY6VAHQABK3EBKDN57")
	if actor != uuid.Nil {
		ctx = httpx.WithActor(ctx, httpx.Actor{UserID: actor, Role: "user"})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req.WithContext(ctx))
	return recorder
}

// register creates an account through the module's own Creator contract, which
// is how `auth` will do it. It exercises the three-row transaction as a side
// effect of setting the test up.
func register(t *testing.T, module *user.Module, email, displayName string) uuid.UUID {
	t.Helper()
	id, err := module.Creator().CreateUser(context.Background(), contract.NewUser{
		Email: email, DisplayName: displayName, Locale: "en", Timezone: "Asia/Ho_Chi_Minh",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return id
}

func outboxRows(t *testing.T) []outboxRow {
	t.Helper()
	const query = `SELECT aggregate, event, payload FROM ops.outbox_events ORDER BY created_at`
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()

	var found []outboxRow
	for rows.Next() {
		var row outboxRow
		var payload []byte
		if err := rows.Scan(&row.Aggregate, &row.Event, &payload); err != nil {
			t.Fatalf("scan outbox row: %v", err)
		}
		if err := json.Unmarshal(payload, &row.Payload); err != nil {
			t.Fatalf("decode outbox payload: %v", err)
		}
		found = append(found, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate outbox: %v", err)
	}
	return found
}

type outboxRow struct {
	Aggregate string
	Event     string
	Payload   map[string]any
}

// Topic is what a consumer subscribes to, and what these tests assert against.
//
// The `event` column holds the bare name; the contract constant that other
// modules match on is the qualified one, and the two are joined here exactly as
// outbox.Event.Topic() joins them. Asserting the raw column against the
// constant is what these tests used to do, and it is why the doubled-topic bug
// went unnoticed until P1.5 wired a consumer.
func (r outboxRow) Topic() string { return r.Aggregate + "." + r.Event }

// TestModule_ProfileUpdateIsServedAndRecorded is the WP1 gate for this module,
// end to end: a real request changes a real row and leaves exactly one event
// for `audit` to consume. Every layer in the slice is the production one.
func TestModule_ProfileUpdateIsServedAndRecorded(t *testing.T) {
	module, router := newModule(t)
	actor := register(t, module, "learner@fluentra.test", "Nghi")

	recorder := request(t, router, http.MethodPatch, "/api/v1/me", actor,
		`{"display_name":"Nghi Nguyen","country":"VN"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	var stored string
	const readName = `SELECT display_name FROM core.profiles WHERE user_id = $1`
	if err := pool.QueryRow(context.Background(), readName, actor).Scan(&stored); err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if stored != "Nghi Nguyen" {
		t.Errorf("stored display name = %q, want the updated value", stored)
	}

	events := outboxRows(t)
	if len(events) != 1 {
		t.Fatalf("outbox has %d rows, want exactly 1: %+v", len(events), events)
	}
	if events[0].Aggregate != contract.Aggregate || events[0].Topic() != contract.EventProfileUpdated {
		t.Fatalf("aggregate/topic = %s/%s, want %s/%s",
			events[0].Aggregate, events[0].Topic(), contract.Aggregate, contract.EventProfileUpdated)
	}
	if events[0].Payload["user_id"] != actor.String() {
		t.Errorf("payload user_id = %v, want %s", events[0].Payload["user_id"], actor)
	}

	changed, ok := events[0].Payload["changed_fields"].([]any)
	if !ok || len(changed) != 2 {
		t.Fatalf("changed_fields = %v, want two field names", events[0].Payload["changed_fields"])
	}
	// The audit trail records which fields moved, never their values.
	for _, field := range changed {
		if field == "Nghi Nguyen" || field == "VN" {
			t.Errorf("the event payload carries a value, not just a field name: %v", field)
		}
	}
}

// TestModule_RejectedUpdateLeavesNothingBehind is the other half of rule L4: a
// request that fails validation must cost neither a row nor an event.
func TestModule_RejectedUpdateLeavesNothingBehind(t *testing.T) {
	module, router := newModule(t)
	actor := register(t, module, "reject@fluentra.test", "Nghi")

	recorder := request(t, router, http.MethodPatch, "/api/v1/me", actor,
		`{"display_name":"Fluentra Support"}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}

	var stored string
	const readName = `SELECT display_name FROM core.profiles WHERE user_id = $1`
	if err := pool.QueryRow(context.Background(), readName, actor).Scan(&stored); err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if stored != "Nghi" {
		t.Errorf("display name = %q, want it unchanged", stored)
	}
	if events := outboxRows(t); len(events) != 0 {
		t.Errorf("outbox has %d rows for a rejected request, want 0", len(events))
	}
}

func TestModule_PreferencesRoundTripThroughHTTP(t *testing.T) {
	module, router := newModule(t)
	actor := register(t, module, "prefs@fluentra.test", "Nghi")

	read := request(t, router, http.MethodGet, "/api/v1/me/preferences", actor, "")
	if read.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body %s)", read.Code, read.Body)
	}

	body := `{"locale":"vi","theme":"dark","daily_goal_minutes":30,` +
		`"notification_channels":["push","in_app"],` +
		`"quiet_hours":{"start":"22:00","end":"07:00"},"ai_processing_opt_out":true}`
	written := request(t, router, http.MethodPut, "/api/v1/me/preferences", actor, body)
	if written.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (body %s)", written.Code, written.Body)
	}

	var stored map[string]any
	if err := json.Unmarshal(written.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stored["locale"] != "vi" || stored["theme"] != "dark" || stored["ai_processing_opt_out"] != true {
		t.Errorf("response = %v, want the submitted values", stored)
	}
	// Channels come back in the declared order, not the order they were sent.
	channels, ok := stored["notification_channels"].([]any)
	if !ok || len(channels) != 2 || channels[0] != "in_app" || channels[1] != "push" {
		t.Errorf("channels = %v, want [in_app push]", stored["notification_channels"])
	}

	// Read it back through a second request: the PUT response and the stored
	// row must agree, which is what makes the operation idempotent.
	again := request(t, router, http.MethodGet, "/api/v1/me/preferences", actor, "")
	if again.Body.String() != written.Body.String() {
		t.Errorf("GET after PUT returned\n%s\nwant\n%s", again.Body, written.Body)
	}

	if events := outboxRows(t); len(events) != 1 || events[0].Topic() != contract.EventPreferencesUpdated {
		t.Errorf("outbox = %+v, want one %s", events, contract.EventPreferencesUpdated)
	}
}

// TestModule_ReaderBatchesAcrossRealRows checks the contract other modules will
// hold, against the real join rather than a fake.
func TestModule_ReaderBatchesAcrossRealRows(t *testing.T) {
	module, _ := newModule(t)

	first := register(t, module, "batch1@fluentra.test", "Learner One")
	second := register(t, module, "batch2@fluentra.test", "Learner Two")
	missing := uuid.New()

	summaries, err := module.Reader().GetManyByIDs(context.Background(),
		[]uuid.UUID{first, second, missing, first})
	if err != nil {
		t.Fatalf("GetManyByIDs: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("returned %d summaries, want 2", len(summaries))
	}
	if summaries[first].DisplayName != "Learner One" {
		t.Errorf("summary = %+v, want the registered profile", summaries[first])
	}
	if summaries[first].Timezone != "Asia/Ho_Chi_Minh" || summaries[first].Locale != "en" {
		t.Errorf("summary = %+v, want the joined profile and preference values", summaries[first])
	}
	if _, present := summaries[missing]; present {
		t.Error("an id with no row appeared in the result")
	}
}

// TestModule_RegistrationIsAtomic proves the adapter that gives the service a
// transactional repository actually does: a duplicate email must leave no
// half-built account behind.
func TestModule_RegistrationIsAtomic(t *testing.T) {
	module, _ := newModule(t)
	register(t, module, "taken@fluentra.test", "First")

	_, err := module.Creator().CreateUser(context.Background(), contract.NewUser{
		Email: "TAKEN@fluentra.test", DisplayName: "Second", Locale: "en", Timezone: "UTC",
	})
	if err == nil {
		t.Fatal("the second registration with the same email succeeded")
	}

	var users int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM core.users`).Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if users != 1 {
		t.Errorf("core.users has %d rows, want 1", users)
	}
	var profiles int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM core.profiles`).Scan(&profiles); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if profiles != 1 {
		t.Errorf("core.profiles has %d rows, want 1: the failed registration left one behind", profiles)
	}
}

// TestModule_UnauthenticatedRequestsAreRefused checks the one thing that must
// hold for every operation once this is mounted for real.
func TestModule_UnauthenticatedRequestsAreRefused(t *testing.T) {
	_, router := newModule(t)

	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/me", ""},
		{http.MethodPatch, "/api/v1/me", `{"display_name":"Nghi"}`},
		{http.MethodGet, "/api/v1/me/preferences", ""},
		{http.MethodPut, "/api/v1/me/preferences", `{}`},
		{http.MethodPost, "/api/v1/me/avatar/upload-intent", `{"content_type":"image/jpeg"}`},
		{http.MethodPut, "/api/v1/me/avatar", `{"object_key":"raw.jpg"}`},
	}
	for _, testCase := range cases {
		recorder := request(t, router, testCase.method, testCase.path, uuid.Nil, testCase.body)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", testCase.method, testCase.path, recorder.Code)
		}
	}
}

func createTestJPEGImage() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := range 64 {
		for y := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 3), B: 150, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

func TestModule_AvatarUploadLifecycle(t *testing.T) {
	module, router, store := newModuleWithStorage(t)
	actorID := register(t, module, "avatar-learner@fluentra.test", "Avatar Learner")

	// 1. Request upload intent.
	intentRecorder := request(
		t, router, http.MethodPost, "/api/v1/me/avatar/upload-intent", actorID, `{"content_type":"image/jpeg"}`,
	)
	if intentRecorder.Code != http.StatusOK {
		t.Fatalf("POST upload-intent: %d, body %s", intentRecorder.Code, intentRecorder.Body)
	}

	var intent struct {
		UploadURL string `json:"upload_url"`
		Method    string `json:"method"`
		ObjectKey string `json:"object_key"`
		MaxBytes  int64  `json:"max_bytes"`
	}
	if err := json.Unmarshal(intentRecorder.Body.Bytes(), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}

	// 2. Upload valid JPEG image to storage at intent.ObjectKey.
	jpegData := createTestJPEGImage()
	if err := store.Put(
		context.Background(), storage.BucketAvatars, intent.ObjectKey,
		bytes.NewReader(jpegData), int64(len(jpegData)), "image/jpeg",
	); err != nil {
		t.Fatalf("store put raw: %v", err)
	}

	// 3. Confirm avatar.
	confirmBody := fmt.Sprintf(`{"object_key":%q}`, intent.ObjectKey)
	confirmRecorder := request(t, router, http.MethodPut, "/api/v1/me/avatar", actorID, confirmBody)
	if confirmRecorder.Code != http.StatusOK {
		t.Fatalf("PUT avatar confirm: %d, body %s", confirmRecorder.Code, confirmRecorder.Body)
	}

	var me struct {
		Profile struct {
			AvatarURL *string `json:"avatar_url"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(confirmRecorder.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if me.Profile.AvatarURL == nil || !strings.HasPrefix(*me.Profile.AvatarURL, "/api/v1/storage/avatars/") {
		t.Errorf("avatar_url = %v, want /api/v1/storage/avatars/...", me.Profile.AvatarURL)
	}

	// 4. Verify outbox event written.
	rows := outboxRows(t)
	if len(rows) != 1 {
		t.Fatalf("outbox has %d rows, want 1", len(rows))
	}
	if rows[0].Topic() != contract.EventProfileUpdated {
		t.Errorf("event topic = %q, want %q", rows[0].Topic(), contract.EventProfileUpdated)
	}
}

func TestModule_AvatarUploadRejectsRenamedExecutable(t *testing.T) {
	module, router, store := newModuleWithStorage(t)
	actorID := register(t, module, "malicious@fluentra.test", "Attacker")

	intentRecorder := request(
		t, router, http.MethodPost, "/api/v1/me/avatar/upload-intent", actorID, `{"content_type":"image/jpeg"}`,
	)
	if intentRecorder.Code != http.StatusOK {
		t.Fatalf("POST upload-intent: %d", intentRecorder.Code)
	}

	var intent struct {
		ObjectKey string `json:"object_key"`
	}
	if err := json.Unmarshal(intentRecorder.Body.Bytes(), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}

	// Put executable bytes disguised as jpg into storage.
	fakeExe := []byte("MZ\x90\x00\x03\x00\x00\x00malicious exe payload")
	if err := store.Put(
		context.Background(), storage.BucketAvatars, intent.ObjectKey,
		bytes.NewReader(fakeExe), int64(len(fakeExe)), "image/jpeg",
	); err != nil {
		t.Fatalf("put fake exe: %v", err)
	}

	// Attempt confirmation.
	confirmBody := fmt.Sprintf(`{"object_key":%q}`, intent.ObjectKey)
	confirmRecorder := request(t, router, http.MethodPut, "/api/v1/me/avatar", actorID, confirmBody)
	if confirmRecorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("confirm fake exe status = %d, want 422", confirmRecorder.Code)
	}

	// Database avatar should remain NULL.
	var avatarID *uuid.UUID
	const readAvatar = `SELECT avatar_asset_id FROM core.profiles WHERE user_id = $1`
	if err := pool.QueryRow(context.Background(), readAvatar, actorID).Scan(&avatarID); err != nil {
		t.Fatalf("query avatar_asset_id: %v", err)
	}
	if avatarID != nil {
		t.Errorf("avatar_asset_id was updated to %v, want NULL", avatarID)
	}
}

// TestModule_IDOR_UserBSeesUserARowsAsNotFound is the P5.5 IDOR suite, against
// real SQL rather than a fake.
//
// The service-level suite in service/idor_test.go drives an in-memory
// repository, which cannot tell whether the query it stands in for carries its
// `AND user_id = $2`. Dropping that clause from the real statement is exactly
// the mistake worth catching, and only a test that reaches PostgreSQL can catch
// it — so this one owns two accounts and asks for each other's rows.
//
// 404 and not 403 throughout: a 403 confirms the row exists, which is the whole
// of what an enumeration attempt is trying to learn.
func TestModule_IDOR_UserBSeesUserARowsAsNotFound(t *testing.T) {
	module, router := newModule(t)

	owner := register(t, module, "idor-owner@example.com", "Owner")
	stranger := register(t, module, "idor-stranger@example.com", "Stranger")

	// An export and a deletion, both addressed by their own id in the URL, which
	// is what makes them reachable by guessing at all. /me and /me/preferences
	// are deliberately not here: they carry no id, so the actor is the address.
	exportResponse := request(t, router, http.MethodPost, "/api/v1/me/export", owner, "")
	if exportResponse.Code != http.StatusAccepted && exportResponse.Code != http.StatusCreated {
		t.Fatalf("create export: status %d body %s", exportResponse.Code, exportResponse.Body.String())
	}
	var export struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(exportResponse.Body.Bytes(), &export); err != nil {
		t.Fatalf("decode export: %v", err)
	}

	// Requesting deletion is DELETE /me, not POST /me/deletion.
	deletionResponse := request(t, router, http.MethodDelete, "/api/v1/me", owner, "")
	if deletionResponse.Code != http.StatusAccepted && deletionResponse.Code != http.StatusCreated {
		t.Fatalf("create deletion: status %d body %s", deletionResponse.Code, deletionResponse.Body.String())
	}
	var deletion struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(deletionResponse.Body.Bytes(), &deletion); err != nil {
		t.Fatalf("decode deletion: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"another learner's export", "/api/v1/me/export/" + export.ID},
		{"another learner's deletion", "/api/v1/me/deletion/" + deletion.ID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// The owner can read it: without this the 404 below would pass just
			// as well against a path that does not resolve for anybody.
			if owned := request(t, router, http.MethodGet, testCase.path, owner, ""); owned.Code != http.StatusOK {
				t.Fatalf("owner GET %s = %d, want 200 — the 404 below would prove nothing",
					testCase.path, owned.Code)
			}

			stranged := request(t, router, http.MethodGet, testCase.path, stranger, "")
			if stranged.Code != http.StatusNotFound {
				t.Errorf("stranger GET %s = %d, want 404 (a 403 would confirm the row exists)",
					testCase.path, stranged.Code)
			}
		})
	}
}
