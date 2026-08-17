//go:build contract

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	admincontract "github.com/fluentra/fluentra/internal/modules/admin/contract"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

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

func TestContract_AdminUserSearchMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/users", http.MethodGet, http.StatusOK)
	actor := &httpx.Actor{UserID: testAdminID, Role: "admin"}

	// 1. Populated page
	svc := &fakeAdminService{
		searchResp: []usercontract.UserSummary{{
			ID:          testTargetID,
			Email:       "learner@example.com",
			DisplayName: "Nghi",
			Status:      "active",
			CreatedAt:   time.Date(2026, 8, 1, 9, 15, 0, 0, time.UTC),
		}},
	}
	server := newServer(svc, newFakeGuard("user.list"))
	rec := doRequest(server, http.MethodGet, "/admin/users", "", actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	assertMatchesSchema(t, schema, rec.Body.Bytes())

	// 2. Empty page (items = [])
	emptySvc := &fakeAdminService{
		searchResp: []usercontract.UserSummary{},
	}
	recEmpty := doRequest(newServer(emptySvc, newFakeGuard("user.list")), http.MethodGet, "/admin/users", "", actor)
	assertMatchesSchema(t, schema, recEmpty.Body.Bytes())

	// 3. Page with next_cursor
	pagedSvc := &fakeAdminService{
		searchResp: []usercontract.UserSummary{{
			ID:          testTargetID,
			Email:       "learner@example.com",
			DisplayName: "Nghi",
			Status:      "active",
			CreatedAt:   time.Date(2026, 8, 1, 9, 15, 0, 0, time.UTC),
		}},
		searchCursor: "next-cursor-token",
	}
	recPaged := doRequest(newServer(pagedSvc, newFakeGuard("user.list")), http.MethodGet, "/admin/users", "", actor)
	assertMatchesSchema(t, schema, recPaged.Body.Bytes())
}

func TestContract_AdminUserDetailMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/users/{id}", http.MethodGet, http.StatusOK)
	actor := &httpx.Actor{UserID: testAdminID, Role: "admin"}

	svc := &fakeAdminService{
		getUserResp: &usercontract.UserDetail{
			ID:          testTargetID,
			Email:       "learner@example.com",
			DisplayName: "Nghi",
			Locale:      "vi-VN",
			Timezone:    "Asia/Ho_Chi_Minh",
			Status:      "active",
			CreatedAt:   time.Date(2026, 8, 1, 9, 15, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 8, 14, 2, 30, 0, 0, time.UTC),
		},
	}
	server := newServer(svc, newFakeGuard("user.read"))
	rec := doRequest(server, http.MethodGet, "/admin/users/"+testTargetID.String(), "", actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_SuspendUserMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/users/{id}/suspend", http.MethodPost, http.StatusOK)
	actor := &httpx.Actor{UserID: testAdminID, Role: "admin"}

	svc := &fakeAdminService{}
	server := newServer(svc, newFakeGuard("user.suspend"))
	body := `{"reason":"Violating community guidelines multiple times"}`
	rec := doRequest(server, http.MethodPost, "/admin/users/"+testTargetID.String()+"/suspend", body, actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_RevokeSessionsMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/users/{id}/sessions/revoke", http.MethodPost, http.StatusOK)
	actor := &httpx.Actor{UserID: testAdminID, Role: "admin"}

	svc := &fakeAdminService{}
	server := newServer(svc, newFakeGuard("user.manage_sessions"))
	body := `{"reason":"Security precaution following compromise report"}`
	rec := doRequest(server, http.MethodPost, "/admin/users/"+testTargetID.String()+"/sessions/revoke", body, actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_ListFlagsMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/flags", http.MethodGet, http.StatusOK)
	actor := &httpx.Actor{UserID: testAdminID, Role: "admin"}

	// Populated list
	svc := &fakeAdminService{
		listFlagsResp: []admincontract.FeatureFlag{{
			Key:            "streaks_v2",
			Enabled:        true,
			RolloutPercent: 25,
			Owner:          "@backend-team",
			ExpiresOn:      "2026-12-31",
			Description:    "Second-generation streak calculation.",
			CreatedAt:      "2026-08-01T09:15:00Z",
			UpdatedAt:      "2026-08-01T09:15:00Z",
		}},
	}
	rec := doRequest(newServer(svc, newFakeGuard("system.flags")), http.MethodGet, "/admin/flags", "", actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	assertMatchesSchema(t, schema, rec.Body.Bytes())

	// Empty list
	emptySvc := &fakeAdminService{listFlagsResp: []admincontract.FeatureFlag{}}
	recEmpty := doRequest(newServer(emptySvc, newFakeGuard("system.flags")), http.MethodGet, "/admin/flags", "", actor)
	assertMatchesSchema(t, schema, recEmpty.Body.Bytes())
}

func TestContract_CreateFlagMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	schema := responseSchema(t, spec, "/admin/flags", http.MethodPost, http.StatusCreated)
	actor := &httpx.Actor{UserID: testAdminID, Role: "admin"}

	svc := &fakeAdminService{
		flagResp: admincontract.FeatureFlag{
			Key:            "streaks_v2",
			Enabled:        true,
			RolloutPercent: 25,
			Owner:          "@backend-team",
			ExpiresOn:      "2026-12-31",
			Description:    "Second-generation streak calculation.",
			CreatedAt:      "2026-08-01T09:15:00Z",
			UpdatedAt:      "2026-08-01T09:15:00Z",
		},
	}
	body := `{
		"key": "streaks_v2",
		"enabled": true,
		"rollout_percent": 25,
		"owner": "@backend-team",
		"expires_on": "2026-12-31",
		"description": "Second-generation streak calculation."
	}`
	rec := doRequest(newServer(svc, newFakeGuard("system.flags")), http.MethodPost, "/admin/flags", body, actor)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}
