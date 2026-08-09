//go:build contract

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// These validate real handler responses against api/openapi/openapi.yaml. The
// DTOs in this package are hand-written, so something has to check they still
// match what the spec promises; this is that something. Run with
// `make test-contract`.

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()

	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: true}
	path := filepath.Join("..", "..", "..", "..", "..", "api", "openapi", "openapi.bundle.yaml")
	spec, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	if err := spec.Validate(loader.Context); err != nil {
		t.Fatalf("the spec itself is invalid: %v", err)
	}
	return spec
}

func responseSchema(t *testing.T, spec *openapi3.T, path, method string, status int) *openapi3.Schema {
	t.Helper()

	item := spec.Paths.Find(path)
	if item == nil {
		t.Fatalf("the spec has no path %q", path)
	}
	operation := item.GetOperation(method)
	if operation == nil {
		t.Fatalf("the spec has no %s %s", method, path)
	}
	response := operation.Responses.Status(status)
	if response == nil || response.Value == nil {
		t.Fatalf("%s %s declares no %d response", method, path, status)
	}
	for _, mediaType := range []string{"application/json", "application/problem+json"} {
		if media := response.Value.Content.Get(mediaType); media != nil && media.Schema != nil {
			return media.Schema.Value
		}
	}
	t.Fatalf("%s %s %d declares no JSON body", method, path, status)
	return nil
}

func assertMatchesSchema(t *testing.T, schema *openapi3.Schema, body []byte) {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if err := schema.VisitJSON(decoded); err != nil {
		t.Fatalf("response does not match the published schema: %v\nbody: %s", err, body)
	}
}

func TestContract_MyPermissionsMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(newFakeRoles())

	// A learner: the empty-permissions case, which is the one a `null` bug
	// would slip through.
	learner := do(t, server, http.MethodGet, "/api/v1/me/permissions", "", &learnerID)
	if learner.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", learner.Code, learner.Body)
	}
	schema := responseSchema(t, spec, "/me/permissions", http.MethodGet, http.StatusOK)
	assertMatchesSchema(t, schema, learner.Body.Bytes())

	// And an admin: the full catalogue, which is what checks the name pattern.
	admin := do(t, server, http.MethodGet, "/api/v1/me/permissions", "", &adminID)
	assertMatchesSchema(t, schema, admin.Body.Bytes())
}

func TestContract_RoleListMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(newFakeRoles())

	recorder := do(t, server, http.MethodGet, "/api/v1/admin/roles", "", &adminID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/roles", http.MethodGet, http.StatusOK), recorder.Body.Bytes())
}

func TestContract_RoleChangesMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(newFakeRoles())

	assign := do(t, server, http.MethodPost,
		"/api/v1/admin/users/"+learnerID.String()+"/roles", `{"role":"admin"}`, &adminID)
	if assign.Code != http.StatusOK {
		t.Fatalf("assign status = %d, want 200 (body %s)", assign.Code, assign.Body)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/users/{id}/roles", http.MethodPost, http.StatusOK), assign.Body.Bytes())

	revoke := do(t, server, http.MethodDelete,
		"/api/v1/admin/users/"+learnerID.String()+"/roles/admin", "", &adminID)
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200 (body %s)", revoke.Code, revoke.Body)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/users/{id}/roles/{role}", http.MethodDelete, http.StatusOK),
		revoke.Body.Bytes())
}

// TestContract_ForbiddenMatchesTheSpec checks the refusal path, which is the
// one clients branch on and the one that gets exercised most in production.
func TestContract_ForbiddenMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(newFakeRoles())

	recorder := do(t, server, http.MethodGet, "/api/v1/admin/roles", "", &learnerID)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/roles", http.MethodGet, http.StatusForbidden), recorder.Body.Bytes())
}

// TestContract_AssignRequestBodyIsValid closes the loop: the body the tests
// send must itself be valid against the spec, or the handler could pass every
// test while rejecting what a real client sends.
func TestContract_AssignRequestBodyIsValid(t *testing.T) {
	spec := loadSpec(t)

	operation := spec.Paths.Find("/admin/users/{id}/roles").GetOperation(http.MethodPost)
	schema := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	for _, body := range []string{`{"role":"admin"}`, `{"role":"user"}`} {
		assertMatchesSchema(t, schema, []byte(body))
	}
}
