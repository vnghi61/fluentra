package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/modules/content/domain"
	"github.com/fluentra/fluentra/internal/modules/content/service"
	contenthttp "github.com/fluentra/fluentra/internal/modules/content/transport/http"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// decisionApproved is the wire value of an approving review decision.
const decisionApproved = "approved"

// adminRequest builds an authenticated admin request against the test router.
// Passing nil for body sends no payload, which is what the four state-change
// endpoints expect.
func adminRequest(method, target string, body any, actorID uuid.UUID) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	if actorID != uuid.Nil {
		req = req.WithContext(httpx.WithActor(req.Context(), httpx.Actor{UserID: actorID, Role: "admin"}))
	}
	return req
}

func do(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeVersion(t *testing.T, rec *httptest.ResponseRecorder) contenthttp.ContentVersionResponse {
	t.Helper()
	var resp contenthttp.ContentVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response %s: %v", rec.Body.String(), err)
	}
	return resp
}

func TestNewHandlerRejectsANilGuard(t *testing.T) {
	t.Parallel()

	if _, err := contenthttp.NewHandler(&mockContentService{}, nil); err == nil {
		t.Fatal("NewHandler accepted a nil guard; admin authoring would be unprotected")
	}
}

func TestUpdateDraftHandler(t *testing.T) {
	t.Parallel()

	itemID := uuid.New()
	actorID := uuid.New()
	var seen service.UpdateDraftRequest

	svc := &mockContentService{
		updateDraftFn: func(
			_ context.Context,
			_, _ uuid.UUID,
			req service.UpdateDraftRequest,
		) (domain.Version, error) {
			seen = req
			return domain.Version{
				ID:        uuid.New(),
				ItemID:    itemID,
				Version:   2,
				Kind:      testKindVocabWord,
				Body:      req.Body,
				CEFRLevel: *req.CEFRLevel,
				Status:    domain.StatusDraft,
			}, nil
		},
	}
	router := setupTestRouter(svc, &mockGuard{})

	level := "B2"
	payload := contenthttp.UpdateDraftRequest{
		CEFRLevel: &level,
		Body:      json.RawMessage(`{"word":"revised"}`),
		Tags:      []string{"environment"},
	}

	rec := do(router, adminRequest(http.MethodPut, "/admin/content/"+itemID.String()+"/draft", payload, actorID))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeVersion(t, rec)
	if resp.Version != 2 || resp.CEFRLevel != "B2" {
		t.Errorf("response = version %d level %q, want 2 and B2", resp.Version, resp.CEFRLevel)
	}
	if resp.MediaRefs == nil || resp.Tags == nil {
		t.Error("media_refs and tags must serialise as [] rather than null")
	}
	if seen.CEFRLevel == nil || *seen.CEFRLevel != "B2" || len(seen.Tags) != 1 {
		t.Errorf("service received %+v, want the decoded level and tags", seen)
	}
}

func TestSubmitForReviewHandler(t *testing.T) {
	t.Parallel()

	itemID := uuid.New()
	svc := &mockContentService{
		submitFn: func(_ context.Context, _, _ uuid.UUID) (domain.Version, error) {
			return domain.Version{ID: uuid.New(), ItemID: itemID, Version: 1, Status: domain.StatusInReview}, nil
		},
	}
	router := setupTestRouter(svc, &mockGuard{})

	rec := do(router, adminRequest(http.MethodPost, "/admin/content/"+itemID.String()+"/submit", nil, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := decodeVersion(t, rec).Status; got != string(domain.StatusInReview) {
		t.Errorf("status = %q, want in_review", got)
	}
}

func TestReviewHandler(t *testing.T) {
	t.Parallel()

	itemID := uuid.New()
	var seen service.ReviewDecisionRequest

	svc := &mockContentService{
		reviewFn: func(
			_ context.Context,
			_, _ uuid.UUID,
			req service.ReviewDecisionRequest,
		) (domain.Version, error) {
			seen = req
			return domain.Version{ID: uuid.New(), ItemID: itemID, Version: 1, Status: domain.StatusApproved}, nil
		},
	}
	router := setupTestRouter(svc, &mockGuard{})

	comments := "Looks good."
	payload := contenthttp.ReviewDecisionRequest{Decision: decisionApproved, Comments: &comments}

	rec := do(router, adminRequest(http.MethodPost, "/admin/content/"+itemID.String()+"/review", payload, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if seen.Decision != domain.ReviewDecisionApproved {
		t.Errorf("decision = %q, want approved", seen.Decision)
	}
	if seen.Comments == nil || *seen.Comments != comments {
		t.Errorf("comments = %v, want %q", seen.Comments, comments)
	}
}

func TestReviewHandlerRejectsAnUnknownDecision(t *testing.T) {
	t.Parallel()

	called := false
	svc := &mockContentService{
		reviewFn: func(
			_ context.Context,
			_, _ uuid.UUID,
			_ service.ReviewDecisionRequest,
		) (domain.Version, error) {
			called = true
			return domain.Version{}, nil
		},
	}
	router := setupTestRouter(svc, &mockGuard{})

	payload := contenthttp.ReviewDecisionRequest{Decision: "looks-fine-to-me"}
	rec := do(router, adminRequest(
		http.MethodPost, "/admin/content/"+uuid.New().String()+"/review", payload, uuid.New(),
	))

	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected a 4xx validation status, got %d: %s", rec.Code, rec.Body.String())
	}
	if called {
		t.Error("an unparseable decision reached the service")
	}
}

func TestPublishHandler(t *testing.T) {
	t.Parallel()

	itemID := uuid.New()
	publishedAt := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	svc := &mockContentService{
		publishFn: func(_ context.Context, _, _ uuid.UUID) (domain.Version, error) {
			return domain.Version{
				ID:          uuid.New(),
				ItemID:      itemID,
				Version:     1,
				Status:      domain.StatusPublished,
				PublishedAt: &publishedAt,
			}, nil
		},
	}
	router := setupTestRouter(svc, &mockGuard{})

	rec := do(router, adminRequest(http.MethodPost, "/admin/content/"+itemID.String()+"/publish", nil, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	resp := decodeVersion(t, rec)
	if resp.Status != string(domain.StatusPublished) || resp.PublishedAt == nil {
		t.Errorf("response = %+v, want a published version carrying published_at", resp)
	}
}

func TestArchiveHandler(t *testing.T) {
	t.Parallel()

	itemID := uuid.New()
	svc := &mockContentService{
		archiveFn: func(_ context.Context, _, _ uuid.UUID) (domain.Item, error) {
			return domain.Item{
				ID:     itemID,
				Kind:   testKindVocabWord,
				Slug:   "archived-slug",
				Status: domain.StatusArchived,
			}, nil
		},
	}
	router := setupTestRouter(svc, &mockGuard{})

	rec := do(router, adminRequest(http.MethodPost, "/admin/content/"+itemID.String()+"/archive", nil, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contenthttp.ContentItemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != string(domain.StatusArchived) {
		t.Errorf("status = %q, want archived", resp.Status)
	}
}

// TestAdminEndpointsRejectAMalformedID keeps a bad path parameter a documented
// 4xx rather than a uuid.Parse panic or a 500 from the service.
func TestAdminEndpointsRejectAMalformedID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPut, "/admin/content/not-a-uuid/draft", contenthttp.UpdateDraftRequest{}},
		{http.MethodPost, "/admin/content/not-a-uuid/submit", nil},
		{http.MethodPost, "/admin/content/not-a-uuid/review", contenthttp.ReviewDecisionRequest{Decision: decisionApproved}},
		{http.MethodPost, "/admin/content/not-a-uuid/publish", nil},
		{http.MethodPost, "/admin/content/not-a-uuid/archive", nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			t.Parallel()
			router := setupTestRouter(&mockContentService{}, &mockGuard{})
			rec := do(router, adminRequest(testCase.method, testCase.path, testCase.body, uuid.New()))
			if rec.Code < 400 || rec.Code >= 500 {
				t.Fatalf("expected a 4xx, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAdminEndpointsRequireAnActor proves the handlers do not fall back to the
// nil UUID when the request carries no authenticated actor.
func TestAdminEndpointsRequireAnActor(t *testing.T) {
	t.Parallel()

	id := uuid.New().String()
	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/admin/content", contenthttp.CreateContentItemRequest{Kind: testKindVocabWord, Slug: "s"}},
		{http.MethodPut, "/admin/content/" + id + "/draft", contenthttp.UpdateDraftRequest{}},
		{http.MethodPost, "/admin/content/" + id + "/submit", nil},
		{http.MethodPost, "/admin/content/" + id + "/review", contenthttp.ReviewDecisionRequest{Decision: decisionApproved}},
		{http.MethodPost, "/admin/content/" + id + "/publish", nil},
		{http.MethodPost, "/admin/content/" + id + "/archive", nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			t.Parallel()
			router := setupTestRouter(&mockContentService{}, &mockGuard{})
			rec := do(router, adminRequest(testCase.method, testCase.path, testCase.body, uuid.Nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 Unauthorized, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestEveryEndpointIsBehindItsOwnPermission denies exactly one permission per
// run and asserts the matching endpoint is the one that 403s. A handler wired
// to the wrong permission fails here rather than in production.
func TestEveryEndpointIsBehindItsOwnPermission(t *testing.T) {
	t.Parallel()

	id := uuid.New().String()
	cases := []struct {
		permission string
		method     string
		path       string
		body       any
	}{
		{contenthttp.PermContentReadPublished, http.MethodGet, "/content", nil},
		{contenthttp.PermContentReadPublished, http.MethodGet, "/content/some-slug", nil},
		{
			contenthttp.PermContentCreate, http.MethodPost, "/admin/content",
			contenthttp.CreateContentItemRequest{Kind: testKindVocabWord, Slug: "s"},
		},
		{contenthttp.PermContentEdit, http.MethodPut, "/admin/content/" + id + "/draft", contenthttp.UpdateDraftRequest{}},
		{contenthttp.PermContentEdit, http.MethodPost, "/admin/content/" + id + "/submit", nil},
		{
			contenthttp.PermContentReview, http.MethodPost, "/admin/content/" + id + "/review",
			contenthttp.ReviewDecisionRequest{Decision: decisionApproved},
		},
		{contenthttp.PermContentPublish, http.MethodPost, "/admin/content/" + id + "/publish", nil},
		{contenthttp.PermContentPublish, http.MethodPost, "/admin/content/" + id + "/archive", nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.permission+" "+testCase.path, func(t *testing.T) {
			t.Parallel()
			router := setupTestRouter(&mockContentService{}, &mockGuard{deniedPermission: testCase.permission})
			rec := do(router, adminRequest(testCase.method, testCase.path, testCase.body, uuid.New()))
			if rec.Code != http.StatusForbidden {
				t.Fatalf(
					"denying %s did not block %s %s: got %d",
					testCase.permission, testCase.method, testCase.path, rec.Code,
				)
			}
		})
	}
}

func TestBrowseHandlerIgnoresUnparseablePaging(t *testing.T) {
	t.Parallel()

	var seen struct {
		limit  int
		offset int
	}
	svc := &mockContentService{}
	svc.browseFn = func(_ context.Context, filter contract.BrowseFilter) ([]*contract.Version, int, error) {
		seen.limit = filter.Limit
		seen.offset = filter.Offset
		return nil, 0, nil
	}
	router := setupTestRouter(svc, &mockGuard{})

	rec := do(router, httptest.NewRequest(http.MethodGet, "/content?limit=many&offset=soon", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if seen.limit != 0 || seen.offset != 0 {
		t.Errorf("filter = limit %d offset %d, want both left at 0 for the service to default", seen.limit, seen.offset)
	}
}
