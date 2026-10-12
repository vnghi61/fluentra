package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/resource/contract"
	"github.com/fluentra/fluentra/internal/modules/resource/domain"
	resourcehttp "github.com/fluentra/fluentra/internal/modules/resource/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type mockResourceService struct {
	createIntentFn func(ctx context.Context, userID uuid.UUID, filename, declaredMIME string) (
		*contract.UploadIntentResult, error,
	)
	confirmFn   func(ctx context.Context, userID, resourceID uuid.UUID) (*contract.SubmitResult, error)
	submitURLFn func(ctx context.Context, userID uuid.UUID, rawURL, title string) (*contract.SubmitResult, error)
	listFn      func(ctx context.Context, userID uuid.UUID, status, kind *string, page, pageSize int) (
		*contract.ResourceList, error,
	)
	getFn        func(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error)
	deleteFn     func(ctx context.Context, id, userID uuid.UUID) error
	revalidateFn func(ctx context.Context, userID, resourceID uuid.UUID) error
}

func (m *mockResourceService) CreateUploadIntent(
	ctx context.Context, userID uuid.UUID, filename, declaredMIME string,
) (*contract.UploadIntentResult, error) {
	if m.createIntentFn != nil {
		return m.createIntentFn(ctx, userID, filename, declaredMIME)
	}
	return nil, nil
}

func (m *mockResourceService) ConfirmUpload(
	ctx context.Context, userID, resourceID uuid.UUID,
) (*contract.SubmitResult, error) {
	if m.confirmFn != nil {
		return m.confirmFn(ctx, userID, resourceID)
	}
	return nil, nil
}

func (m *mockResourceService) SubmitURL(
	ctx context.Context, userID uuid.UUID, rawURL, title string,
) (*contract.SubmitResult, error) {
	if m.submitURLFn != nil {
		return m.submitURLFn(ctx, userID, rawURL, title)
	}
	return nil, nil
}

func (m *mockResourceService) ListResources(
	ctx context.Context, userID uuid.UUID, status, kind *string, page, pageSize int,
) (*contract.ResourceList, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID, status, kind, page, pageSize)
	}
	return &contract.ResourceList{}, nil
}

func (m *mockResourceService) GetResource(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id, userID)
	}
	return nil, domain.ErrResourceNotFound
}

func (m *mockResourceService) DeleteResource(ctx context.Context, id, userID uuid.UUID) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id, userID)
	}
	return nil
}

func (m *mockResourceService) RevalidateResource(ctx context.Context, userID, resourceID uuid.UUID) error {
	if m.revalidateFn != nil {
		return m.revalidateFn(ctx, userID, resourceID)
	}
	return nil
}

func setupTestRouter(svc resourcehttp.ResourceService) *chi.Mux {
	r := chi.NewRouter()
	handler := resourcehttp.NewHandler(svc)
	handler.Routes(r)
	return r
}

func withActor(req *http.Request, userID uuid.UUID) *http.Request {
	ctx := httpx.WithActor(req.Context(), httpx.Actor{
		UserID: userID,
	})
	return req.WithContext(ctx)
}

func TestHandler_UploadIntent(t *testing.T) {
	userID := uuid.New()
	resourceID := uuid.New()

	svc := &mockResourceService{
		createIntentFn: func(_ context.Context, uID uuid.UUID, _, _ string) (
			*contract.UploadIntentResult, error,
		) {
			if uID != userID {
				t.Errorf("expected userID %v, got %v", userID, uID)
			}
			return &contract.UploadIntentResult{
				ID:        resourceID,
				UploadURL: "https://storage.local/upload",
				ObjectKey: "user/key.pdf",
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
	}

	router := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]string{
		"filename":     "test.pdf",
		"content_type": "application/pdf",
	})
	req := httptest.NewRequest(http.MethodPost, "/me/resources/upload-intent", bytes.NewReader(body))
	req = withActor(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Submit_UnownedReturns404(t *testing.T) {
	userID := uuid.New()
	targetID := uuid.New()

	svc := &mockResourceService{
		confirmFn: func(_ context.Context, _, _ uuid.UUID) (*contract.SubmitResult, error) {
			return nil, domain.ErrResourceNotFound
		},
	}

	router := setupTestRouter(svc)

	body, _ := json.Marshal(map[string]any{
		"resource_id": targetID,
	})
	req := httptest.NewRequest(http.MethodPost, "/me/resources", bytes.NewReader(body))
	req = withActor(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for unowned resource, got %d", w.Code)
	}
}

func TestHandler_Get_UnownedReturns404(t *testing.T) {
	userID := uuid.New()
	targetID := uuid.New()

	svc := &mockResourceService{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*contract.Resource, error) {
			return nil, domain.ErrResourceNotFound
		},
	}

	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/me/resources/"+targetID.String(), nil)
	req = withActor(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for unowned resource, got %d", w.Code)
	}
}

func TestHandler_Delete_UnownedReturns404(t *testing.T) {
	userID := uuid.New()
	targetID := uuid.New()

	svc := &mockResourceService{
		deleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			return domain.ErrResourceNotFound
		},
	}

	router := setupTestRouter(svc)

	req := httptest.NewRequest(http.MethodDelete, "/me/resources/"+targetID.String(), nil)
	req = withActor(req, userID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for delete unowned resource, got %d", w.Code)
	}
}

func TestHandler_ListAndDetailAgreeOnProcessingStatus(t *testing.T) {
	t.Parallel()
	const (
		processing = "processing"
		ready      = "ready"
		failed     = "failed"
	)
	for _, tc := range []struct {
		name     string
		statuses []string
		want     string
	}{
		{"no renditions", nil, processing},
		{"pending", []string{domain.RenditionStatusPending}, processing},
		{"ready before pending", []string{domain.RenditionStatusReady, domain.RenditionStatusPending}, processing},
		{"pending before ready", []string{domain.RenditionStatusPending, domain.RenditionStatusReady}, processing},
		{"completed preview", []string{domain.RenditionStatusReady}, ready},
		{"ready with skipped", []string{domain.RenditionStatusReady, domain.RenditionStatusSkipped}, ready},
		{"ready with failed", []string{domain.RenditionStatusFailed, domain.RenditionStatusReady}, ready},
		{"all failed", []string{domain.RenditionStatusFailed}, failed},
		{"all skipped", []string{domain.RenditionStatusSkipped}, failed},
		{"failed and skipped", []string{domain.RenditionStatusFailed, domain.RenditionStatusSkipped}, failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID, resourceID := uuid.New(), uuid.New()
			resource := contract.Resource{ID: resourceID, UserID: userID, Kind: domain.KindFile, Status: domain.StatusValidated}
			for _, status := range tc.statuses {
				resource.Renditions = append(resource.Renditions, contract.Rendition{
					ResourceID: resourceID, Status: status, Kind: domain.RenditionKindPreview,
				})
			}
			readyCount := readyRenditionCount(resource.Renditions)
			svc := &mockResourceService{
				listFn: func(_ context.Context, id uuid.UUID, _, _ *string, _, _ int) (*contract.ResourceList, error) {
					if id != userID {
						t.Fatal("list lost the authenticated user")
					}
					return &contract.ResourceList{Items: []contract.Resource{resource}, Total: 1, Page: 1, PageSize: 20}, nil
				},
				getFn: func(_ context.Context, id, owner uuid.UUID) (*contract.Resource, error) {
					if id != resourceID || owner != userID {
						t.Fatal("detail lost the authenticated user or resource ID")
					}
					return &resource, nil
				},
			}
			router := setupTestRouter(svc)
			type resourceStatus struct {
				ID               uuid.UUID         `json:"id"`
				ProcessingStatus string            `json:"processing_status"`
				Renditions       []json.RawMessage `json:"renditions"`
			}
			for _, path := range []string{"/me/resources", "/me/resources/" + resourceID.String()} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, withActor(httptest.NewRequest(http.MethodGet, path, nil), userID))
				if w.Code != http.StatusOK {
					t.Fatalf("GET %s: status=%d, body=%s", path, w.Code, w.Body.String())
				}
				var got resourceStatus
				if path == "/me/resources" {
					var list struct {
						Items []resourceStatus `json:"items"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
						t.Fatal(err)
					}
					if len(list.Items) != 1 {
						t.Fatalf("list returned %d resources, want 1", len(list.Items))
					}
					got = list.Items[0]
				} else if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.ID != resourceID || got.ProcessingStatus != tc.want || len(got.Renditions) != readyCount {
					t.Errorf("GET %s: got %+v, want status=%s and %d ready renditions", path, got, tc.want, readyCount)
				}
			}
		})
	}
}

func readyRenditionCount(renditions []contract.Rendition) int {
	count := 0
	for _, rendition := range renditions {
		if rendition.Status == domain.RenditionStatusReady {
			count++
		}
	}
	return count
}

func TestHandler_RevalidateAuthenticatesAndScopesTheRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		authenticated bool
		validID       bool
		serviceErr    error
		wantStatus    int
		wantCode      string
		wantCalls     int
	}{
		{"owner accepted", true, true, nil, http.StatusAccepted, "", 1},
		{"foreign owner", true, true, domain.ErrResourceNotFound, http.StatusNotFound, "RESOURCE_NOT_FOUND", 1},
		{"no authentication", false, true, nil, http.StatusUnauthorized, "UNAUTHORIZED", 0},
		{"invalid resource ID", true, false, nil, http.StatusBadRequest, "INVALID_RESOURCE_ID", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID, resourceID := uuid.New(), uuid.New()
			calls := 0
			svc := &mockResourceService{revalidateFn: func(_ context.Context, owner, id uuid.UUID) error {
				calls++
				if owner != userID || id != resourceID {
					t.Fatalf("revalidation IDs = %s/%s, want %s/%s", owner, id, userID, resourceID)
				}
				return tc.serviceErr
			}}
			id := resourceID.String()
			if !tc.validID {
				id = "invalid"
			}
			req := httptest.NewRequest(http.MethodPost, "/me/resources/"+id+"/revalidate", nil)
			if tc.authenticated {
				req = withActor(req, userID)
			}
			w := httptest.NewRecorder()
			setupTestRouter(svc).ServeHTTP(w, req)
			if w.Code != tc.wantStatus || calls != tc.wantCalls {
				t.Fatalf("status=%d, calls=%d; want %d/%d: %s", w.Code, calls, tc.wantStatus, tc.wantCalls, w.Body.String())
			}
			if tc.wantCode != "" {
				var problem apperr.Problem
				if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != tc.wantCode || problem.Status != tc.wantStatus {
					t.Errorf("problem = %+v, want code %s", problem, tc.wantCode)
				}
			}
		})
	}
}
