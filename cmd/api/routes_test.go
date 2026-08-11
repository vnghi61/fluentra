package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	auditcontract "github.com/fluentra/fluentra/internal/modules/audit/contract"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// The wiring tests below need no database. Every operation resolves the caller
// before it touches one, so an unauthenticated request proves the route is
// mounted without a single query — which is exactly the thing P1.5 changes and
// nothing else about these modules.

func newWiredRouter(t *testing.T) http.Handler {
	t.Helper()

	// A nil pool is safe here and only here: construction touches nothing, and
	// no request in this file gets past the guard.
	modules := newIdentity(identityDeps{
		Env:        "test",
		OTPHMACKey: []byte("test-otp-hmac-key-at-least-32-bytes-long"),
		Tokens: authservice.TokenConfig{
			SigningKey: []byte("test-jwt-signing-key-at-least-32-bytes-long"),
			Issuer:     "fluentra-test",
			Audience:   testServiceName,
		},
	})
	return httpx.NewRouter(httpx.RouterDependencies{Modules: modules.Routes})
}

func call(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	var body = http.NoBody
	request := httptest.NewRequest(method, path, body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestEveryPhaseOneOperationIsMounted is the P1.5 acceptance criterion "`/me`
// and `/admin/*` reachable", stated so that it fails for the right reason.
//
// It asserts "not 404" rather than a particular status. A route that is not
// mounted returns 404; a route that is mounted and refuses an anonymous caller
// returns 401 or 403. Asserting the refusal is what distinguishes "wired" from
// "wired and open".
func TestEveryPhaseOneOperationIsMounted(t *testing.T) {
	t.Parallel()

	router := newWiredRouter(t)
	someone := uuid.New().String()

	// Which module owns the route, for the failure message.
	const (
		ownerUser  = "user"
		ownerRBAC  = "rbac"
		ownerAudit = "audit"
	)

	cases := []struct {
		method, path string
		owner        string
	}{
		{http.MethodGet, "/api/v1/me", ownerUser},
		{http.MethodPatch, "/api/v1/me", ownerUser},
		{http.MethodGet, "/api/v1/me/preferences", ownerUser},
		{http.MethodPut, "/api/v1/me/preferences", ownerUser},
		{http.MethodGet, "/api/v1/me/permissions", ownerRBAC},
		{http.MethodGet, "/api/v1/admin/roles", ownerRBAC},
		{http.MethodPost, "/api/v1/admin/users/" + someone + "/roles", ownerRBAC},
		{http.MethodDelete, "/api/v1/admin/users/" + someone + "/roles/user", ownerRBAC},
		{http.MethodGet, "/api/v1/admin/audit-logs", ownerAudit},
		{http.MethodGet, "/api/v1/admin/security-events", ownerAudit},
		{http.MethodPost, "/api/v1/admin/security-events/" + someone + "/resolve", ownerAudit},
	}

	for _, testCase := range cases {
		recorder := call(t, router, testCase.method, testCase.path)
		if recorder.Code == http.StatusNotFound {
			t.Errorf("%s %s (%s) is not mounted", testCase.method, testCase.path, testCase.owner)
			continue
		}
		if recorder.Code != http.StatusUnauthorized && recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s (%s) answered an anonymous caller with %d, want 401 or 403",
				testCase.method, testCase.path, testCase.owner, recorder.Code)
		}
	}
}

// TestTheAdminPrefixRefusesAnonymousCallersWhole.
//
// `rbac` mounts a subrouter at `/admin`, so its catch-all answers every path
// under that prefix — including ones no handler exists for. An anonymous caller
// therefore cannot tell a real admin route from an invented one, which is the
// behaviour you want: the shape of the back office is not public.
//
// It also means this test cannot tell which module served `/admin/audit-logs`,
// because rbac's catch-all and audit's handler both refuse before they differ.
// That distinction needs a signed-in administrator and a database, and it is
// made in wiring_integration_test.go.
func TestTheAdminPrefixRefusesAnonymousCallersWhole(t *testing.T) {
	t.Parallel()

	router := newWiredRouter(t)
	for _, path := range []string{
		"/api/v1/admin/audit-logs",
		"/api/v1/admin/security-events",
		"/api/v1/admin/roles",
		"/api/v1/admin/nothing-here",
	} {
		recorder := call(t, router, http.MethodGet, path)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s answered an anonymous caller with %d, want 401 from AdminOnly", path, recorder.Code)
		}
	}
}

// TestUnknownRoutesOutsideAdminStill404 is the control. Without it, a router
// that refused everything would pass the test above.
//
// The paths are all outside `/admin` for the reason given there.
func TestUnknownRoutesOutsideAdminStill404(t *testing.T) {
	t.Parallel()

	router := newWiredRouter(t)
	for _, path := range []string{
		"/api/v1/me/nothing-here",
		"/api/v1/users/00000000-0000-0000-0000-000000000000",
		"/api/v1/nothing-here",
	} {
		if recorder := call(t, router, http.MethodGet, path); recorder.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, recorder.Code)
		}
	}
}

// TestInfrastructureRoutesSurviveTheModules checks the modules did not displace
// what was already there.
func TestInfrastructureRoutesSurviveTheModules(t *testing.T) {
	t.Parallel()

	router := newWiredRouter(t)
	if recorder := call(t, router, http.MethodGet, "/api/v1/ping"); recorder.Code == http.StatusNotFound {
		t.Error("/api/v1/ping stopped being routed once the modules were mounted")
	}
}

// TestGuardConvertsPermissionsWithoutLosingThem keeps the string boundary
// between `audit` and `rbac` honest: the names audit asks for must be names
// rbac's catalogue actually contains, or every audit operation would 403 for
// an administrator holding every permission.
func TestGuardConvertsPermissionsWithoutLosingThem(t *testing.T) {
	t.Parallel()

	catalogue := make(map[rbaccontract.Permission]struct{}, len(rbaccontract.All()))
	for _, permission := range rbaccontract.All() {
		catalogue[permission] = struct{}{}
	}

	asked := []string{
		auditcontract.PermissionRead,
		auditcontract.PermissionExport,
		auditcontract.PermissionManage,
	}
	for _, permission := range asked {
		if _, found := catalogue[rbaccontract.Permission(permission)]; !found {
			t.Errorf("audit asks for %q, which is not in rbac's catalogue", permission)
		}
	}
}
