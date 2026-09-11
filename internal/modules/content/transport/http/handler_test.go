package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	contenthttp "github.com/fluentra/fluentra/internal/modules/content/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	testKindVocabWord = "vocab_word"
	roleAdmin         = "admin"
	statusPublished   = "published"
)

type mockContentService struct {
	getPublishedSlugFn func(ctx context.Context, slug string) (*contract.Version, error)
	browseFn           func(ctx context.Context, filter contract.BrowseFilter) ([]*contract.Version, int, error)
	createItemFn       func(
		ctx context.Context,
		actorID uuid.UUID,
		req service.CreateItemRequest,
	) (domain.Item, domain.Version, error)
	updateDraftFn func(
		ctx context.Context,
		actorID, itemID uuid.UUID,
		req service.UpdateDraftRequest,
	) (domain.Version, error)
	submitFn func(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error)
	reviewFn func(
		ctx context.Context,
		reviewerID, itemID uuid.UUID,
		req service.ReviewDecisionRequest,
	) (domain.Version, error)
	publishFn        func(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error)
	archiveFn        func(ctx context.Context, actorID, itemID uuid.UUID) (domain.Item, error)
	listAdminItemsFn func(
		ctx context.Context, status, kind, query *string, limit, offset int,
	) ([]domain.Item, int64, error)
	getAdminItemDetailFn func(
		ctx context.Context, id uuid.UUID,
	) (domain.Item, []domain.Version, error)
	reportItemFn func(
		ctx context.Context, userID, versionID uuid.UUID, reason domain.ReportReason, note *string,
	) (domain.ItemReport, error)
	listReportedContentFn func(
		ctx context.Context, limit, offset int,
	) ([]domain.ReportedVersionSummary, int, error)
}

func (m *mockContentService) GetPublishedVersionBySlug(ctx context.Context, slug string) (*contract.Version, error) {
	if m.getPublishedSlugFn != nil {
		return m.getPublishedSlugFn(ctx, slug)
	}
	return nil, domain.ErrContentNotPublished
}

func (m *mockContentService) Browse(
	ctx context.Context,
	filter contract.BrowseFilter,
) ([]*contract.Version, int, error) {
	if m.browseFn != nil {
		return m.browseFn(ctx, filter)
	}
	return []*contract.Version{}, 0, nil
}

func (m *mockContentService) CreateItem(
	ctx context.Context,
	actorID uuid.UUID,
	req service.CreateItemRequest,
) (domain.Item, domain.Version, error) {
	if m.createItemFn != nil {
		return m.createItemFn(ctx, actorID, req)
	}
	return domain.Item{}, domain.Version{}, nil
}

func (m *mockContentService) UpdateDraft(
	ctx context.Context,
	actorID, itemID uuid.UUID,
	req service.UpdateDraftRequest,
) (domain.Version, error) {
	if m.updateDraftFn != nil {
		return m.updateDraftFn(ctx, actorID, itemID, req)
	}
	return domain.Version{}, nil
}

func (m *mockContentService) SubmitForReview(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error) {
	if m.submitFn != nil {
		return m.submitFn(ctx, actorID, itemID)
	}
	return domain.Version{}, nil
}

func (m *mockContentService) Review(
	ctx context.Context,
	reviewerID, itemID uuid.UUID,
	req service.ReviewDecisionRequest,
) (domain.Version, error) {
	if m.reviewFn != nil {
		return m.reviewFn(ctx, reviewerID, itemID, req)
	}
	return domain.Version{}, nil
}

func (m *mockContentService) Publish(ctx context.Context, actorID, itemID uuid.UUID) (domain.Version, error) {
	if m.publishFn != nil {
		return m.publishFn(ctx, actorID, itemID)
	}
	return domain.Version{}, nil
}

func (m *mockContentService) Archive(ctx context.Context, actorID, itemID uuid.UUID) (domain.Item, error) {
	if m.archiveFn != nil {
		return m.archiveFn(ctx, actorID, itemID)
	}
	return domain.Item{}, nil
}

func (m *mockContentService) ListAdminItems(
	ctx context.Context, status, kind, query *string, limit, offset int,
) ([]domain.Item, int64, error) {
	if m.listAdminItemsFn != nil {
		return m.listAdminItemsFn(ctx, status, kind, query, limit, offset)
	}
	return []domain.Item{}, 0, nil
}

func (m *mockContentService) ReportItem(
	ctx context.Context, userID, versionID uuid.UUID, reason domain.ReportReason, note *string,
) (domain.ItemReport, error) {
	if m.reportItemFn != nil {
		return m.reportItemFn(ctx, userID, versionID, reason, note)
	}
	return domain.ItemReport{
		ID:               uuid.New(),
		ContentVersionID: versionID,
		UserID:           userID,
		Reason:           reason,
		Note:             note,
		CreatedAt:        time.Now(),
	}, nil
}

func (m *mockContentService) ListReportedContent(
	ctx context.Context, limit, offset int,
) ([]domain.ReportedVersionSummary, int, error) {
	if m.listReportedContentFn != nil {
		return m.listReportedContentFn(ctx, limit, offset)
	}
	return []domain.ReportedVersionSummary{}, 0, nil
}

func (m *mockContentService) GetAdminItemDetail(
	ctx context.Context, id uuid.UUID,
) (domain.Item, []domain.Version, error) {
	if m.getAdminItemDetailFn != nil {
		return m.getAdminItemDetailFn(ctx, id)
	}
	return domain.Item{}, []domain.Version{}, nil
}

type mockGuard struct {
	deniedPermission string
}

func (g *mockGuard) Require(_ context.Context, permission string) error {
	if g.deniedPermission == permission {
		return apperr.New(apperr.Forbidden, "FORBIDDEN", "Permission denied.")
	}
	return nil
}

func setupTestRouter(svc contenthttp.ContentService, guard contenthttp.Guard) http.Handler {
	r := chi.NewRouter()
	h, err := contenthttp.NewHandler(svc, guard)
	if err != nil {
		panic(err)
	}
	h.Routes(r)
	h.AdminRoutes(r)
	return r
}

func TestBrowseHandler(t *testing.T) {
	t.Parallel()

	svc := &mockContentService{
		browseFn: func(_ context.Context, _ contract.BrowseFilter) ([]*contract.Version, int, error) {
			pubAt := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
			return []*contract.Version{
				{
					ID:          uuid.New(),
					ItemID:      uuid.New(),
					Version:     1,
					Kind:        testKindVocabWord,
					Body:        json.RawMessage(`{"word":"hello"}`),
					CEFRLevel:   "A1",
					Status:      statusPublished,
					Tags:        []string{"greetings"},
					PublishedAt: &pubAt,
				},
			}, 1, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	req := httptest.NewRequest(http.MethodGet, "/content?kind="+testKindVocabWord+"&limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contenthttp.ContentVersionListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Errorf("got %d items, total %d, want 1", len(resp.Items), resp.Total)
	}
}

func TestGetBySlugHandler(t *testing.T) {
	t.Parallel()

	verID := uuid.New()
	svc := &mockContentService{
		getPublishedSlugFn: func(_ context.Context, slug string) (*contract.Version, error) {
			if slug == "hello-world" {
				return &contract.Version{
					ID:        verID,
					ItemID:    uuid.New(),
					Version:   1,
					Kind:      testKindVocabWord,
					Body:      json.RawMessage(`{"word":"hello"}`),
					CEFRLevel: "A1",
					Status:    statusPublished,
				}, nil
			}
			return nil, domain.ErrContentNotPublished
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	// Success
	req := httptest.NewRequest(http.MethodGet, "/content/hello-world", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 404 Not Found for unpublished slug
	req404 := httptest.NewRequest(http.MethodGet, "/content/missing-slug", nil)
	rec404 := httptest.NewRecorder()
	router.ServeHTTP(rec404, req404)

	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec404.Code)
	}
}

func TestAdminCreateItemHandler(t *testing.T) {
	t.Parallel()

	actorID := uuid.New()
	svc := &mockContentService{
		createItemFn: func(
			_ context.Context,
			actorID uuid.UUID,
			req service.CreateItemRequest,
		) (domain.Item, domain.Version, error) {
			return domain.Item{
				ID:        uuid.New(),
				Kind:      req.Kind,
				Slug:      req.Slug,
				Status:    domain.StatusDraft,
				OwnerID:   actorID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}, domain.Version{Version: 1}, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	payload := contenthttp.CreateContentItemRequest{
		Kind:      testKindVocabWord,
		Slug:      "hello-slug",
		CEFRLevel: "A1",
		Body:      json.RawMessage(`{"word":"hello"}`),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/admin/content", bytes.NewReader(body))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID, Role: roleAdmin}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReportItemHandler(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	versionID := uuid.New()
	reportedCalled := false

	svc := &mockContentService{
		reportItemFn: func(
			_ context.Context, uID, vID uuid.UUID, reason domain.ReportReason, note *string,
		) (domain.ItemReport, error) {
			reportedCalled = true
			if uID != userID {
				t.Errorf("uID = %v, want %v", uID, userID)
			}
			if vID != versionID {
				t.Errorf("vID = %v, want %v", vID, versionID)
			}
			if reason != domain.ReportReasonTypo {
				t.Errorf("reason = %v, want typo", reason)
			}
			return domain.ItemReport{
				ID:               uuid.New(),
				ContentVersionID: vID,
				UserID:           uID,
				Reason:           reason,
				Note:             note,
				CreatedAt:        time.Now(),
			}, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	payload := map[string]any{
		"reason": "typo",
		"note":   "Small spelling issue",
	}
	body, _ := json.Marshal(payload)

	target := fmt.Sprintf("/content/versions/%s/reports", versionID)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: userID, Role: "learner"}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	if !reportedCalled {
		t.Errorf("expected reportItemFn to be called")
	}
}

func TestAdminListReportsHandler(t *testing.T) {
	t.Parallel()

	adminID := uuid.New()
	versionID := uuid.New()
	itemID := uuid.New()

	svc := &mockContentService{
		listReportedContentFn: func(
			_ context.Context, _, _ int,
		) ([]domain.ReportedVersionSummary, int, error) {
			return []domain.ReportedVersionSummary{
				{
					ContentVersionID: versionID,
					ItemID:           itemID,
					Slug:             "reported-word",
					Kind:             "vocab_word",
					CEFRLevel:        "B1",
					ItemStatus:       statusPublished,
					ReportCount:      5,
					LastReportedAt:   time.Now(),
				},
			}, 1, nil
		},
	}

	router := setupTestRouter(svc, &mockGuard{})

	req := httptest.NewRequest(http.MethodGet, "/admin/content/reports?page=1&per_page=20", nil)
	req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: adminID, Role: roleAdmin}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	items, ok := res["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 item in items, got %v", res)
	}
}
