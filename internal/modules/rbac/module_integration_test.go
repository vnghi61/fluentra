//go:build integration

package rbac_test

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
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/fluentra/fluentra/db/migrations"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
)

// moduleDatabase is this package's own, for the same reason every other suite
// has one: the outbox and worker tests truncate shared `ops` tables and assert
// exact counts, and this file writes to them.
const moduleDatabase = "fluentra_rbac_module_test"

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

// newModule builds the module over the real pool with no cache, so every check
// is resolved from the database. The caching path has its own tests; what this
// file is for is the SQL, the seeded catalogue and the locking.
func newModule(t *testing.T) (*rbac.Module, http.Handler) {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	const reset = `TRUNCATE core.users CASCADE; TRUNCATE ops.outbox_events`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	module := rbac.New(rbac.Deps{Pool: pool, Env: "test"})
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) { module.Routes(api) })
	return module, router
}

// createUser inserts an account directly. This suite is about roles, and
// going through the user module to make one would couple the two.
func createUser(t *testing.T, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	userID, err := id.NewUUIDv7(ctx)
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	const insert = `INSERT INTO core.users (id, email) VALUES ($1, $2)`
	if _, err := pool.Exec(ctx, insert, userID, email); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

func grant(t *testing.T, userID uuid.UUID, role contract.Role) {
	t.Helper()
	const insert = `
		INSERT INTO core.user_roles (user_id, role_id)
		SELECT $1, id FROM core.roles WHERE name = $2
		ON CONFLICT DO NOTHING`
	if _, err := pool.Exec(context.Background(), insert, userID, string(role)); err != nil {
		t.Fatalf("grant %s: %v", role, err)
	}
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

// TestSeededCatalogueMatchesTheConstants is the check that keeps
// contract.All() and the migration from drifting apart. A constant with no row
// silently denies; a row with no constant is unreachable. Both are bugs, and
// neither shows up anywhere else.
func TestSeededCatalogueMatchesTheConstants(t *testing.T) {
	newModule(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `SELECT name FROM core.permissions ORDER BY name`)
	if err != nil {
		t.Fatalf("read permissions: %v", err)
	}
	defer rows.Close()

	var stored []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		stored = append(stored, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}

	declared := make([]string, 0, len(contract.All()))
	for _, permission := range contract.All() {
		declared = append(declared, permission.String())
	}
	slices.Sort(declared)

	if !slices.Equal(stored, declared) {
		t.Errorf("the catalogue and contract.All() disagree\n  database: %v\n  constants: %v", stored, declared)
	}
}

// TestSeededRolesAreExactlyTwo is BR-RBAC-02 against the real seed.
func TestSeededRolesAreExactlyTwo(t *testing.T) {
	newModule(t)

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM core.roles`).Scan(&count); err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 2 {
		t.Errorf("core.roles has %d rows, want exactly 2", count)
	}
}

// TestAdminHoldsEverythingAndLearnerHoldsNothing checks the role-permission
// mapping the migration builds with a set difference.
func TestAdminHoldsEverythingAndLearnerHoldsNothing(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	admin := createUser(t, "admin@fluentra.test")
	grant(t, admin, contract.RoleAdmin)
	learner := createUser(t, "learner@fluentra.test")
	grant(t, learner, contract.RoleUser)

	adminPermissions, err := module.RoleReader().PermissionsOf(ctx, admin)
	if err != nil {
		t.Fatalf("PermissionsOf(admin): %v", err)
	}
	if len(adminPermissions) != len(contract.All()) {
		t.Errorf("admin holds %d permissions, want the whole catalogue (%d)",
			len(adminPermissions), len(contract.All()))
	}

	learnerPermissions, err := module.RoleReader().PermissionsOf(ctx, learner)
	if err != nil {
		t.Fatalf("PermissionsOf(learner): %v", err)
	}
	if len(learnerPermissions) != 0 {
		t.Errorf("a learner holds %v, want none", learnerPermissions)
	}
}

// TestGuardDeniesEveryPermissionForAnAccountWithNoRoles is deny-by-default
// against real data: an account that exists but was never granted anything is
// refused everything, including permissions that are not in the catalogue.
func TestGuardDeniesEveryPermissionForAnAccountWithNoRoles(t *testing.T) {
	module, _ := newModule(t)

	roleless := createUser(t, "roleless@fluentra.test")
	ctx := httpx.WithActor(context.Background(), httpx.Actor{UserID: roleless})

	for _, permission := range contract.All() {
		if err := module.Authorizer().Require(ctx, permission); err == nil {
			t.Errorf("an account with no roles was allowed %s", permission)
		}
	}
	for _, undeclared := range []contract.Permission{"", "admin", "rbac.*"} {
		if err := module.Authorizer().Require(ctx, undeclared); err == nil {
			t.Errorf("an undeclared permission %q was allowed", undeclared)
		}
	}
}

// TestLastAdminIsProtectedAgainstConcurrentRevocations drives the race the
// guard exists for: two administrators revoke each other at the same moment.
// Exactly one may win. If both did, the system would have no administrator and
// no way to appoint one.
//
// SERIALIZABLE plus dbx.InTx's retry is what holds this, not the row lock —
// removing the lock and running this fifteen times still passes.
func TestLastAdminIsProtectedAgainstConcurrentRevocations(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	first := createUser(t, "first-admin@fluentra.test")
	second := createUser(t, "second-admin@fluentra.test")
	grant(t, first, contract.RoleAdmin)
	grant(t, second, contract.RoleAdmin)

	// Each revokes the other, at the same time.
	type outcome struct{ err error }
	results := make(chan outcome, 2)
	go func() {
		_, err := module.RevokeRole(ctx, first, second, contract.RoleAdmin)
		results <- outcome{err}
	}()
	go func() {
		_, err := module.RevokeRole(ctx, second, first, contract.RoleAdmin)
		results <- outcome{err}
	}()
	succeeded := 0
	for range 2 {
		if (<-results).err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Errorf("%d of 2 concurrent revocations succeeded, want exactly 1", succeeded)
	}

	var remaining int
	const count = `
		SELECT count(*) FROM core.user_roles ur
		JOIN core.roles r ON r.id = ur.role_id WHERE r.name = 'admin'`
	if err := pool.QueryRow(ctx, count).Scan(&remaining); err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("%d administrators remain, want exactly 1", remaining)
	}
}

// TestRoleChangeIsServedAndRecorded is the end-to-end path: a real request
// changes a real row and leaves an event for `audit` to consume.
func TestRoleChangeIsServedAndRecorded(t *testing.T) {
	module, router := newModule(t)

	admin := createUser(t, "granter@fluentra.test")
	grant(t, admin, contract.RoleAdmin)
	learner := createUser(t, "grantee@fluentra.test")
	grant(t, learner, contract.RoleUser)

	path := "/api/v1/admin/users/" + learner.String() + "/roles"
	recorder := request(t, router, http.MethodPost, path, admin, `{"role":"admin"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	roles, err := module.RoleReader().RolesOf(context.Background(), learner)
	if err != nil {
		t.Fatalf("RolesOf: %v", err)
	}
	if !slices.Contains(roles, contract.RoleAdmin) {
		t.Errorf("roles = %v, want admin among them", roles)
	}

	events := outboxEvents(t)
	if len(events) != 1 || events[0] != contract.EventRoleAssigned {
		t.Fatalf("outbox = %v, want one %s", events, contract.EventRoleAssigned)
	}
}

// TestLearnerIsRefusedByTheAdminGroup is the acceptance criterion, end to end
// against real role data rather than a fake.
func TestLearnerIsRefusedByTheAdminGroup(t *testing.T) {
	_, router := newModule(t)

	learner := createUser(t, "nosy@fluentra.test")
	grant(t, learner, contract.RoleUser)
	target := createUser(t, "target@fluentra.test")

	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/roles", ""},
		{http.MethodPost, "/api/v1/admin/users/" + target.String() + "/roles", `{"role":"admin"}`},
		{http.MethodDelete, "/api/v1/admin/users/" + target.String() + "/roles/user", ""},
	}
	for _, testCase := range cases {
		recorder := request(t, router, testCase.method, testCase.path, learner, testCase.body)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", testCase.method, testCase.path, recorder.Code)
		}
	}

	// And nothing was granted along the way.
	var assignments int
	const count = `SELECT count(*) FROM core.user_roles WHERE user_id = $1`
	if err := pool.QueryRow(context.Background(), count, target).Scan(&assignments); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if assignments != 0 {
		t.Errorf("the target has %d role assignments, want 0", assignments)
	}
}

func TestMyPermissionsReflectsRealRoles(t *testing.T) {
	_, router := newModule(t)

	admin := createUser(t, "me-admin@fluentra.test")
	grant(t, admin, contract.RoleAdmin)

	recorder := request(t, router, http.MethodGet, "/api/v1/me/permissions", admin, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}

	var body struct {
		Roles       []string `json:"roles"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Roles) != 1 || body.Roles[0] != "admin" {
		t.Errorf("roles = %v, want [admin]", body.Roles)
	}
	if len(body.Permissions) != len(contract.All()) {
		t.Errorf("permissions = %d entries, want the catalogue (%d)", len(body.Permissions), len(contract.All()))
	}
}

// TestForgetUserRemovesAssignments is the `user.deleted` reaction, and the
// cascade behind it: deleting the account takes the assignments with it.
func TestForgetUserRemovesAssignments(t *testing.T) {
	module, _ := newModule(t)
	ctx := context.Background()

	doomed := createUser(t, "doomed@fluentra.test")
	grant(t, doomed, contract.RoleUser)

	if err := module.ForgetUser(ctx, doomed); err != nil {
		t.Fatalf("ForgetUser: %v", err)
	}
	roles, err := module.RoleReader().RolesOf(ctx, doomed)
	if err != nil {
		t.Fatalf("RolesOf: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("roles = %v, want none", roles)
	}
}

func outboxEvents(t *testing.T) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT event FROM ops.outbox_events ORDER BY created_at`)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()

	var events []string
	for rows.Next() {
		var event string
		if err := rows.Scan(&event); err != nil {
			t.Fatalf("scan: %v", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	return events
}
