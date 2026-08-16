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

// The contract tests answer one question the unit tests cannot: does what this
// handler actually writes match what api/openapi/openapi.yaml promises?
//
// The DTOs in this package are hand-written rather than taken from the
// generated models, because a business module importing api/openapi would
// couple it to every other module's spec. That choice is only safe if
// something checks the two agree — this is that something. Run with
// `make test-contract`.

// loadSpec reads the bundled spec, which is the same artefact the code
// generators consume, so a test passing here and a client generated from the
// spec cannot disagree.
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

// responseSchema pulls the 200 schema for one operation out of the spec.
func responseSchema(t *testing.T, spec *openapi3.T, path, method string) *openapi3.Schema {
	t.Helper()

	item := spec.Paths.Find(path)
	if item == nil {
		t.Fatalf("the spec has no path %q", path)
	}
	operation := item.GetOperation(method)
	if operation == nil {
		t.Fatalf("the spec has no %s %s", method, path)
	}
	response := operation.Responses.Status(http.StatusOK)
	if response == nil || response.Value == nil {
		t.Fatalf("%s %s declares no 200 response", method, path)
	}
	media := response.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s declares no application/json 200 body", method, path)
	}
	return media.Schema.Value
}

// assertMatchesSchema validates a real response body against the published
// schema, including the parts a Go struct cannot express: required fields,
// enum members, string patterns and numeric bounds.
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

func TestContract_GetMeMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	recorder := authenticated(t, server, http.MethodGet, "/api/v1/me", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me", http.MethodGet), recorder.Body.Bytes())
}

func TestContract_PatchMeMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"display_name":"Nghi Nguyen"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me", http.MethodPatch), recorder.Body.Bytes())
}

func TestContract_PreferencesMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	read := authenticated(t, server, http.MethodGet, "/api/v1/me/preferences", "")
	if read.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body %s)", read.Code, read.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/preferences", http.MethodGet), read.Body.Bytes())

	written := authenticated(t, server, http.MethodPut, "/api/v1/me/preferences", validPreferencesBody)
	if written.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (body %s)", written.Code, written.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/preferences", http.MethodPut), written.Body.Bytes())
}

func TestContract_AvatarEndpointsMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	intent := authenticated(t, server, http.MethodPost, "/api/v1/me/avatar/upload-intent", `{"content_type":"image/png"}`)
	if intent.Code != http.StatusOK {
		t.Fatalf("POST upload-intent status = %d, want 200 (body %s)", intent.Code, intent.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/avatar/upload-intent", http.MethodPost), intent.Body.Bytes())

	confirm := authenticated(t, server, http.MethodPut, "/api/v1/me/avatar", `{"object_key":"users/0199a1c2-3d4e-7f80-9abc-def012345678/2026/08/avatar-raw.jpg"}`)
	if confirm.Code != http.StatusOK {
		t.Fatalf("PUT avatar status = %d, want 200 (body %s)", confirm.Code, confirm.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/avatar", http.MethodPut), confirm.Body.Bytes())
}

// TestContract_RequestBodiesUsedByTheTestsAreValid closes the other half of the
// loop. If the bodies these tests send were not themselves valid against the
// spec, a handler could pass every test above while rejecting everything a
// real client sends.
func TestContract_RequestBodiesUsedByTheTestsAreValid(t *testing.T) {
	spec := loadSpec(t)

	cases := []struct {
		path   string
		method string
		body   string
	}{
		{path: "/me", method: http.MethodPatch, body: `{"display_name":"Nghi Nguyen"}`},
		{path: "/me", method: http.MethodPatch, body: `{"timezone":"Asia/Ho_Chi_Minh"}`},
		{path: "/me", method: http.MethodPatch, body: `{"date_of_birth":"1998-03-04"}`},
		{path: "/me/preferences", method: http.MethodPut, body: validPreferencesBody},
		{path: "/me/avatar", method: http.MethodPut, body: `{"object_key":"users/0199a1c2-3d4e-7f80-9abc-def012345678/2026/08/avatar-raw.jpg"}`},
	}
	for _, testCase := range cases {
		item := spec.Paths.Find(testCase.path)
		operation := item.GetOperation(testCase.method)
		schema := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
		assertMatchesSchema(t, schema, []byte(testCase.body))
	}
}

// TestContract_ProblemResponsesMatchTheSpec checks the failure path too. An
// error shape that drifts is worse than a success shape that does: clients
// branch on `code`, and they only find out it changed in production.
func TestContract_ProblemResponsesMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	item := spec.Paths.Find("/me")
	response := item.GetOperation(http.MethodPatch).Responses.Status(http.StatusUnprocessableEntity)
	schema := response.Value.Content.Get("application/problem+json").Schema.Value

	recorder := authenticated(t, server, http.MethodPatch, "/api/v1/me", `{"nope":1}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", recorder.Code)
	}
	assertMatchesSchema(t, schema, recorder.Body.Bytes())
}

func TestContract_ExportResponsesMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	postReq := authenticated(t, server, http.MethodPost, "/api/v1/me/export", "")
	if postReq.Code != http.StatusAccepted {
		t.Fatalf("POST /me/export status = %d, want 202 (body %s)", postReq.Code, postReq.Body)
	}
	item := spec.Paths.Find("/me/export")
	operation := item.GetOperation(http.MethodPost)
	response := operation.Responses.Status(http.StatusAccepted)
	assertMatchesSchema(t, response.Value.Content.Get("application/json").Schema.Value, postReq.Body.Bytes())

	getReq := authenticated(t, server, http.MethodGet, "/api/v1/me/export/0199a1c2-3d4e-7f80-9abc-def999999999", "")
	if getReq.Code != http.StatusOK {
		t.Fatalf("GET /me/export/{id} status = %d, want 200 (body %s)", getReq.Code, getReq.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/export/{id}", http.MethodGet), getReq.Body.Bytes())
}

func TestContract_DeletionResponsesMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	server := newServer(&fakeAccounts{})

	// 1. DELETE /me -> 202
	deleteReq := authenticated(t, server, http.MethodDelete, "/api/v1/me", "")
	if deleteReq.Code != http.StatusAccepted {
		t.Fatalf("DELETE /me status = %d, want 202 (body %s)", deleteReq.Code, deleteReq.Body)
	}
	item := spec.Paths.Find("/me")
	operation := item.GetOperation(http.MethodDelete)
	response := operation.Responses.Status(http.StatusAccepted)
	assertMatchesSchema(t, response.Value.Content.Get("application/json").Schema.Value, deleteReq.Body.Bytes())

	// 2. POST /me/deletion/cancel -> 200
	cancelReq := authenticated(t, server, http.MethodPost, "/api/v1/me/deletion/cancel", "")
	if cancelReq.Code != http.StatusOK {
		t.Fatalf("POST /me/deletion/cancel status = %d, want 200 (body %s)", cancelReq.Code, cancelReq.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/deletion/cancel", http.MethodPost), cancelReq.Body.Bytes())

	// 3. GET /me/deletion/{id} -> 200
	getReq := authenticated(t, server, http.MethodGet, "/api/v1/me/deletion/0199a1c2-3d4e-7f80-9abc-def888888888", "")
	if getReq.Code != http.StatusOK {
		t.Fatalf("GET /me/deletion/{id} status = %d, want 200 (body %s)", getReq.Code, getReq.Body)
	}
	assertMatchesSchema(t, responseSchema(t, spec, "/me/deletion/{id}", http.MethodGet), getReq.Body.Bytes())
}
