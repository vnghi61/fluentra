package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/rbac/domain"
	rbachttp "github.com/fluentra/fluentra/internal/modules/rbac/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

var (
	adminID   = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	learnerID = uuid.MustParse("0199b2d3-4e5f-7a81-8bcd-ef0123456789")
)

const testRequestID = "01KZGA1FXY6VAHQABK3EBKDN57"

// fieldRole is the member the assign-role body carries.
const fieldRole = "role"

// fakeRoles is a service stand-in whose answers come from a role map, so a
// test can say "this caller is a learner" and have every layer below agree.
type fakeRoles struct {
	roles      map[uuid.UUID][]contract.Role
	rolesErr   error
	assignErr  error
	revokeErr  error
	seenActor  uuid.UUID
	seenTarget uuid.UUID
	seenRole   contract.Role
	assigned   bool
	revoked    bool
}

func newFakeRoles() *fakeRoles {
	return &fakeRoles{roles: map[uuid.UUID][]contract.Role{
		adminID:   {contract.RoleAdmin, contract.RoleUser},
		learnerID: {contract.RoleUser},
	}}
}

func (f *fakeRoles) HasRole(_ context.Context, userID uuid.UUID, role contract.Role) (bool, error) {
	if f.rolesErr != nil {
		return false, f.rolesErr
	}
	return slices.Contains(f.roles[userID], role), nil
}

// Require mirrors the real guard closely enough for the transport tests: the
// admin holds the whole catalogue, everybody else holds nothing.
func (f *fakeRoles) Require(ctx context.Context, permission contract.Permission) error {
	if !permission.Valid() {
		return domain.ErrPermissionDenied
	}
	actor, ok := httpx.ActorFrom(ctx)
	if !ok {
		return domain.ErrPermissionDenied
	}
	if slices.Contains(f.roles[actor.UserID], contract.RoleAdmin) {
		return nil
	}
	return domain.ErrPermissionDenied
}

func (f *fakeRoles) RolesOf(_ context.Context, userID uuid.UUID) ([]contract.Role, error) {
	if f.rolesErr != nil {
		return nil, f.rolesErr
	}
	return f.roles[userID], nil
}

func (f *fakeRoles) PermissionsOf(_ context.Context, userID uuid.UUID) ([]contract.Permission, error) {
	if slices.Contains(f.roles[userID], contract.RoleAdmin) {
		return contract.All(), nil
	}
	return nil, nil
}

func (f *fakeRoles) ListRoles(context.Context) ([]domain.RoleWithPermissions, error) {
	return []domain.RoleWithPermissions{
		{Name: contract.RoleAdmin, Description: "Full administrative access.", Permissions: contract.All()},
		{Name: contract.RoleUser, Description: "A learner.", Permissions: nil},
	}, nil
}

func (f *fakeRoles) AssignRole(
	_ context.Context, actorID, targetID uuid.UUID, role contract.Role,
) ([]contract.Role, error) {
	f.seenActor, f.seenTarget, f.seenRole, f.assigned = actorID, targetID, role, true
	if f.assignErr != nil {
		return nil, f.assignErr
	}
	return []contract.Role{role}, nil
}

func (f *fakeRoles) RevokeRole(
	_ context.Context, actorID, targetID uuid.UUID, role contract.Role,
) ([]contract.Role, error) {
	f.seenActor, f.seenTarget, f.seenRole, f.revoked = actorID, targetID, role, true
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	return []contract.Role{contract.RoleUser}, nil
}

func newServer(roles rbachttp.Roles) http.Handler {
	router := chi.NewRouter()
	router.Route("/api/v1", func(api chi.Router) { rbachttp.NewHandler(roles).Routes(api) })
	return router
}

func do(
	t *testing.T, handler http.Handler, method, path, body string, actor *uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	ctx := httpx.WithRequestID(request.Context(), testRequestID)
	if actor != nil {
		ctx = httpx.WithActor(ctx, httpx.Actor{UserID: *actor, Role: "user"})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))
	return recorder
}

type problem struct {
	Status int              `json:"status"`
	Code   string           `json:"code"`
	Errors []fieldViolation `json:"errors"`
}

type fieldViolation struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

func decodeProblem(t *testing.T, recorder *httptest.ResponseRecorder) problem {
	t.Helper()
	var decoded problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode problem: %v (body %s)", err, recorder.Body)
	}
	return decoded
}

// adminRoutes is every route under /admin this module mounts. The tests below
// assert against all of them rather than a sample, because "a user gets 403 on
// /admin/*" is a claim about the group, not about one handler.
var adminRoutes = []struct{ method, path, body string }{
	{http.MethodGet, "/api/v1/admin/roles", ""},
	{http.MethodPost, "/api/v1/admin/users/" + learnerID.String() + "/roles", `{"role":"admin"}`},
	{http.MethodDelete, "/api/v1/admin/users/" + learnerID.String() + "/roles/admin", ""},
}

// TestAdminRoutes_RefuseALearner is the P1.3 acceptance criterion.
func TestAdminRoutes_RefuseALearner(t *testing.T) {
	t.Parallel()
	roles := newFakeRoles()
	server := newServer(roles)

	for _, route := range adminRoutes {
		recorder := do(t, server, route.method, route.path, route.body, &learnerID)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", route.method, route.path, recorder.Code)
			continue
		}
		if got := decodeProblem(t, recorder).Code; got != "PERMISSION_DENIED" {
			t.Errorf("%s %s code = %q, want PERMISSION_DENIED", route.method, route.path, got)
		}
	}
	if roles.assigned || roles.revoked {
		t.Error("a refused request still reached the service")
	}
}

func TestAdminRoutes_RefuseAnUnauthenticatedCaller(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())

	for _, route := range adminRoutes {
		recorder := do(t, server, route.method, route.path, route.body, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", route.method, route.path, recorder.Code)
		}
	}
}

// TestAdminRoutes_RefuseWhenTheRoleLookupFails is the failure mode that would
// otherwise turn a database outage into an unguarded back office.
func TestAdminRoutes_RefuseWhenTheRoleLookupFails(t *testing.T) {
	t.Parallel()
	roles := newFakeRoles()
	roles.rolesErr = errors.New("database unreachable")
	server := newServer(roles)

	for _, route := range adminRoutes {
		recorder := do(t, server, route.method, route.path, route.body, &adminID)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d during an outage, want 403", route.method, route.path, recorder.Code)
		}
	}
}

func TestAdminRoutes_AdmitAnAdmin(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())

	for _, route := range adminRoutes {
		recorder := do(t, server, route.method, route.path, route.body, &adminID)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s %s = %d, want 200 (body %s)", route.method, route.path, recorder.Code, recorder.Body)
		}
	}
}

func TestGetMyPermissions_ReadsTheActorAndNoOneElse(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())

	recorder := do(t, server, http.MethodGet, "/api/v1/me/permissions", "", &learnerID)
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
	if len(body.Roles) != 1 || body.Roles[0] != "user" {
		t.Errorf("roles = %v, want [user]", body.Roles)
	}
	// A learner holds no named permissions, and the field must still be an
	// empty array rather than null — the schema declares it required.
	if body.Permissions == nil {
		t.Error("permissions is null; the schema requires an array")
	}
	if len(body.Permissions) != 0 {
		t.Errorf("permissions = %v, want none", body.Permissions)
	}

	// There is no route that reads anybody else's permissions.
	other := do(t, server, http.MethodGet, "/api/v1/users/"+adminID.String()+"/permissions", "", &learnerID)
	if other.Code != http.StatusNotFound {
		t.Errorf("a per-user permissions route exists: %d", other.Code)
	}
}

func TestGetMyPermissions_RequiresACaller(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())

	recorder := do(t, server, http.MethodGet, "/api/v1/me/permissions", "", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestAssignRole_PassesTheActorAndTargetThrough(t *testing.T) {
	t.Parallel()
	roles := newFakeRoles()
	server := newServer(roles)

	path := "/api/v1/admin/users/" + learnerID.String() + "/roles"
	recorder := do(t, server, http.MethodPost, path, `{"role":"admin"}`, &adminID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	if roles.seenActor != adminID || roles.seenTarget != learnerID || roles.seenRole != contract.RoleAdmin {
		t.Errorf("service saw actor=%s target=%s role=%s", roles.seenActor, roles.seenTarget, roles.seenRole)
	}
}

func TestAssignRole_RejectsABadBody(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())
	path := "/api/v1/admin/users/" + learnerID.String() + "/roles"

	cases := map[string]struct {
		body   string
		status int
		field  string
		code   string
	}{
		"unknown field":  {body: `{"role":"admin","extra":1}`, status: 422, field: "extra", code: "UNKNOWN_FIELD"},
		"missing role":   {body: `{}`, status: 422, field: fieldRole, code: "REQUIRED"},
		"wrong type":     {body: `{"role":42}`, status: 422, field: fieldRole, code: "TYPE"},
		"a third role":   {body: `{"role":"superadmin"}`, status: 422, field: fieldRole, code: "UNKNOWN"},
		"not an object":  {body: `[]`, status: 400},
		"malformed json": {body: `{`, status: 400},
	}
	for label, testCase := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			recorder := do(t, server, http.MethodPost, path, testCase.body, &adminID)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d (body %s)", recorder.Code, testCase.status, recorder.Body)
			}
			if testCase.field == "" {
				return
			}
			decoded := decodeProblem(t, recorder)
			if len(decoded.Errors) != 1 {
				t.Fatalf("errors = %+v, want one", decoded.Errors)
			}
			if decoded.Errors[0].Field != testCase.field || decoded.Errors[0].Code != testCase.code {
				t.Errorf("violation = %+v, want %s/%s", decoded.Errors[0], testCase.field, testCase.code)
			}
		})
	}
}

// TestRoleRoutes_MalformedTargetIsNotFound keeps the routes from confirming
// what a valid id looks like to a caller who is probing.
func TestRoleRoutes_MalformedTargetIsNotFound(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())

	recorder := do(t, server, http.MethodPost, "/api/v1/admin/users/not-a-uuid/roles",
		`{"role":"admin"}`, &adminID)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestRevokeRole_RejectsARoleOutsideTheTwo(t *testing.T) {
	t.Parallel()
	roles := newFakeRoles()
	server := newServer(roles)

	path := "/api/v1/admin/users/" + learnerID.String() + "/roles/superadmin"
	recorder := do(t, server, http.MethodDelete, path, "", &adminID)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", recorder.Code, recorder.Body)
	}
	if roles.revoked {
		t.Error("the service was called with a role outside the two")
	}
}

func TestListRoles_RendersTheCatalogue(t *testing.T) {
	t.Parallel()
	server := newServer(newFakeRoles())

	recorder := do(t, server, http.MethodGet, "/api/v1/admin/roles", "", &adminID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var body struct {
		Items []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Permissions []string `json:"permissions"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("returned %d roles, want exactly 2", len(body.Items))
	}
	for _, item := range body.Items {
		if item.Permissions == nil {
			t.Errorf("%s has null permissions; the schema requires an array", item.Name)
		}
	}
}
