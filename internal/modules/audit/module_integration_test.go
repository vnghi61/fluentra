//go:build integration

package audit_test

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

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/audit"
	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// moduleDatabase is this package's own, for the same reason every other suite
// has one: the outbox and worker tests truncate shared `ops` tables and assert
// exact counts, and this file writes to them.
const moduleDatabase = "fluentra_audit_module_test"

// Literals these tests reach for repeatedly.
const (
	roleAdmin         = "admin"
	fieldDisplayName  = "display_name"
	fieldTimezone     = "timezone"
	permissionSuspend = "user.suspend"
)

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

// allowAll is the guard stand-in. Authorization has its own tests in the
// transport package against a fake that refuses; what this file is for is the
// SQL, the grants and the partitions.
type allowAll struct{}

func (allowAll) Require(context.Context, string) error { return nil }

func newModule(t *testing.T) (*audit.Module, http.Handler) {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	// audit_logs holds no DELETE grant even for the migration owner's default
	// privileges path, but this suite connects as the superuser, which may
	// truncate. That is the point of resetting here rather than deleting.
	const reset = `TRUNCATE audit.audit_logs; TRUNCATE audit.security_events; TRUNCATE ops.outbox_events`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	module := audit.New(audit.Deps{
		Pool:      pool,
		Guard:     allowAll{},
		IPHashKey: []byte("integration-test-key"),
	})
	router := chi.NewRouter()
	router.Route("/api/v1", module.Routes)
	return module, router
}

func request(
	t *testing.T, handler http.Handler, method, path string, actor uuid.UUID, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader = http.NoBody
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	ctx := httpx.WithRequestID(req.Context(), "01KZGA1FXY6VAHQABK3EBKDN57")
	if actor != uuid.Nil {
		ctx = httpx.WithActor(ctx, httpx.Actor{UserID: actor, Role: roleAdmin})
	}
	handler.ServeHTTP(recorder, req.WithContext(ctx))
	return recorder
}

// asApplicationRole runs statements on one connection with SET LOCAL ROLE
// fluentra_app, so privileges are checked as the deployed application and not
// as the superuser this suite connects with.
func asApplicationRole(t *testing.T, statements ...string) []error {
	t.Helper()
	ctx := context.Background()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	results := make([]error, 0, len(statements))
	for _, statement := range statements {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE fluentra_app"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("assume fluentra_app: %v", err)
		}
		_, execErr := tx.Exec(ctx, statement)
		results = append(results, execErr)
		if execErr != nil {
			_ = tx.Rollback(ctx)
			continue
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	return results
}

// TestApplicationRoleCannotUpdateOrDeleteAuditLogs is the acceptance criterion
// of P1.4, and the reason BR-AUDIT-01 is a grant rather than a convention.
//
// It matters that this is checked against the real role. The application never
// issues an UPDATE against this table, so a test that only exercised the Go
// code would pass on a database where the grant had quietly been restored —
// and the bootstrap migration's ALTER DEFAULT PRIVILEGES does hand out all
// four privileges, so "quietly restored" is the default state, not a
// hypothetical.
func TestApplicationRoleCannotUpdateOrDeleteAuditLogs(t *testing.T) {
	newModule(t)

	insert := fmt.Sprintf(
		`INSERT INTO audit.audit_logs (event_id, action, created_at) VALUES (%s, 'user.profile_updated', now())`,
		"gen_random_uuid()")

	results := asApplicationRole(t,
		insert,
		`UPDATE audit.audit_logs SET action = 'tampered.by_hand'`,
		`DELETE FROM audit.audit_logs`,
		`TRUNCATE audit.audit_logs`,
	)

	if results[0] != nil {
		t.Fatalf("the application role could not INSERT, which it must be able to do: %v", results[0])
	}
	for index, operation := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
		if results[index+1] == nil {
			t.Errorf("the application role was allowed to %s audit_logs", operation)
			continue
		}
		if !strings.Contains(results[index+1].Error(), "permission denied") {
			t.Errorf("%s failed for the wrong reason: %v", operation, results[index+1])
		}
	}

	var surviving int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit.audit_logs`).Scan(&surviving); err != nil {
		t.Fatalf("count: %v", err)
	}
	if surviving != 1 {
		t.Errorf("%d rows survived, want the 1 that was inserted", surviving)
	}
}

// TestPartitionsAreAppendOnlyToo closes the hole the parent-table grant leaves.
// Privileges are checked on the relation named in the statement, so a caller
// naming a partition directly bypasses a grant that only covers the parent.
func TestPartitionsAreAppendOnlyToo(t *testing.T) {
	newModule(t)

	partition := currentPartition(t, "audit_logs")
	results := asApplicationRole(t,
		fmt.Sprintf(`UPDATE audit.%s SET action = 'tampered.by_hand'`, partition),
		fmt.Sprintf(`DELETE FROM audit.%s`, partition),
	)
	for index, operation := range []string{"UPDATE", "DELETE"} {
		if results[index] == nil {
			t.Errorf("the application role was allowed to %s the partition %s directly", operation, partition)
		}
	}
}

// TestNewPartitionsInheritTheRestrictedGrants is the one that would have caught
// the bug this design exists to avoid: a partition created next month by the
// rotation job is created by the migration role, so the bootstrap's default
// privileges hand it UPDATE and DELETE. Append-only would last exactly until
// the month rolled over.
func TestNewPartitionsInheritTheRestrictedGrants(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	if err := module.RotatePartitions(ctx); err != nil {
		t.Fatalf("RotatePartitions: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT c.relname,
		       has_table_privilege('fluentra_app', c.oid, 'UPDATE'),
		       has_table_privilege('fluentra_app', c.oid, 'DELETE'),
		       has_table_privilege('fluentra_app', c.oid, 'INSERT'),
		       has_table_privilege('fluentra_app', c.oid, 'SELECT')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'audit' AND c.relname ~ '^audit_logs_y\d{4}m\d{2}$'
		ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("read partition grants: %v", err)
	}
	defer rows.Close()

	checked := 0
	for rows.Next() {
		var name string
		var canUpdate, canDelete, canInsert, canSelect bool
		if err := rows.Scan(&name, &canUpdate, &canDelete, &canInsert, &canSelect); err != nil {
			t.Fatalf("scan: %v", err)
		}
		checked++
		if canUpdate || canDelete {
			t.Errorf("%s grants the application role update=%v delete=%v", name, canUpdate, canDelete)
		}
		if !canInsert || !canSelect {
			t.Errorf("%s does not grant insert=%v select=%v, so the trail cannot be written or read",
				name, canInsert, canSelect)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	// The migration creates the current month plus three; rotation keeps that.
	if checked < 4 {
		t.Errorf("checked %d partitions, want at least the current month plus three ahead", checked)
	}
}

// TestSecurityEventsTakeUpdateButNotDelete: resolving an event is the whole
// point of that table, and making an event disappear is not.
func TestSecurityEventsTakeUpdateButNotDelete(t *testing.T) {
	newModule(t)

	results := asApplicationRole(t,
		`INSERT INTO audit.security_events (event_id, kind, severity, created_at)
		 VALUES (gen_random_uuid(), 'rbac.access_denied', 'medium', now())`,
		`UPDATE audit.security_events SET resolved_at = now(), resolved_by = gen_random_uuid(),
		     resolution_note = 'triaged'`,
		`DELETE FROM audit.security_events`,
	)
	if results[0] != nil {
		t.Fatalf("INSERT was refused: %v", results[0])
	}
	if results[1] != nil {
		t.Errorf("UPDATE was refused, so an event could never be resolved: %v", results[1])
	}
	if results[2] == nil {
		t.Error("the application role was allowed to DELETE a security event")
	}
}

// TestDuplicateEventProducesOneRow is the acceptance criterion for
// idempotency, against the real unique index rather than a fake that models it.
func TestDuplicateEventProducesOneRow(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	if err := module.Subscribe(bus); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	eventID, err := id.NewUUIDv7(ctx)
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"user_id":        uuid.New(),
		"actor_id":       uuid.New(),
		"changed_fields": []string{fieldDisplayName},
		"occurred_at":    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	message := eventbus.Message{ID: eventID, Topic: "user.profile_updated", Payload: payload}

	for attempt := range 3 {
		if err := bus.Publish(ctx, message); err != nil {
			t.Fatalf("delivery %d: %v", attempt+1, err)
		}
	}

	if got := countLogs(t); got != 1 {
		t.Errorf("three deliveries of one event produced %d rows, want 1", got)
	}
}

// TestOutboxEventBecomesAnAuditEntry is the WP1 gate, end to end: a business
// write commits an event in its own transaction, the publisher picks it up, and
// the trail has a row for it.
//
// The event is written by hand rather than by calling the `user` module,
// because `audit` may not import it — which is also exactly what the consumer
// has to cope with in production, so writing the payload here is testing the
// real contract rather than working around a boundary.
func TestOutboxEventBecomesAnAuditEntry(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	if err := module.Subscribe(bus); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	actorID := uuid.New()
	targetID := uuid.New()
	writer := outbox.NewWriter()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	eventID, err := writer.Write(ctx, tx, "user", "profile_updated", map[string]any{
		"user_id":        targetID,
		"actor_id":       actorID,
		"changed_fields": []string{fieldDisplayName, fieldTimezone},
		"occurred_at":    time.Now().UTC(),
	})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("write outbox event: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	publisher := outbox.NewPublisher(pool, dispatcher{bus: bus}, 10, time.Second)
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}

	assertEntryMatchesEvent(t, eventID, actorID, targetID)
	assertOutboxRowPublished(t, eventID)
}

// assertEntryMatchesEvent reads the single row the consumer wrote and checks it
// against the event that produced it.
func assertEntryMatchesEvent(t *testing.T, eventID, actorID, targetID uuid.UUID) {
	t.Helper()

	var (
		storedAction  string
		storedActor   *uuid.UUID
		storedTarget  *string
		storedFields  []string
		storedEventID uuid.UUID
	)
	const read = `
		SELECT action, actor_id, target_id, changed_fields, event_id
		FROM audit.audit_logs`
	if err := pool.QueryRow(context.Background(), read).Scan(
		&storedAction, &storedActor, &storedTarget, &storedFields, &storedEventID,
	); err != nil {
		t.Fatalf("read the audit entry: %v", err)
	}

	if storedAction != "user.profile_updated" {
		t.Errorf("action = %q, want the event topic", storedAction)
	}
	if storedActor == nil || *storedActor != actorID {
		t.Errorf("actor_id = %v, want %s", storedActor, actorID)
	}
	if storedTarget == nil || *storedTarget != targetID.String() {
		t.Errorf("target_id = %v, want %s", storedTarget, targetID)
	}
	if len(storedFields) != 2 {
		t.Errorf("changed_fields = %v, want both names", storedFields)
	}
	if storedEventID != eventID {
		t.Errorf("event_id = %s, want the outbox event id %s", storedEventID, eventID)
	}
}

// assertOutboxRowPublished checks the event is marked done, so the publisher
// does not deliver it forever.
func assertOutboxRowPublished(t *testing.T, eventID uuid.UUID) {
	t.Helper()

	var published *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT published_at FROM ops.outbox_events WHERE event_id = $1`, eventID).Scan(&published); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	if published == nil {
		t.Error("the outbox event was not marked published after the consumer accepted it")
	}
}

// TestRecordedValuesAreRedactedInTheTable is BR-AUDIT-04 against real columns.
// The Go-level test proves the function redacts; this proves nothing between
// the function and the disk puts the value back.
func TestRecordedValuesAreRedactedInTheTable(t *testing.T) {
	module, _ := newModule(t)
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: uuid.New(), Role: roleAdmin})

	module.Recorder().Record(ctx, contract.Entry{
		Action:     "user.updated_profile",
		TargetType: "user",
		TargetID:   uuid.New().String(),
		Before:     map[string]any{fieldDisplayName: "Nghi", fieldTimezone: "UTC"},
		After:      map[string]any{fieldDisplayName: "Nghi Nguyen", fieldTimezone: "Asia/Ho_Chi_Minh"},
		Meta:       map[string]any{"email": "learner@example.com", "reason": "support request"},
	})

	var before, after, meta []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT before, after, meta FROM audit.audit_logs`).Scan(&before, &after, &meta); err != nil {
		t.Fatalf("read the entry: %v", err)
	}
	stored := string(before) + string(after) + string(meta)
	for _, leaked := range []string{"Nghi", "Nghi Nguyen", "learner@example.com"} {
		if strings.Contains(stored, leaked) {
			t.Errorf("%q reached the table: %s", leaked, stored)
		}
	}
	if !strings.Contains(string(after), "Asia/Ho_Chi_Minh") {
		t.Errorf("the timezone was redacted too, and it is not personal data: %s", after)
	}
	if !strings.Contains(string(meta), "support request") {
		t.Errorf("the stated reason was lost: %s", meta)
	}
}

// TestRotationIsIdempotent: the cron scheduler's advisory lock can be lost, and
// two workers may run this at once. Neither may fail.
func TestRotationIsIdempotent(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	for range 3 {
		if err := module.RotatePartitions(ctx); err != nil {
			t.Fatalf("RotatePartitions: %v", err)
		}
	}

	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'audit' AND c.relname ~ '^audit_logs_y\d{4}m\d{2}$'`).Scan(&count); err != nil {
		t.Fatalf("count partitions: %v", err)
	}
	if count != 4 {
		t.Errorf("%d partitions after three rotations, want the current month plus three", count)
	}
}

// TestRetentionDetachesOnlyWhatHasExpired drives the retention function against
// a partition planted in the past, and asserts it leaves the live ones alone.
func TestRetentionDetachesOnlyWhatHasExpired(t *testing.T) {
	newModule(t)
	ctx := context.Background()

	// A partition for a month three years ago, made the way the rotation
	// function makes them so the names match what retention parses.
	const plant = `
		CREATE TABLE IF NOT EXISTS audit.audit_logs_y2023m01
		PARTITION OF audit.audit_logs
		FOR VALUES FROM ('2023-01-01 00:00:00+00') TO ('2023-02-01 00:00:00+00')`
	if _, err := pool.Exec(ctx, plant); err != nil {
		t.Fatalf("plant an expired partition: %v", err)
	}

	var detached []string
	if err := pool.QueryRow(ctx,
		`SELECT audit.detach_expired_partitions(interval '2 years')`).Scan(&detached); err != nil {
		t.Fatalf("detach_expired_partitions: %v", err)
	}

	if len(detached) != 1 || detached[0] != "audit_logs_y2023m01" {
		t.Fatalf("detached = %v, want exactly the expired partition", detached)
	}

	// It is out of the tree...
	var stillAttached bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_inherits i
			JOIN pg_class c ON c.oid = i.inhrelid
			WHERE c.relname = 'audit_logs_y2023m01')`).Scan(&stillAttached); err != nil {
		t.Fatalf("check attachment: %v", err)
	}
	if stillAttached {
		t.Error("the expired partition is still attached")
	}

	// ...but not destroyed: detaching is reversible, dropping is not, and the
	// archival step has not run.
	var stillExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'audit' AND c.relname = 'audit_logs_y2023m01')`).Scan(&stillExists); err != nil {
		t.Fatalf("check existence: %v", err)
	}
	if !stillExists {
		t.Error("the expired partition was dropped; retention detaches so it can be archived")
	}

	// And the live partitions are untouched.
	var live int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'audit_logs'`).Scan(&live); err != nil {
		t.Fatalf("count live partitions: %v", err)
	}
	if live != 4 {
		t.Errorf("%d live partitions remain, want the current month plus three", live)
	}

	if _, err := pool.Exec(ctx, `DROP TABLE audit.audit_logs_y2023m01`); err != nil {
		t.Fatalf("clean up the planted partition: %v", err)
	}
}

// TestSearchReadsBackWhatWasRecorded is the admin surface over real rows,
// including the window default that keeps the query inside a bounded set of
// partitions.
func TestSearchReadsBackWhatWasRecorded(t *testing.T) {
	module, router := newModule(t)

	admin := uuid.New()
	target := uuid.New()
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: admin, Role: roleAdmin})
	module.Recorder().Record(ctx, contract.Entry{
		Action: "user.read_profile", TargetType: "user", TargetID: target.String(),
	})

	recorder := request(t, router, http.MethodGet,
		"/api/v1/admin/audit-logs?target_type=user&target_id="+target.String(), admin, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	var page struct {
		Items []struct {
			Action   string `json:"action"`
			ActorID  string `json:"actor_id"`
			TargetID string `json:"target_id"`
		} `json:"items"`
		Page struct {
			HasMore bool `json:"has_more"`
			Limit   int  `json:"limit"`
		} `json:"page"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1 (body %s)", len(page.Items), recorder.Body)
	}
	if page.Items[0].Action != "user.read_profile" {
		t.Errorf("action = %q", page.Items[0].Action)
	}
	if page.Items[0].ActorID != admin.String() {
		t.Errorf("actor_id = %q, want the caller %s", page.Items[0].ActorID, admin)
	}
	if page.Page.HasMore {
		t.Error("has_more = true with one row")
	}
}

// TestSecurityEventFeedFiltersTheOpenQueue reads the triage feed over real
// rows, including the `resolved=false` filter the dashboard opens with.
func TestSecurityEventFeedFiltersTheOpenQueue(t *testing.T) {
	module, router := newModule(t)

	admin := uuid.New()
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: admin, Role: roleAdmin})
	for _, severity := range []contract.Severity{contract.SeverityLow, contract.SeverityCritical} {
		module.SecurityRecorder().RecordSecurityEvent(ctx, contract.SecurityEvent{
			Kind: "rbac.access_denied", Severity: severity,
			Detail: map[string]any{"permission": permissionSuspend},
		})
	}

	recorder := request(t, router, http.MethodGet,
		"/api/v1/admin/security-events?resolved=false&severity=critical", admin, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	var page struct {
		Items []struct {
			Kind       string         `json:"kind"`
			Severity   string         `json:"severity"`
			Detail     map[string]any `json:"detail"`
			ResolvedAt *string        `json:"resolved_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want only the critical one (body %s)", len(page.Items), recorder.Body)
	}
	if page.Items[0].Severity != "critical" || page.Items[0].ResolvedAt != nil {
		t.Errorf("item = %+v, want an open critical event", page.Items[0])
	}
	if page.Items[0].Detail["permission"] != permissionSuspend {
		t.Errorf("detail = %v, want the permission that was refused", page.Items[0].Detail)
	}
}

// TestResolvingASecurityEventIsRecordedAndThenRefused drives the triage flow
// over real rows: the update lands, and the second attempt is a conflict.
func TestResolvingASecurityEventIsRecordedAndThenRefused(t *testing.T) {
	module, router := newModule(t)

	admin := uuid.New()
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: admin, Role: roleAdmin})
	module.SecurityRecorder().RecordSecurityEvent(ctx, contract.SecurityEvent{
		Kind: "rbac.access_denied", Severity: contract.SeverityMedium,
		Detail: map[string]any{"permission": permissionSuspend},
	})

	var eventID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM audit.security_events`).Scan(&eventID); err != nil {
		t.Fatalf("read the event: %v", err)
	}

	path := "/api/v1/admin/security-events/" + eventID.String() + "/resolve"
	first := request(t, router, http.MethodPost, path, admin, `{"note":"Known load test."}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first resolve = %d (body %s)", first.Code, first.Body)
	}

	second := request(t, router, http.MethodPost, path, admin, `{"note":"A second opinion."}`)
	if second.Code != http.StatusConflict {
		t.Errorf("second resolve = %d, want 409 (body %s)", second.Code, second.Body)
	}

	var storedNote string
	var resolvedBy uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT resolution_note, resolved_by FROM audit.security_events`).Scan(&storedNote, &resolvedBy); err != nil {
		t.Fatalf("read the resolution: %v", err)
	}
	if storedNote != "Known load test." {
		t.Errorf("note = %q, want the first explanation kept", storedNote)
	}
	if resolvedBy != admin {
		t.Errorf("resolved_by = %s, want %s", resolvedBy, admin)
	}
}

// TestCronJobsAreDistinctlyLocked guards against two scheduled jobs sharing an
// advisory lock id, whose symptom is a job that "sometimes does not run".
func TestCronJobsAreDistinctlyLocked(t *testing.T) {
	module, _ := newModule(t)

	jobs := module.CronJobs()
	if len(jobs) != 2 {
		t.Fatalf("CronJobs = %d, want rotation and retention", len(jobs))
	}
	if jobs[0].LockID == jobs[1].LockID {
		t.Errorf("both jobs use lock id %d", jobs[0].LockID)
	}
	for _, scheduled := range jobs {
		if scheduled.Interval <= 0 || scheduled.Task == nil || scheduled.Name == "" {
			t.Errorf("job %+v is not runnable", scheduled)
		}
	}
}

// dispatcher forwards outbox events onto the bus, exactly as cmd/worker does.
type dispatcher struct{ bus *eventbus.InProcessBus }

func (d dispatcher) Dispatch(ctx context.Context, event outbox.Event) error {
	return d.bus.Publish(ctx, eventbus.Message{
		ID:      event.ID,
		Topic:   event.Topic(),
		Payload: event.Payload,
		Attempt: event.Attempt,
	})
}

func countLogs(t *testing.T) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM audit.audit_logs`).Scan(&count); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	return count
}

func currentPartition(t *testing.T, parent string) string {
	t.Helper()
	now := time.Now().UTC()
	return fmt.Sprintf("%s_y%04dm%02d", parent, now.Year(), int(now.Month()))
}
