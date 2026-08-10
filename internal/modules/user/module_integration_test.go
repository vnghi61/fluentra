//go:build integration

package user_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/user"
	"github.com/fluentra/fluentra/internal/modules/user/contract"
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

// newModule builds the module over the real pool and mounts it the way the
// composition root will, then clears the tables so each test starts empty.
func newModule(t *testing.T) (*user.Module, http.Handler) {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	const reset = `TRUNCATE core.users CASCADE; TRUNCATE ops.outbox_events`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	module := user.New(user.Deps{Pool: pool})
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) { module.Routes(api) })
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
	}
	for _, testCase := range cases {
		recorder := request(t, router, testCase.method, testCase.path, uuid.Nil, testCase.body)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", testCase.method, testCase.path, recorder.Code)
		}
	}
}
