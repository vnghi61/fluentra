//go:build integration

package user_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth"
	authservice "github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/rbac"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/modules/user"
	usercontract "github.com/fluentra/fluentra/internal/modules/user/contract"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

const deletionJobName = "user.account_deletion_executor"

type deletionTestHarness struct {
	userModule *user.Module
	authModule *auth.Module
	rbacModule *rbac.Module
	bus        *eventbus.InProcessBus
	router     chi.Router
	jwtKey     []byte
	userID     uuid.UUID
}

type testDispatcher struct{ bus *eventbus.InProcessBus }

func (d testDispatcher) Dispatch(ctx context.Context, event outbox.Event) error {
	return d.bus.Publish(ctx, eventbus.Message{
		ID:      event.ID,
		Topic:   event.Topic(),
		Payload: event.Payload,
		Attempt: event.Attempt,
	})
}

func (h *deletionTestHarness) dispatchOutboxEvents(ctx context.Context, t *testing.T) {
	t.Helper()
	publisher := outbox.NewPublisher(pool, testDispatcher{bus: h.bus}, 50, time.Second)
	if err := publisher.ProcessBatch(ctx); err != nil {
		t.Fatalf("ProcessBatch outbox events: %v", err)
	}
}

func setupDeletionHarness(t *testing.T) *deletionTestHarness {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	const reset = `
		TRUNCATE core.users CASCADE;
		TRUNCATE ops.outbox_events;
		TRUNCATE core.user_deletions CASCADE;
	`
	if _, err := pool.Exec(context.Background(), reset); err != nil {
		t.Fatalf("reset tables: %v", err)
	}

	bus := eventbus.NewInProcessBus(eventbus.NewRegistry())
	store := newTestStorage(t)

	userMod := user.New(user.Deps{
		Pool:    pool,
		Storage: store,
	})

	jwtKey := []byte("test-jwt-signing-key-at-least-32-bytes-long")
	authMod := auth.New(auth.Deps{
		Pool:       pool,
		OTPHMACKey: []byte("test-otp-hmac-key-at-least-32-bytes-long"),
		Registrar:  userMod.Registrar(),
		Tokens: authservice.TokenConfig{
			SigningKey: jwtKey,
			Issuer:     "fluentra",
			Audience:   "fluentra-api",
		},
	})

	rbacMod := rbac.New(rbac.Deps{
		Pool: pool,
		Env:  "local",
	})

	if err := authMod.Subscribe(bus); err != nil {
		t.Fatalf("subscribe auth: %v", err)
	}
	if err := rbacMod.Subscribe(bus); err != nil {
		t.Fatalf("subscribe rbac: %v", err)
	}

	ctx := context.Background()
	userID, err := userMod.Registrar().CreateUser(ctx, usercontract.NewUser{
		Email:       "gdpr-learner@example.com",
		DisplayName: "GDPR Learner",
		Locale:      "en",
		Timezone:    "UTC",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_ = userMod.Registrar().MarkEmailVerified(ctx, userID)
	_, _ = rbacMod.AssignRole(ctx, uuid.New(), userID, rbaccontract.RoleUser)

	router := chi.NewRouter()
	router.Use(authMod.Authenticate())
	userMod.Routes(router)

	return &deletionTestHarness{
		userModule: userMod,
		authModule: authMod,
		rbacModule: rbacMod,
		bus:        bus,
		router:     router,
		jwtKey:     jwtKey,
		userID:     userID,
	}
}

func (h *deletionTestHarness) authReq(t *testing.T, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := authservice.IssueAccessTokenForTest(h.jwtKey, h.userID, "user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, "/me"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, "/me"+path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestModule_DeletionLifecycle(t *testing.T) {
	h := setupDeletionHarness(t)
	ctx := context.Background()

	// 1. Verify /me works initially
	res := h.authReq(t, http.MethodGet, "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("GET /me status = %d, want 200", res.Code)
	}

	// 2. Request deletion: DELETE /me -> 202
	delRes := h.authReq(t, http.MethodDelete, "", "")
	if delRes.Code != http.StatusAccepted {
		t.Fatalf("DELETE /me status = %d, want 202; body: %s", delRes.Code, delRes.Body.String())
	}

	// Duplicate deletion request returns 409
	dupRes := h.authReq(t, http.MethodDelete, "", "")
	if dupRes.Code != http.StatusConflict {
		t.Errorf("duplicate DELETE /me status = %d, want 409", dupRes.Code)
	}

	// 3. Cancel deletion before execution
	cancelRes := h.authReq(t, http.MethodPost, "/deletion/cancel", "")
	if cancelRes.Code != http.StatusOK {
		t.Fatalf("POST /me/deletion/cancel status = %d, want 200", cancelRes.Code)
	}

	summary, err := h.userModule.Reader().GetByID(ctx, h.userID)
	if err != nil {
		t.Fatalf("get user summary: %v", err)
	}
	if summary.Status != "active" {
		t.Errorf("got status %s, want active after cancellation", summary.Status)
	}

	// 4. Request deletion again and execute via scheduled job
	testExecuteDueDeletion(ctx, t, h)
}

func testExecuteDueDeletion(ctx context.Context, t *testing.T, h *deletionTestHarness) {
	t.Helper()

	delRes := h.authReq(t, http.MethodDelete, "", "")
	if delRes.Code != http.StatusAccepted {
		t.Fatalf("DELETE /me second request status = %d, want 202", delRes.Code)
	}

	var delBody struct {
		ID     uuid.UUID `json:"id"`
		UserID uuid.UUID `json:"user_id"`
		Status string    `json:"status"`
	}
	_ = json.Unmarshal(delRes.Body.Bytes(), &delBody)

	var countBefore int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM core.users").Scan(&countBefore); err != nil {
		t.Fatalf("count users before: %v", err)
	}

	updateSQL := `UPDATE core.user_deletions SET execute_at = now() - INTERVAL '1 day' WHERE id = $1`
	if _, err := pool.Exec(ctx, updateSQL, delBody.ID); err != nil {
		t.Fatalf("update execute_at: %v", err)
	}

	// Run DeletionExecutor scheduled job
	for _, c := range h.userModule.CronJobs() {
		if c.Name == deletionJobName {
			if err := c.Task(ctx); err != nil {
				t.Fatalf("execute deletion task: %v", err)
			}
		}
	}

	// Dispatch user.deleted outbox event to auth and rbac
	h.dispatchOutboxEvents(ctx, t)

	// Assert user row is kept and anonymised
	var countAfter int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM core.users").Scan(&countAfter); err != nil {
		t.Fatalf("count users after: %v", err)
	}
	if countAfter != countBefore {
		t.Errorf("got user count %d, want %d (row must NOT be cascade deleted)", countAfter, countBefore)
	}

	var email, status, displayName string
	err := pool.QueryRow(ctx, `
		SELECT u.email, u.status, p.display_name
		FROM core.users u
		LEFT JOIN core.profiles p ON p.user_id = u.id
		WHERE u.id = $1
	`, h.userID).Scan(&email, &status, &displayName)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}

	expectedEmail := fmt.Sprintf("deleted-%s@anonymised.invalid", h.userID)
	if email != expectedEmail || status != "deleted" || displayName != "Deleted User" {
		t.Errorf("anonymisation failed: email=%q, status=%q, displayName=%q", email, status, displayName)
	}

	// Verify cross-module data purge in auth and rbac
	assertNoAuthDataRemains(ctx, t, h.userID)
	assertNoRBACDataRemains(ctx, t, h.userID)

	// Irreversible after execution: cannot cancel
	cancelAfterRes := h.authReq(t, http.MethodPost, "/deletion/cancel", "")
	if cancelAfterRes.Code != http.StatusConflict {
		t.Errorf("POST /me/deletion/cancel after execution status = %d, want 409", cancelAfterRes.Code)
	}
}

func assertNoAuthDataRemains(ctx context.Context, t *testing.T, userID uuid.UUID) {
	t.Helper()
	var sessionCount, credCount int
	if err := pool.QueryRow(
		ctx, "SELECT count(*) FROM core.sessions WHERE user_id = $1", userID,
	).Scan(&sessionCount); err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Errorf("expected 0 sessions, got %d", sessionCount)
	}

	if err := pool.QueryRow(
		ctx, "SELECT count(*) FROM core.credentials WHERE user_id = $1", userID,
	).Scan(&credCount); err != nil {
		t.Fatalf("query credentials: %v", err)
	}
	if credCount != 0 {
		t.Errorf("expected 0 credentials, got %d", credCount)
	}
}

func assertNoRBACDataRemains(ctx context.Context, t *testing.T, userID uuid.UUID) {
	t.Helper()
	var roleCount int
	if err := pool.QueryRow(
		ctx, "SELECT count(*) FROM rbac.user_roles WHERE user_id = $1", userID,
	).Scan(&roleCount); err != nil {
		t.Fatalf("query roles: %v", err)
	}
	if roleCount != 0 {
		t.Errorf("expected 0 role assignments, got %d", roleCount)
	}
}

func TestModule_SessionsRevokedImmediatelyOnDeletionRequest(t *testing.T) {
	h := setupDeletionHarness(t)
	ctx := context.Background()

	// 1. Issue token and verify it works
	res1 := h.authReq(t, http.MethodGet, "", "")
	if res1.Code != http.StatusOK {
		t.Fatalf("GET /me before deletion status = %d, want 200", res1.Code)
	}

	// 2. Request deletion
	delRes := h.authReq(t, http.MethodDelete, "", "")
	if delRes.Code != http.StatusAccepted {
		t.Fatalf("DELETE /me status = %d, want 202", delRes.Code)
	}

	// 3. Dispatch outbox events so auth module receives user.deletion_requested
	h.dispatchOutboxEvents(ctx, t)

	// 4. Assert user is no longer active and mutating calls are rejected
	patchRes := h.authReq(t, http.MethodPatch, "", `{"display_name":"Hacker"}`)
	if patchRes.Code != http.StatusForbidden {
		t.Errorf("PATCH /me after deletion request status = %d, want 403", patchRes.Code)
	}
}

func TestModule_DeletionExecutorIsIdempotent(t *testing.T) {
	h := setupDeletionHarness(t)
	ctx := context.Background()

	// Request deletion
	delRes := h.authReq(t, http.MethodDelete, "", "")
	if delRes.Code != http.StatusAccepted {
		t.Fatalf("DELETE /me status = %d, want 202", delRes.Code)
	}

	var delBody struct {
		ID uuid.UUID `json:"id"`
	}
	_ = json.Unmarshal(delRes.Body.Bytes(), &delBody)

	updateSQL := `UPDATE core.user_deletions SET execute_at = now() - INTERVAL '1 day' WHERE id = $1`
	if _, err := pool.Exec(ctx, updateSQL, delBody.ID); err != nil {
		t.Fatalf("update execute_at: %v", err)
	}

	// Run executor FIRST time
	for _, c := range h.userModule.CronJobs() {
		if c.Name == deletionJobName {
			if err := c.Task(ctx); err != nil {
				t.Fatalf("first execution failed: %v", err)
			}
		}
	}
	h.dispatchOutboxEvents(ctx, t)

	// Run executor SECOND time (idempotency check)
	for _, c := range h.userModule.CronJobs() {
		if c.Name == deletionJobName {
			if err := c.Task(ctx); err != nil {
				t.Fatalf("second execution failed (not idempotent): %v", err)
			}
		}
	}
	h.dispatchOutboxEvents(ctx, t)

	// User should still be anonymised, no errors
	var email string
	if err := pool.QueryRow(ctx, "SELECT email FROM core.users WHERE id = $1", h.userID).Scan(&email); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if !strings.HasPrefix(email, "deleted-") {
		t.Errorf("user not anonymised after second run: %s", email)
	}
}
