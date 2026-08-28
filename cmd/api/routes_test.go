package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
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
		ownerUser    = "user"
		ownerRBAC    = "rbac"
		ownerAudit   = "audit"
		ownerContent = "content"
		ownerLesson  = "lesson"
	)

	cases := []struct {
		method, path string
		owner        string
	}{
		{http.MethodGet, "/api/v1/me", ownerUser},
		{http.MethodPatch, "/api/v1/me", ownerUser},
		{http.MethodGet, "/api/v1/me/preferences", ownerUser},
		{http.MethodPut, "/api/v1/me/preferences", ownerUser},
		{http.MethodPost, "/api/v1/me/avatar/upload-intent", ownerUser},
		{http.MethodPut, "/api/v1/me/avatar", ownerUser},
		{http.MethodGet, "/api/v1/me/permissions", ownerRBAC},
		{http.MethodGet, "/api/v1/admin/roles", ownerRBAC},
		{http.MethodPost, "/api/v1/admin/users/" + someone + "/roles", ownerRBAC},
		{http.MethodDelete, "/api/v1/admin/users/" + someone + "/roles/user", ownerRBAC},
		{http.MethodGet, "/api/v1/admin/audit-logs", ownerAudit},
		{http.MethodGet, "/api/v1/admin/security-events", ownerAudit},
		{http.MethodPost, "/api/v1/admin/security-events/" + someone + "/resolve", ownerAudit},
		{http.MethodGet, "/api/v1/content", ownerContent},
		{http.MethodGet, "/api/v1/content/sample-slug", ownerContent},
		{http.MethodPost, "/api/v1/admin/content", ownerContent},
		{http.MethodPut, "/api/v1/admin/content/" + someone + "/draft", ownerContent},
		{http.MethodPost, "/api/v1/admin/content/" + someone + "/submit", ownerContent},
		{http.MethodPost, "/api/v1/admin/content/" + someone + "/review", ownerContent},
		{http.MethodPost, "/api/v1/admin/content/" + someone + "/publish", ownerContent},
		{http.MethodPost, "/api/v1/admin/content/" + someone + "/archive", ownerContent},
		// The three learner-facing lesson routes moved to
		// TestPublicCurriculumRoutesAreMounted. They no longer refuse an
		// anonymous caller — that is the point of ADR-0025 — so they cannot be
		// asserted by a test whose question is "does this refuse?", and a
		// request that now reaches the service would dereference this file's
		// deliberately nil pool.
		{http.MethodPost, "/api/v1/admin/courses", ownerLesson},
		{http.MethodPut, "/api/v1/admin/lessons/" + someone + "/activities", ownerLesson},
		{http.MethodPost, "/api/v1/admin/lessons/" + someone + "/publish", ownerLesson},
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

// TestPublicCurriculumRoutesAreMounted checks the three routes ADR-0025 opened.
//
// It reads the route tree rather than issuing a request, and that is the whole
// reason it is a separate test. Every other route in this file proves it is
// mounted by being refused, which never reaches a handler and so never touches
// the deliberately nil pool. These three are refused by nothing now, so a
// request would run the real service and panic on that pool — proving the route
// is open, then failing for a reason that has nothing to do with routing.
//
// Walking the tree answers the question this test actually asks: is the pattern
// registered? Whether it is open to anonymous callers is asserted where the
// handler can be given a fake service, in the lesson module's own suite.
func TestPublicCurriculumRoutesAreMounted(t *testing.T) {
	t.Parallel()

	mux, ok := newWiredRouter(t).(*chi.Mux)
	if !ok {
		t.Fatal("the API router is no longer a chi.Mux; this test reads its route tree")
	}

	registered := make(map[string]bool)
	err := chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	for _, route := range []string{
		"GET /api/v1/courses",
		"GET /api/v1/courses/{slug}",
		"GET /api/v1/lessons/{id}",
	} {
		if !registered[route] {
			t.Errorf("%s is not mounted", route)
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
