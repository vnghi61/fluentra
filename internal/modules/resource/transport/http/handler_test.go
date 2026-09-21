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
	getFn    func(ctx context.Context, id, userID uuid.UUID) (*contract.Resource, error)
	deleteFn func(ctx context.Context, id, userID uuid.UUID) error
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
