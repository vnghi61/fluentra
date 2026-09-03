package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	admincontract "github.com/fluentra/fluentra/internal/modules/admin/contract"
	adminsvc "github.com/fluentra/fluentra/internal/modules/admin/service"
	adminhttp "github.com/fluentra/fluentra/internal/modules/admin/transport/http"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	httpMethodGet  = http.MethodGet
	httpMethodPost = http.MethodPost
	roleAdmin      = "admin"
)

var (
	testAdminID  = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	testTargetID = uuid.MustParse("0199b2d3-4e5f-7a81-8bcd-ef0123456789")
)

type fakeGuard struct {
	held map[string]struct{}
	seen []string
}

func newFakeGuard(permissions ...string) *fakeGuard {
	held := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		held[permission] = struct{}{}
	}
	return &fakeGuard{held: held}
}

func (f *fakeGuard) Require(_ context.Context, permission string) error {
	f.seen = append(f.seen, permission)
	if _, ok := f.held[permission]; ok {
		return nil
	}
	return apperr.New(apperr.Forbidden, "PERMISSION_DENIED", "You do not have permission to perform this action.")
}

type fakeAdminService struct {
	searchCalled     bool
	getUserCalled    bool
	suspendCalled    bool
	reinstateCalled  bool
	revokeCalled     bool
	listFlagsCalled  bool
	createFlagCalled bool
	updateFlagCalled bool
	deleteFlagCalled bool

	seenActorID   uuid.UUID
	seenTargetID  uuid.UUID
	seenReason    string
	seenFlagKey   string
	seenCreateReq adminsvc.CreateFlagRequest
	seenUpdateReq adminsvc.UpdateFlagRequest

	searchResp   []usercontract.UserSummary
	searchCursor string

	getUserResp *usercontract.UserDetail
	getErr      error

	listFlagsResp []admincontract.FeatureFlag
	flagResp      admincontract.FeatureFlag

	aiUsageResp []adminsvc.AIUsageStatus
	aiUsageErr  error
}

func (f *fakeAdminService) SearchUsers(
	_ context.Context, _ usercontract.UserFilter, _ string, _ int,
) ([]usercontract.UserSummary, string, error) {
	f.searchCalled = true
	return f.searchResp, f.searchCursor, nil
}

func (f *fakeAdminService) GetUserByID(
	_ context.Context, actorID uuid.UUID, targetID uuid.UUID,
) (*usercontract.UserDetail, error) {
	f.getUserCalled = true
	f.seenActorID = actorID
	f.seenTargetID = targetID
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getUserResp, nil
}

func (f *fakeAdminService) SuspendUser(_ context.Context, actorID uuid.UUID, targetID uuid.UUID, reason string) error {
	f.suspendCalled = true
	f.seenActorID = actorID
	f.seenTargetID = targetID
	f.seenReason = reason
	return nil
}

func (f *fakeAdminService) ReinstateUser(
	_ context.Context, actorID uuid.UUID, targetID uuid.UUID, reason string,
) error {
	f.reinstateCalled = true
	f.seenActorID = actorID
	f.seenTargetID = targetID
	f.seenReason = reason
	return nil
}

func (f *fakeAdminService) RevokeUserSessions(
	_ context.Context, actorID uuid.UUID, targetID uuid.UUID, reason string,
) error {
	f.revokeCalled = true
	f.seenActorID = actorID
	f.seenTargetID = targetID
	f.seenReason = reason
	return nil
}

func (f *fakeAdminService) ListFlags(_ context.Context) ([]admincontract.FeatureFlag, error) {
	f.listFlagsCalled = true
	return f.listFlagsResp, nil
}

func (f *fakeAdminService) CreateFlag(
	_ context.Context, req adminsvc.CreateFlagRequest,
) (admincontract.FeatureFlag, error) {
	f.createFlagCalled = true
	f.seenCreateReq = req
	return f.flagResp, nil
}

func (f *fakeAdminService) UpdateFlag(
	_ context.Context, key string, req adminsvc.UpdateFlagRequest,
) (admincontract.FeatureFlag, error) {
	f.updateFlagCalled = true
	f.seenFlagKey = key
	f.seenUpdateReq = req
	return f.flagResp, nil
}

func (f *fakeAdminService) DeleteFlag(_ context.Context, key string) error {
	f.deleteFlagCalled = true
	f.seenFlagKey = key
	return nil
}

func (f *fakeAdminService) GetAIUsage(_ context.Context) ([]adminsvc.AIUsageStatus, error) {
	return f.aiUsageResp, f.aiUsageErr
}

func newServer(svc adminhttp.AdminService, guard adminhttp.Guard) http.Handler {
	handler := adminhttp.NewHandler(svc, guard)
	router := chi.NewRouter()
	handler.Routes(router)
	return router
}

func doRequest(
	handler http.Handler,
	method, path string,
	body string,
	actor *httpx.Actor,
) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if actor != nil {
		req = req.WithContext(httpx.WithActor(req.Context(), *actor))
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestUnauthenticated_Returns401(t *testing.T) {
	svc := &fakeAdminService{}
	guard := newFakeGuard("user.list", "user.read", "user.suspend", "system.flags")
	srv := newServer(svc, guard)

	paths := []struct {
		method string
		path   string
	}{
		{httpMethodGet, "/admin/users"},
		{httpMethodGet, "/admin/users/" + testTargetID.String()},
		{httpMethodPost, "/admin/users/" + testTargetID.String() + "/suspend"},
		{httpMethodGet, "/admin/flags"},
		{"DELETE", "/admin/flags/test-flag"},
	}

	for _, tc := range paths {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doRequest(srv, tc.method, tc.path, "", nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 UNAUTHENTICATED, got %d", rec.Code)
			}
			if svc.searchCalled || svc.getUserCalled || svc.suspendCalled || svc.listFlagsCalled || svc.deleteFlagCalled {
				t.Fatalf("service should NOT have been called when unauthenticated")
			}
		})
	}
}

func TestGuardRefuses_Returns403(t *testing.T) {
	svc := &fakeAdminService{}
	guard := newFakeGuard() // holds no permissions
	srv := newServer(svc, guard)
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodGet, "/admin/users", "", actor)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 FORBIDDEN, got %d", rec.Code)
	}
	if svc.searchCalled {
		t.Fatalf("service searchUsers should NOT have been called when guard refused")
	}
}

func TestMalformedUUID_Returns422(t *testing.T) {
	svc := &fakeAdminService{}
	guard := newFakeGuard("user.read", "user.suspend", "user.reinstate", "user.manage_sessions")
	srv := newServer(svc, guard)
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	paths := []struct {
		method string
		path   string
	}{
		{httpMethodGet, "/admin/users/invalid-uuid"},
		{httpMethodPost, "/admin/users/not-a-uuid/suspend"},
		{httpMethodPost, "/admin/users/not-a-uuid/reinstate"},
		{httpMethodPost, "/admin/users/not-a-uuid/sessions/revoke"},
	}

	for _, tc := range paths {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doRequest(srv, tc.method, tc.path, `{"reason":"a long enough reason"}`, actor)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422 UNPROCESSABLE_ENTITY for malformed UUID, got %d", rec.Code)
			}
		})
	}
}

func TestDeleteFlag_Returns204NoContent(t *testing.T) {
	svc := &fakeAdminService{}
	guard := newFakeGuard("system.flags")
	srv := newServer(svc, guard)
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, "DELETE", "/admin/flags/my-flag", "", actor)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 StatusNoContent, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body for 204 No Content, got %q", rec.Body.String())
	}
	if !svc.deleteFlagCalled {
		t.Fatalf("expected DeleteFlag to be called")
	}
	if svc.seenFlagKey != "my-flag" {
		t.Fatalf("expected flag key 'my-flag', got %q", svc.seenFlagKey)
	}
}

func TestSuspendUser_Success(t *testing.T) {
	svc := &fakeAdminService{}
	guard := newFakeGuard("user.suspend")
	srv := newServer(svc, guard)
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	body := `{"reason":"Violating community guidelines multiple times"}`
	rec := doRequest(srv, "POST", "/admin/users/"+testTargetID.String()+"/suspend", body, actor)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !svc.suspendCalled {
		t.Fatalf("expected SuspendUser to be called")
	}
	if svc.seenActorID != testAdminID || svc.seenTargetID != testTargetID {
		t.Fatalf("actor or target ID mismatch")
	}
	if svc.seenReason != "Violating community guidelines multiple times" {
		t.Fatalf("reason mismatch, got %q", svc.seenReason)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if resp["status"] != "suspended" {
		t.Fatalf("expected status 'suspended', got %v", resp["status"])
	}
}

func TestAdminGetAIUsage_Success(t *testing.T) {
	reqLimit := 1000
	tokenLimit := int64(500000)
	svc := &fakeAdminService{
		aiUsageResp: []adminsvc.AIUsageStatus{
			{
				Provider:          "openai_compatible",
				Task:              "vocab_verify",
				RequestsToday:     45,
				TokensToday:       12050,
				DailyRequestLimit: &reqLimit,
				DailyTokenLimit:   &tokenLimit,
				IsExhausted:       false,
			},
		},
	}
	guard := newFakeGuard("admin.dashboard")
	srv := newServer(svc, guard)
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodGet, "/admin/ai/usage", "", actor)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp struct {
		Items []adminhttp.AdminAIUsageItemDTO `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Provider != "openai_compatible" || resp.Items[0].Task != "vocab_verify" {
		t.Fatalf("unexpected item content: %+v", resp.Items[0])
	}
}

func TestAdminGetAIUsage_Forbidden(t *testing.T) {
	svc := &fakeAdminService{}
	guard := newFakeGuard("some.other.permission")
	srv := newServer(svc, guard)
	actor := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}

	rec := doRequest(srv, httpMethodGet, "/admin/ai/usage", "", actor)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
	}
}
