//go:build contract

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

// This file exists because content had no contract test at all, and the two
// admin reads shipped disagreeing with the schemas they declare: the list left
// out `limit` and `offset`, which AdminContentItemList requires, and the detail
// nested the item under an `item` key that AdminContentItemDetail does not have.
// Both compiled. Every field the detail screen reads — status, kind, and the id
// it posts each state transition to — was undefined in the browser.

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
	media := response.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s %d declares no JSON body", method, path, status)
	}
	return media.Schema.Value
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

func contractItem() domain.Item {
	versionID := uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345602")
	return domain.Item{
		ID:               uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345601"),
		Kind:             "vocab_word",
		Slug:             "environment-n-1",
		CurrentVersionID: &versionID,
		Status:           domain.StatusInReview,
		OwnerID:          uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345603"),
		CreatedAt:        time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 8, 2, 11, 30, 0, 0, time.UTC),
	}
}

func contractVersion() domain.Version {
	return domain.Version{
		ID:        uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345602"),
		ItemID:    uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345601"),
		Version:   2,
		Kind:      "vocab_word",
		Body:      json.RawMessage(`{"word":"environment","definition":"The natural world."}`),
		CEFRLevel: "B1",
		Status:    domain.StatusInReview,
		CreatedAt: time.Date(2026, 8, 2, 11, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 2, 11, 30, 0, 0, time.UTC),
	}
}

func TestContract_AdminContentListMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &mockContentService{
		listAdminItemsFn: func(
			_ context.Context, _, _, _ *string, _, _ int,
		) ([]domain.Item, int64, error) {
			return []domain.Item{contractItem()}, 1, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/content?status=in_review", http.NoBody))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/content", http.MethodGet, http.StatusOK),
		recorder.Body.Bytes())
}

func TestContract_AdminContentDetailMatchesTheSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &mockContentService{
		getAdminItemDetailFn: func(
			_ context.Context, _ uuid.UUID,
		) (domain.Item, []domain.Version, error) {
			return contractItem(), []domain.Version{contractVersion()}, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet, "/admin/content/0199a1c2-3d4e-7f80-9abc-def012345601", http.NoBody))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", recorder.Code, recorder.Body)
	}
	assertMatchesSchema(t,
		responseSchema(t, spec, "/admin/content/{id}", http.MethodGet, http.StatusOK),
		recorder.Body.Bytes())
}
