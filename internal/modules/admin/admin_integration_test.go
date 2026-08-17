//go:build integration

// Package admin_test exercises the P4.1 admin user-management journey against a
// real PostgreSQL: an administrator suspends a learner and the learner's
// sessions die, an administrator cannot act on their own account, and a
// non-admin is refused every admin endpoint.
//
// It is an integration suite rather than a unit one because what it asserts is
// a property of the database and of the wiring between modules: the suspension
// must change the user row the user module owns, revoke the session rows the
// auth module owns, and refuse the routes the rbac module guards — none of
// which a fake can prove.
package admin_test

import (
	"context"
	"database/sql"
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
	"github.com/fluentra/fluentra/internal/modules/admin"
	admincontract "github.com/fluentra/fluentra/internal/modules/admin/contract"
	"github.com/fluentra/fluentra/internal/modules/audit"
	"github.com/fluentra/fluentra/internal/modules/auth"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/user"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
)

// adminIntegrationDatabase is this package's own, for the reason every suite in
// the repository has one: sharing TEST_DATABASE_URL means one package's truncate
// is another package's missing row.
const adminIntegrationDatabase = "fluentra_admin_integration_test"

// signingKey is long enough for auth.New to accept it and is not a secret in
// any deployment.
const signingKey = "integration-test-jwt-signing-key-32b" // gitleaks:allow

// otpKey keys the challenge HMAC. Any 32 bytes will do; the value is never
// asserted.
const otpKey = "integration-test-hmac-key-32-bytes--" // gitleaks:allow

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		os.Exit(m.Run())
	}

	dsn, dropDatabase, err := createDatabase(base, adminIntegrationDatabase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare %s: %v\n", adminIntegrationDatabase, err)
		os.Exit(1)
	}
	if err := migrateUp(dsn); err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", adminIntegrationDatabase, err)
		os.Exit(1)
	}

	created, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		dropDatabase()
		fmt.Fprintf(os.Stderr, "pool for %s: %v\n", adminIntegrationDatabase, err)
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
	adminConn, err := sql.Open("pgx", maintenance)
	if err != nil {
		return "", nil, fmt.Errorf("open maintenance database: %w", err)
	}
	defer func() { _ = adminConn.Close() }()

	ctx := context.Background()
	drop := fmt.Sprintf("DROP DATABASE IF EXISTS %q WITH (FORCE)", name)
	if _, err := adminConn.ExecContext(ctx, drop); err != nil {
		return "", nil, fmt.Errorf("drop stale %s: %w", name, err)
	}
	if _, err := adminConn.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name)); err != nil {
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

// testSystem is the five modules wired the way the composition root wires them:
// the same admin group, the same AdminOnly guard, the same lazy guard the admin
// handler needs.
type testSystem struct {
	router http.Handler
	user   *user.Module
	// flags is the same FlagReader other modules consume, so an evaluation in a
	// test goes through the cache and bucketing production uses.
	flags admincontract.FlagReader
}

// systemGuard adapts rbac's Authorizer to the string-keyed guard the admin
// handler declares, exactly as lazyGuard does in cmd/api/modules.go.
type systemGuard struct {
	rbac *rbac.Module
}

func (g systemGuard) Require(ctx context.Context, permission string) error {
	return g.rbac.Authorizer().Require(ctx, rbaccontract.Permission(permission))
}

func setupTestSystem(t *testing.T) *testSystem {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	auditMod := audit.New(audit.Deps{Pool: pool})
	rbacMod := rbac.New(rbac.Deps{Pool: pool})
	userMod := user.New(user.Deps{Pool: pool})

	jwtKey := []byte(signingKey)
	authMod := auth.New(auth.Deps{
		Pool:       pool,
		OTPHMACKey: []byte(otpKey),
		Registrar:  userMod.Registrar(),
		Tokens: authservice.TokenConfig{
			SigningKey: jwtKey,
			Issuer:     "fluentra",
			Audience:   "fluentra-api",
		},
		Roles: rbacMod.RoleReader(),
	})

	adminMod := admin.New(admin.Deps{
		Pool:           pool,
		UserReader:     userMod.AdminReader(),
		UserManager:    userMod.AdminManager(),
		SessionRevoker: authMod.SessionRevoker(),
		Audit:          auditMod.Recorder(),
		Guard:          systemGuard{rbac: rbacMod},
	})

	r := chi.NewRouter()
	r.Group(func(api chi.Router) {
		api.Use(authMod.Authenticate())
		userMod.Routes(api)
		rbacMod.Routes(api)
		authMod.Routes(api)

		api.Group(func(adm chi.Router) {
			adm.Use(rbacMod.AdminOnly())
			auditMod.Routes(adm)
			adminMod.Routes(adm)
		})
	})

	return &testSystem{router: r, user: userMod, flags: adminMod.FlagReader()}
}

// createTestUser creates an account through the real user module, so the user,
// profile and preference rows are written the way production writes them.
func createTestUser(t *testing.T, sys *testSystem, email string) uuid.UUID {
	t.Helper()
	userID, err := sys.user.Registrar().CreateUser(context.Background(), usercontract.NewUser{
		Email:       email,
		DisplayName: "Test Learner",
		Locale:      "en",
		Timezone:    "UTC",
	})
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
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

// mintToken signs an access token the auth module's Authenticate middleware
// will verify, carrying the given role claim.
func mintToken(t *testing.T, userID uuid.UUID, role string) string {
	t.Helper()
	token, err := authservice.IssueAccessTokenForTest([]byte(signingKey), userID, role)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

func doRequest(
	t *testing.T, sys *testSystem, method, path, token, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	sys.router.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------- tests

// TestModule_AdminSuspendUser_SuspendsAndRevokesSessions is the P4.1 acceptance
// criterion: a suspension changes the user status the user module owns and
// revokes the session rows the auth module owns.
//
// It deliberately does NOT assert that the learner's already-issued access
// token dies instantly. SessionRevoker.RevokeAll revokes sessions and refresh
// tokens, not access tokens; an access token stays valid until it expires
// (ADR-0007's accepted ≤15-minute window). What is guaranteed — and asserted
// here — is that the session is revoked, so login and refresh are refused
// immediately.
func TestModule_AdminSuspendUser_SuspendsAndRevokesSessions(t *testing.T) {
	sys := setupTestSystem(t)
	ctx := context.Background()

	adminID := createTestUser(t, sys, "admin@fluentra.test")
	grantAdmin(t, adminID)
	targetID := createTestUser(t, sys, "learner@fluentra.test")

	// Give the learner an active session, so "revokes sessions" has something
	// to revoke.
	sessionID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO core.sessions (id, user_id, absolute_expires_at, idle_window)
		VALUES ($1, $2, NOW() + INTERVAL '1 day', INTERVAL '30 days')`,
		sessionID, targetID); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	adminToken := mintToken(t, adminID, "admin")
	body := `{"reason":"Repeated violations of the community guidelines"}`
	rec := doRequest(t, sys, http.MethodPost,
		"/admin/users/"+targetID.String()+"/suspend", adminToken, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("suspend = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	// The user module owns the status change.
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM core.users WHERE id = $1`, targetID).Scan(&status); err != nil {
		t.Fatalf("read user status: %v", err)
	}
	if status != "suspended" {
		t.Errorf("status = %q, want suspended", status)
	}

	// The auth module owns the session revocation.
	var revokedAt *string
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at::text FROM core.sessions WHERE id = $1`, sessionID).Scan(&revokedAt); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if revokedAt == nil {
		t.Error("the session survived the suspension")
	}

	// The admin module owns the action record.
	var actions int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM core.admin_actions WHERE target_id = $1 AND action = 'suspend'`,
		targetID).Scan(&actions); err != nil {
		t.Fatalf("count admin actions: %v", err)
	}
	if actions != 1 {
		t.Errorf("%d admin action rows, want 1", actions)
	}
}

// TestModule_AdminCannotSuspendSelf is the self-administration guard at the
// boundary: the service refuses before any write, and the account stays active.
func TestModule_AdminCannotSuspendSelf(t *testing.T) {
	sys := setupTestSystem(t)
	ctx := context.Background()

	adminID := createTestUser(t, sys, "self@fluentra.test")
	grantAdmin(t, adminID)
	adminToken := mintToken(t, adminID, "admin")

	body := `{"reason":"Trying to suspend myself"}`
	rec := doRequest(t, sys, http.MethodPost,
		"/admin/users/"+adminID.String()+"/suspend", adminToken, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("self-suspension = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM core.users WHERE id = $1`, adminID).Scan(&status); err != nil {
		t.Fatalf("read user status: %v", err)
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}
}

// TestModule_NonAdminCannotAccessAdminEndpoints checks the AdminOnly route
// guard: a learner is refused every admin route with 403 before any handler
// runs.
func TestModule_NonAdminCannotAccessAdminEndpoints(t *testing.T) {
	sys := setupTestSystem(t)

	learnerID := createTestUser(t, sys, "learner-forbidden@fluentra.test")
	learnerToken := mintToken(t, learnerID, "user")

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/users"},
		{http.MethodGet, "/admin/users/" + learnerID.String()},
		{http.MethodPost, "/admin/users/" + learnerID.String() + "/suspend"},
		{http.MethodPost, "/admin/users/" + learnerID.String() + "/reinstate"},
		{http.MethodPost, "/admin/users/" + learnerID.String() + "/sessions/revoke"},
		{http.MethodGet, "/admin/flags"},
		{http.MethodPost, "/admin/flags"},
		{http.MethodPut, "/admin/flags/example"},
		{http.MethodDelete, "/admin/flags/example"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			body := ""
			if ep.method == http.MethodPost || ep.method == http.MethodPut {
				body = `{"reason":"A reason long enough to pass validation"}`
			}
			rec := doRequest(t, sys, ep.method, ep.path, learnerToken, body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s = %d, want 403", ep.method, ep.path, rec.Code)
			}
		})
	}
}

// TestModule_FeatureFlagCRUDRoundTrip exercises the P4.2 flag endpoints against
// a real database.
//
// It exists because a coverage run showed every feature-flag method on the
// repository at 0%: the service had unit tests with a fake repo, and the HTTP
// layer had handler tests with a fake service, so both sides were tested and
// the SQL between them had never once run. The CHECK constraints on
// core.feature_flags in particular — rollout_percent's range and expires_on
// being in the future — are enforced by the database and by nothing else, so a
// fake cannot show they hold.
func TestModule_FeatureFlagCRUDRoundTrip(t *testing.T) {
	sys := setupTestSystem(t)
	adminID := createTestUser(t, sys, "flag-admin@fluentra.test")
	grantAdmin(t, adminID)
	token := mintToken(t, adminID, "admin")

	const key = "streaks_v2"
	expiry := time.Now().AddDate(0, 2, 0).Format("2006-01-02")

	create := fmt.Sprintf(
		`{"key":%q,"enabled":false,"rollout_percent":0,"owner":"@backend-team",`+
			`"expires_on":%q,"description":"Second-generation streak calculation."}`, key, expiry)
	if rec := doRequest(t, sys, http.MethodPost, "/admin/flags", token, create); rec.Code != http.StatusCreated {
		t.Fatalf("create flag: status %d, body %s", rec.Code, rec.Body)
	}

	// The key is the primary key, so the second create is the conflict path.
	if rec := doRequest(t, sys, http.MethodPost, "/admin/flags", token, create); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create: status %d, want 409 (body %s)", rec.Code, rec.Body)
	}

	listed := doRequest(t, sys, http.MethodGet, "/admin/flags", token, "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list flags: status %d, body %s", listed.Code, listed.Body)
	}
	if !strings.Contains(listed.Body.String(), key) {
		t.Fatalf("list does not contain the flag just created: %s", listed.Body)
	}

	update := fmt.Sprintf(
		`{"enabled":true,"rollout_percent":50,"expires_on":%q,"description":"Rolled out to half."}`, expiry)
	updated := doRequest(t, sys, http.MethodPut, "/admin/flags/"+key, token, update)
	if updated.Code != http.StatusOK {
		t.Fatalf("update flag: status %d, body %s", updated.Code, updated.Body)
	}

	// Evaluation reads through the 30 s cache, which the update invalidates.
	// At 100% the bucketing is bypassed, so this asserts the write landed
	// rather than asserting anything about which side of a rollout a user sits.
	full := fmt.Sprintf(
		`{"enabled":true,"rollout_percent":100,"expires_on":%q,"description":"Everyone."}`, expiry)
	if rec := doRequest(t, sys, http.MethodPut, "/admin/flags/"+key, token, full); rec.Code != http.StatusOK {
		t.Fatalf("update to 100%%: status %d, body %s", rec.Code, rec.Body)
	}
	enabled, err := sys.flags.IsEnabled(context.Background(), key, adminID)
	if err != nil {
		t.Fatalf("evaluate flag: %v", err)
	}
	if !enabled {
		t.Fatal("flag reads as disabled at 100% rollout")
	}

	if rec := doRequest(t, sys, http.MethodDelete, "/admin/flags/"+key, token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete flag: status %d, body %s", rec.Code, rec.Body)
	}
	if rec := doRequest(t, sys, http.MethodPut, "/admin/flags/"+key, token, update); rec.Code != http.StatusNotFound {
		t.Fatalf("update after delete: status %d, want 404 (body %s)", rec.Code, rec.Body)
	}
}

// TestModule_FeatureFlagValidationIsEnforcedByTheDatabase covers the refusals
// that a fake repository cannot prove, because the constraint lives in the
// migration rather than in Go.
func TestModule_FeatureFlagValidationIsEnforcedByTheDatabase(t *testing.T) {
	sys := setupTestSystem(t)
	adminID := createTestUser(t, sys, "flag-validation@fluentra.test")
	grantAdmin(t, adminID)
	token := mintToken(t, adminID, "admin")

	future := time.Now().AddDate(0, 2, 0).Format("2006-01-02")
	past := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	for _, testCase := range []struct {
		name string
		body string
	}{
		{
			name: "expiry in the past",
			body: fmt.Sprintf(`{"key":"past_flag","owner":"@team","expires_on":%q,`+
				`"description":"Expired on arrival."}`, past),
		},
		{
			name: "rollout above 100",
			body: fmt.Sprintf(`{"key":"over_flag","owner":"@team","rollout_percent":101,`+
				`"expires_on":%q,"description":"Impossible share."}`, future),
		},
		{
			name: "no owner",
			body: fmt.Sprintf(`{"key":"ownerless_flag","owner":"","expires_on":%q,`+
				`"description":"Nobody to chase."}`, future),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rec := doRequest(t, sys, http.MethodPost, "/admin/flags", token, testCase.body)
			if rec.Code < 400 || rec.Code >= 500 {
				t.Fatalf("status %d, want a 4xx refusal (body %s)", rec.Code, rec.Body)
			}
		})
	}
}
