//go:build contract

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
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

func TestContract_AuditLogSearchMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/audit-logs", http.MethodGet, http.StatusOK)

	// A populated page: the case that checks every field's type and pattern.
	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead))
	populated := do(t, server, http.MethodGet, pathAuditLogs, "", &adminID)
	if populated.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", populated.Code, populated.Body)
	}
	assertMatchesSchema(t, schema, populated.Body.Bytes())

	// And an empty one, which is where a `null` instead of `[]` would slip
	// through — required arrays are the field clients crash on.
	emptyTrail := newFakeTrail()
	emptyTrail.logs = nil
	empty := do(t, newServer(emptyTrail, newFakeGuard(contract.PermissionRead)),
		http.MethodGet, pathAuditLogs, "", &adminID)
	assertMatchesSchema(t, schema, empty.Body.Bytes())

	// And a page that has a successor, so next_cursor is exercised.
	pagedTrail := newFakeTrail()
	pagedTrail.hasMore = true
	paged := do(t, newServer(pagedTrail, newFakeGuard(contract.PermissionRead)),
		http.MethodGet, pathAuditLogs+"?limit=1", "", &adminID)
	assertMatchesSchema(t, schema, paged.Body.Bytes())
}

func TestContract_SecurityEventSearchMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/security-events", http.MethodGet, http.StatusOK)

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead))
	recorder := do(t, server, http.MethodGet, pathSecurityEvents, "", &adminID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t, schema, recorder.Body.Bytes())

	emptyTrail := newFakeTrail()
	emptyTrail.events = nil
	empty := do(t, newServer(emptyTrail, newFakeGuard(contract.PermissionRead)),
		http.MethodGet, pathSecurityEvents, "", &adminID)
	assertMatchesSchema(t, schema, empty.Body.Bytes())
}

func TestContract_ResolveMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionManage))
	recorder := do(t, server, http.MethodPost,
		pathSecurityEvents+"/"+openEvent.String()+"/resolve",
		`{"note":"Known load test, permission denial expected."}`, &adminID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/security-events/{id}/resolve", http.MethodPost, http.StatusOK),
		recorder.Body.Bytes())
}

// TestContract_ForbiddenMatchesTheSpec checks the refusal path, which is the
// one clients branch on and the one an unauthorised caller sees most.
func TestContract_ForbiddenMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)

	server := newServer(newFakeTrail(), newFakeGuard())
	recorder := do(t, server, http.MethodGet, pathAuditLogs, "", &adminID)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/audit-logs", http.MethodGet, http.StatusForbidden),
		recorder.Body.Bytes())
}

// TestContract_ResolveRequestBodyIsValid closes the loop: the body these tests
// send must itself be valid against the spec, or the handler could pass every
// test while rejecting what a real client sends.
func TestContract_ResolveRequestBodyIsValid(t *testing.T) {
	spec := loadSpec(t)

	operation := spec.Paths.Find("/admin/security-events/{id}/resolve").GetOperation(http.MethodPost)
	schema := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	assertMatchesSchema(t, schema, []byte(`{"note":"Known load test, permission denial expected."}`))
}

// TestContract_SearchParametersAreDeclared guards against a filter the handler
// reads and the spec never mentions — a query string that works in production
// and is invisible to every generated client.
func TestContract_SearchParametersAreDeclared(t *testing.T) {
	spec := loadSpec(t)

	expected := map[string][]string{
		"/admin/audit-logs": {
			"actor_id", "action", "target_type", "target_id", "from", "to", "cursor", "limit",
		},
		"/admin/security-events": {
			"kind", "severity", "resolved", "user_id", "from", "to", "cursor", "limit",
		},
	}
	for path, names := range expected {
		operation := spec.Paths.Find(path).GetOperation(http.MethodGet)
		declared := make(map[string]struct{}, len(operation.Parameters))
		for _, parameter := range operation.Parameters {
			declared[parameter.Value.Name] = struct{}{}
		}
		for _, name := range names {
			if _, found := declared[name]; !found {
				t.Errorf("%s reads %q but the spec does not declare it", path, name)
			}
		}
	}
}
