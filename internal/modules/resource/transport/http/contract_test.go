//go:build contract

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	resourcehttp "github.com/fluentra/fluentra/internal/modules/resource/transport/http"
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

func operationResponseSchema(t *testing.T, spec *openapi3.T, path, method string, status int) *openapi3.Schema {
	t.Helper()

	item := spec.Paths.Find(path)
	if item == nil {
		t.Fatalf("the spec has no path %q", path)
	}
	op := item.GetOperation(method)
	if op == nil {
		t.Fatalf("the spec has no %s %s", method, path)
	}
	resp := op.Responses.Status(status)
	if resp == nil || resp.Value == nil {
		t.Fatalf("%s %s declares no %d response", method, path, status)
	}
	media := resp.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s declares no application/json %d body", method, path, status)
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
		t.Fatalf("response does not match schema: %v\nbody: %s", err, body)
	}
}

func newTestServer(svc resourcehttp.ResourceService) http.Handler {
	r := chi.NewRouter()
	handler := resourcehttp.NewHandler(svc)
	r.Route("/api/v1", func(sub chi.Router) {
		handler.Routes(sub)
	})
	return r
}

func authReq(t *testing.T, server http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	actor := httpx.Actor{
		UserID: uuid.MustParse("018f3a5e-7b82-7d2c-80a2-bf3d6118d531"),
		Role:   "user",
	}
	ctx := httpx.WithActor(req.Context(), actor)
	ctx = httpx.WithRequestID(ctx, "01J8XQ7Z9K3M4N5P6Q7R8S9T0V")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func sampleResource() contract.Resource {
	key := "user/018f3a5e-7b82-7d2c-80a2-bf3d6118d531/2026/09/sample.pdf"
	size := int64(2458120)
	sum := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	url := "https://storage.local/download/sample.pdf"
	rendURL := "https://storage.local/fluentra-derived/renditions/sample/thumbnail.png"
	w := 320
	h := 240
	now := time.Now().UTC()
	exp := now.Add(1 * time.Hour)
	rSize := int64(12450)
	return contract.Resource{
		ID:               uuid.MustParse("018f3a5e-7b82-7d2c-80a2-bf3d6118d531"),
		UserID:           uuid.MustParse("018f3a5e-7b82-7d2c-80a2-bf3d6118d531"),
		Kind:             domain.KindFile,
		Title:            "Advanced Grammar Guide.pdf",
		ObjectKey:        &key,
		OriginalFilename: "Advanced Grammar Guide.pdf",
		DeclaredMIME:     "application/pdf",
		DetectedMIME:     "application/pdf",
		ByteSize:         &size,
		Checksum:         &sum,
		DownloadURL:      &url,
		Status:           domain.StatusValidated,
		FailureReason:    "",
		CreatedAt:        now,
		UpdatedAt:        now,
		ValidatedAt:      &now,
		Renditions: []contract.Rendition{
			{
				Kind:        domain.RenditionKindThumbnail,
				Status:      domain.RenditionStatusReady,
				MIMEType:    "image/png",
				Width:       &w,
				Height:      &h,
				ByteSize:    &rSize,
				URL:         &rendURL,
				ExpiresAt:   &exp,
				ToolVersion: "pure-go",
			},
		},
	}
}

func TestContract_UploadIntentMatchesSpec(t *testing.T) {
	spec := loadSpec(t)
	resID := uuid.MustParse("018f3a5e-7b82-7d2c-80a2-bf3d6118d531")
	svc := &mockResourceService{
		createIntentFn: func(_ context.Context, _ uuid.UUID, _, _ string) (
			*contract.UploadIntentResult, error,
		) {
			return &contract.UploadIntentResult{
				ID:        resID,
				UploadURL: "https://storage.local/upload",
				ObjectKey: "user/key.pdf",
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
	}

	server := newTestServer(svc)
	rec := authReq(
		t, server, http.MethodPost, "/api/v1/me/resources/upload-intent",
		`{"filename":"guide.pdf","content_type":"application/pdf"}`,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	schema := operationResponseSchema(t, spec, "/me/resources/upload-intent", http.MethodPost, http.StatusOK)
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_SubmitResourceMatchesSpec(t *testing.T) {
	spec := loadSpec(t)
	resID := uuid.MustParse("018f3a5e-7b82-7d2c-80a2-bf3d6118d531")
	svc := &mockResourceService{
		confirmFn: func(_ context.Context, _, _ uuid.UUID) (*contract.SubmitResult, error) {
			return &contract.SubmitResult{
				ID:     resID,
				Status: domain.StatusUploaded,
			}, nil
		},
	}

	server := newTestServer(svc)
	rec := authReq(
		t, server, http.MethodPost, "/api/v1/me/resources",
		`{"resource_id":"018f3a5e-7b82-7d2c-80a2-bf3d6118d531"}`,
	)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %s)", rec.Code, rec.Body)
	}

	schema := operationResponseSchema(t, spec, "/me/resources", http.MethodPost, http.StatusAccepted)
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_ListResourcesMatchesSpec(t *testing.T) {
	spec := loadSpec(t)
	res := sampleResource()
	svc := &mockResourceService{
		listFn: func(_ context.Context, _ uuid.UUID, _, _ *string, _, _ int) (*contract.ResourceList, error) {
			return &contract.ResourceList{
				Items:    []contract.Resource{res},
				Total:    1,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}

	server := newTestServer(svc)
	rec := authReq(t, server, http.MethodGet, "/api/v1/me/resources", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	schema := operationResponseSchema(t, spec, "/me/resources", http.MethodGet, http.StatusOK)
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_GetResourceMatchesSpec(t *testing.T) {
	spec := loadSpec(t)
	res := sampleResource()
	svc := &mockResourceService{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*contract.Resource, error) {
			return &res, nil
		},
	}

	server := newTestServer(svc)
	rec := authReq(t, server, http.MethodGet, "/api/v1/me/resources/018f3a5e-7b82-7d2c-80a2-bf3d6118d531", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}

	schema := operationResponseSchema(t, spec, "/me/resources/{id}", http.MethodGet, http.StatusOK)
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}

func TestContract_DeleteResourceMatchesSpec(t *testing.T) {
	svc := &mockResourceService{
		deleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return nil
		},
	}

	server := newTestServer(svc)
	rec := authReq(t, server, http.MethodDelete, "/api/v1/me/resources/018f3a5e-7b82-7d2c-80a2-bf3d6118d531", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
}

func TestContract_RequestBodiesMatchSpec(t *testing.T) {
	spec := loadSpec(t)

	cases := []struct {
		path   string
		method string
		body   string
	}{
		{
			path:   "/me/resources/upload-intent",
			method: http.MethodPost,
			body:   `{"filename":"test.pdf","content_type":"application/pdf"}`,
		},
		{
			path:   "/me/resources",
			method: http.MethodPost,
			body:   `{"resource_id":"018f3a5e-7b82-7d2c-80a2-bf3d6118d531"}`,
		},
		{
			path:   "/me/resources",
			method: http.MethodPost,
			body:   `{"url":"https://example.com/article","title":"An Article"}`,
		},
	}

	for _, tc := range cases {
		item := spec.Paths.Find(tc.path)
		if item == nil {
			t.Fatalf("path %q not in spec", tc.path)
		}
		op := item.GetOperation(tc.method)
		if op == nil {
			t.Fatalf("method %s not in %s", tc.method, tc.path)
		}
		schema := op.RequestBody.Value.Content.Get("application/json").Schema.Value
		assertMatchesSchema(t, schema, []byte(tc.body))
	}
}

func TestContract_ProblemResponsesMatchSpec(t *testing.T) {
	spec := loadSpec(t)
	svc := &mockResourceService{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*contract.Resource, error) {
			return nil, domain.ErrResourceNotFound
		},
	}

	server := newTestServer(svc)
	rec := authReq(t, server, http.MethodGet, "/api/v1/me/resources/018f3a5e-7b82-7d2c-80a2-bf3d6118d531", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body)
	}

	item := spec.Paths.Find("/me/resources/{id}")
	resp := item.GetOperation(http.MethodGet).Responses.Status(http.StatusNotFound)
	schema := resp.Value.Content.Get("application/problem+json").Schema.Value
	assertMatchesSchema(t, schema, rec.Body.Bytes())
}
