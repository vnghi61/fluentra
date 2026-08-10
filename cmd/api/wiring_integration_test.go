//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/fluentra/fluentra/db/migrations"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// This file is the WP1 gate (phase-1-plan.md §1.5): "a user row can be created,
// read, updated and role-checked through the API, and every write appears in
// `audit_logs`". It is the only place the three modules are exercised together,
// which is what P1.5 exists to make possible.

const wiringDatabase = "fluentra_wiring_test"

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, wiringDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", wiringDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", wiringDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", wiringDatabase, err)
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

// stack is the api and the worker, built the way the two binaries build them,
// over one database.
type stack struct {
	modules *identity
	router  http.Handler
	bus     *eventbus.InProcessBus
}

func newStack(t *testing.T) *stack {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	const reset = `TRUNCATE core.users CASCADE;
	               TRUNCATE ops.outbox_events;
	               TRUNCATE audit.audit_logs;
	               TRUNCATE audit.security_events`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	// No cache, so every permission check resolves from the database. The
	// cached path has its own tests in rbac; what this file is about is that
	// the three modules reach each other at all.
	modules := newIdentity(identityDeps{
		Pool:       pool,
		Env:        "test",
		OTPHMACKey: []byte("test-otp-hmac-key-at-least-32-bytes-long"),
	})

	// The worker half. Subscribing before anything publishes is the order
	// cmd/worker uses, and for the reason stated there.
	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	if err := modules.audit.Subscribe(bus); err != nil {
		t.Fatalf("subscribe the audit consumer: %v", err)
	}

	return &stack{
		modules: modules,
		router:  httpx.NewRouter(httpx.RouterDependencies{Modules: modules.Routes}),
		bus:     bus,
	}
}

// drainOutbox runs the publisher once, which is what the worker's polling loop
// does on every tick.
func (s *stack) drainOutbox(t *testing.T) {
	t.Helper()
	publisher := outbox.NewPublisher(pool, busDispatcher{bus: s.bus}, 50, time.Second)
	if err := publisher.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("process the outbox: %v", err)
	}
}

// busDispatcher mirrors cmd/worker's, forwarding outbox events onto the bus.
type busDispatcher struct{ bus *eventbus.InProcessBus }

func (d busDispatcher) Dispatch(ctx context.Context, event outbox.Event) error {
	return d.bus.Publish(ctx, eventbus.Message{
		ID: event.ID, Topic: event.Topic(), Payload: event.Payload, Attempt: event.Attempt,
	})
}

// tracedRequest issues a request inside a real recording span, so the trace id
// the modules read from the context is a real one rather than the zero value
// the no-op global provider hands out.
func (s *stack) tracedRequest(
	t *testing.T, method, path string, actor uuid.UUID, body string,
) (*httptest.ResponseRecorder, string) {
	t.Helper()

	provider := sdktrace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer("wiring-test").Start(context.Background(), method+" "+path)
	defer span.End()
	traceID := span.SpanContext().TraceID().String()

	var reader = http.NoBody
	request := httptest.NewRequest(method, path, reader)
	if body != "" {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	ctx = httpx.WithRequestID(ctx, "01KZGA1FXY6VAHQABK3EBKDN57")
	if actor != uuid.Nil {
		ctx = httpx.WithActor(ctx, httpx.Actor{UserID: actor, Role: roleOf(t, actor)})
	}

	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request.WithContext(ctx))
	return recorder, traceID
}

func roleOf(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	roles, err := s0.modules.rbac.RoleReader().RolesOf(context.Background(), userID)
	if err != nil {
		t.Fatalf("read roles: %v", err)
	}
	for _, role := range roles {
		if role.String() == "admin" {
			return "admin"
		}
	}
	return "user"
}

// s0 is the stack the current test is using. The actor's role has to be looked
// up while building a request, and threading the stack through every helper for
// one lookup made the tests harder to read than this does.
var s0 *stack

func newTestStack(t *testing.T) *stack {
	t.Helper()
	s0 = newStack(t)
	return s0
}

func createLearner(t *testing.T, stack *stack, email, name string) uuid.UUID {
	t.Helper()
	userID, err := stack.modules.user.Creator().CreateUser(context.Background(), usercontract.NewUser{
		Email: email, DisplayName: name, Locale: "en", Timezone: "Asia/Ho_Chi_Minh",
	})
	if err != nil {
		t.Fatalf("create %s: %v", email, err)
	}
	return userID
}

func grantAdmin(t *testing.T, userID uuid.UUID) {
	t.Helper()
	const insert = `
		INSERT INTO core.user_roles (user_id, role_id)
		SELECT $1, id FROM core.roles WHERE name = 'admin'
		ON CONFLICT DO NOTHING`
	if _, err := pool.Exec(context.Background(), insert, userID); err != nil {
		t.Fatalf("grant admin: %v", err)
	}
}

// TestProfileUpdateReachesTheAuditTrail is the P1.5 acceptance criterion, and
// with it the WP1 gate: a write through the API becomes a row in audit_logs
// with the right actor, without either module knowing the other exists.
func TestProfileUpdateReachesTheAuditTrail(t *testing.T) {
	stack := newTestStack(t)

	learner := createLearner(t, stack, "learner@fluentra.test", "Nghi")

	recorder, _ := stack.tracedRequest(t, http.MethodPatch, "/api/v1/me", learner,
		`{"display_name":"Nghi Nguyen"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PATCH /me = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	// Nothing is recorded until the worker runs. That is the design — the write
	// and its event commit together, and the trail catches up.
	if got := countAuditLogs(t); got != 0 {
		t.Fatalf("%d audit rows before the outbox was drained, want 0", got)
	}
	stack.drainOutbox(t)

	var (
		action    string
		actorID   *uuid.UUID
		targetID  *string
		fields    []string
		createdAt time.Time
	)
	const read = `SELECT action, actor_id, target_id, changed_fields, created_at FROM audit.audit_logs`
	if err := pool.QueryRow(context.Background(), read).
		Scan(&action, &actorID, &targetID, &fields, &createdAt); err != nil {
		t.Fatalf("read the audit entry: %v", err)
	}

	if action != "user.profile_updated" {
		t.Errorf("action = %q", action)
	}
	if actorID == nil || *actorID != learner {
		t.Errorf("actor_id = %v, want the learner %s", actorID, learner)
	}
	if targetID == nil || *targetID != learner.String() {
		t.Errorf("target_id = %v, want %s", targetID, learner)
	}
	if len(fields) != 1 || fields[0] != "display_name" {
		t.Errorf("changed_fields = %v, want [display_name]", fields)
	}
	if time.Since(createdAt) > time.Hour {
		t.Errorf("created_at = %s, which is not the time the change happened", createdAt)
	}

	// BR-AUDIT-04, against the real column: the new display name must not be
	// in the trail.
	var before, after []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT before, after FROM audit.audit_logs`).Scan(&before, &after); err != nil {
		t.Fatalf("read the diff: %v", err)
	}
	if strings.Contains(string(before)+string(after), "Nghi") {
		t.Errorf("the display name reached the trail: before=%s after=%s", before, after)
	}
}

// TestPreferenceReplacementIsAuditedToo checks the second write operation the
// user module serves, so the gate is "every write" rather than "the one write
// somebody remembered to test".
func TestPreferenceReplacementIsAuditedToo(t *testing.T) {
	stack := newTestStack(t)

	learner := createLearner(t, stack, "prefs@fluentra.test", "Prefs")

	body := `{"locale":"vi","theme":"dark","daily_goal_minutes":30,` +
		`"notification_channels":["in_app"],"quiet_hours":{"start":"22:00","end":"07:00"},` +
		`"ai_processing_opt_out":false}`
	recorder, _ := stack.tracedRequest(t, http.MethodPut, "/api/v1/me/preferences", learner, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT /me/preferences = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	stack.drainOutbox(t)
	if got := auditActions(t); len(got) != 1 || got[0] != "user.preferences_updated" {
		t.Errorf("audit actions = %v, want [user.preferences_updated]", got)
	}
}

// TestRoleChangeIsAudited closes the third side of WP1: a role check is only
// worth having if changing one leaves a record.
func TestRoleChangeIsAudited(t *testing.T) {
	stack := newTestStack(t)

	admin := createLearner(t, stack, "admin@fluentra.test", "Ops Lead")
	grantAdmin(t, admin)
	target := createLearner(t, stack, "target@fluentra.test", "Grantee")

	path := "/api/v1/admin/users/" + target.String() + "/roles"
	recorder, _ := stack.tracedRequest(t, http.MethodPost, path, admin, `{"role":"admin"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST %s = %d, want 200 (body %s)", path, recorder.Code, recorder.Body)
	}

	stack.drainOutbox(t)

	var action string
	var actorID *uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT action, actor_id FROM audit.audit_logs`).Scan(&action, &actorID); err != nil {
		t.Fatalf("read the audit entry: %v", err)
	}
	if action != "rbac.role_assigned" {
		t.Errorf("action = %q", action)
	}
	if actorID == nil || *actorID != admin {
		t.Errorf("actor_id = %v, want the granting admin %s", actorID, admin)
	}
}

// TestAuditSearchIsServedAndGuarded is what the anonymous routing test could
// not settle: that `/admin/audit-logs` reaches audit's handler rather than
// rbac's catch-all, and that the composition root's Group really does put it
// behind the role guard.
func TestAuditSearchIsServedAndGuarded(t *testing.T) {
	stack := newTestStack(t)

	admin := createLearner(t, stack, "reader@fluentra.test", "Trail Reader")
	grantAdmin(t, admin)
	learner := createLearner(t, stack, "nosy@fluentra.test", "Curious Learner")

	// A learner is refused — by AdminOnly, before audit's handler runs.
	refused, _ := stack.tracedRequest(t, http.MethodGet, "/api/v1/admin/audit-logs", learner, "")
	if refused.Code != http.StatusForbidden {
		t.Errorf("a learner got %d from /admin/audit-logs, want 403 (body %s)", refused.Code, refused.Body)
	}

	// An admin reaches the handler. A 404 here would mean rbac's `/admin`
	// mount had swallowed the path and audit was never wired in at all.
	served, _ := stack.tracedRequest(t, http.MethodGet, "/api/v1/admin/audit-logs", admin, "")
	if served.Code != http.StatusOK {
		t.Fatalf("an admin got %d from /admin/audit-logs, want 200 (body %s)", served.Code, served.Body)
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
		Page  struct {
			Limit int `json:"limit"`
		} `json:"page"`
	}
	if err := json.Unmarshal(served.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Page.Limit == 0 {
		t.Errorf("the response is not an audit page: %s", served.Body)
	}

	// And the security feed, which is served by the same group.
	events, _ := stack.tracedRequest(t, http.MethodGet, "/api/v1/admin/security-events", admin, "")
	if events.Code != http.StatusOK {
		t.Errorf("an admin got %d from /admin/security-events, want 200 (body %s)", events.Code, events.Body)
	}
}

// TestTheTrailRecordsTheRequestsTraceID is BR-AUDIT-07 end to end: an audit row
// links to the trace of the action, not to the trace of the worker that filed
// it.
//
// That distinction is the whole value of the field. The audit trail and the
// distributed trace answer different halves of the same question — "what
// changed" and "what happened" — and they are only worth joining if the id in
// the row is the one an operator can paste into Tempo and see the request.
//
// It works because `ops.outbox_events` carries the producing transaction's
// traceparent and the publisher restores it into the context it dispatches
// with. `audit` reads the trace from the context it is handed and did not
// change at all.
func TestTheTrailRecordsTheRequestsTraceID(t *testing.T) {
	stack := newTestStack(t)

	learner := createLearner(t, stack, "traced@fluentra.test", "Traced")
	recorder, requestTrace := stack.tracedRequest(t, http.MethodPatch, "/api/v1/me", learner,
		`{"display_name":"Traced Learner"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PATCH /me = %d (body %s)", recorder.Code, recorder.Body)
	}

	// The row records the request's traceparent at write time.
	var storedParent *string
	if err := pool.QueryRow(context.Background(),
		`SELECT traceparent FROM ops.outbox_events`).Scan(&storedParent); err != nil {
		t.Fatalf("read the outbox traceparent: %v", err)
	}
	if storedParent == nil || !strings.Contains(*storedParent, requestTrace) {
		t.Fatalf("outbox traceparent = %v, want it to carry the request's trace %s",
			storedParent, requestTrace)
	}

	// Drain in a *different* trace, which is what the worker really is: its
	// polling loop has nothing to do with the request that caused the event.
	provider := sdktrace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()
	ctx, span := provider.Tracer("wiring-test").Start(context.Background(), "worker.drain")
	workerTrace := span.SpanContext().TraceID().String()
	if workerTrace == requestTrace {
		t.Fatal("the worker span landed in the request's trace; the test cannot tell them apart")
	}

	publisher := outbox.NewPublisher(pool, busDispatcher{bus: stack.bus}, 50, time.Second)
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("process the outbox: %v", err)
	}
	span.End()

	var traceID *string
	if err := pool.QueryRow(context.Background(),
		`SELECT trace_id FROM audit.audit_logs`).Scan(&traceID); err != nil {
		t.Fatalf("read the trace id: %v", err)
	}
	if traceID == nil {
		t.Fatal("no trace_id was recorded, so the row links to no trace at all")
	}
	if _, err := trace.TraceIDFromHex(*traceID); err != nil {
		t.Errorf("trace_id = %q, which is not a W3C trace id: %v", *traceID, err)
	}
	if *traceID != requestTrace {
		t.Errorf("trace_id = %q, want the request's trace %q", *traceID, requestTrace)
	}
	if *traceID == workerTrace {
		t.Errorf("trace_id = %q, which is the worker's trace — the traceparent was not carried",
			*traceID)
	}
}

// TestAnEventWrittenOutsideASpanStillRecords. Not every write happens in a
// request: a seed, a job, a migration. Those have no trace, and the entry must
// still be filed rather than dropped.
func TestAnEventWrittenOutsideASpanStillRecords(t *testing.T) {
	stack := newTestStack(t)

	learner := createLearner(t, stack, "untraced@fluentra.test", "Untraced")

	// Written with a plain context: no span, so no traceparent.
	writer := outbox.NewWriter()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := writer.Write(context.Background(), tx, "user", "user.profile_updated",
		map[string]any{"user_id": learner, "changed_fields": []string{"timezone"}}); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatalf("write: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	stack.drainOutbox(t)

	var action string
	var traceID *string
	if err := pool.QueryRow(context.Background(),
		`SELECT action, trace_id FROM audit.audit_logs`).Scan(&action, &traceID); err != nil {
		t.Fatalf("read the audit entry: %v", err)
	}
	if action != "user.profile_updated" {
		t.Errorf("action = %q", action)
	}
	if traceID != nil {
		t.Errorf("trace_id = %q for an event written outside a span, want null rather than an invented one",
			*traceID)
	}
}

func countAuditLogs(t *testing.T) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM audit.audit_logs`).Scan(&count); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	return count
}

func auditActions(t *testing.T) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT action FROM audit.audit_logs ORDER BY created_at`)
	if err != nil {
		t.Fatalf("read audit actions: %v", err)
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatalf("scan: %v", err)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	return actions
}

// chiRouterIsUsed keeps the chi import honest — routePattern in main.go needs
// it, and the build-tagged file must not drift from that.
var _ = chi.NewRouter
