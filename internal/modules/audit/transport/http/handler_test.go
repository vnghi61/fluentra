package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/audit/contract"
	"github.com/fluentra/fluentra/internal/modules/audit/domain"
	audithttp "github.com/fluentra/fluentra/internal/modules/audit/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

var (
	adminID   = uuid.MustParse("0199a1c2-3d4e-7f80-9abc-def012345678")
	learnerID = uuid.MustParse("0199b2d3-4e5f-7a81-8bcd-ef0123456789")
	openEvent = uuid.MustParse("0199c3e4-5f60-7b82-9cde-f01234567890")
)

const testRequestID = "01KZGA1FXY6VAHQABK3EBKDN57"

// The two search paths, spelled once.
const (
	pathAuditLogs      = "/api/v1/admin/audit-logs"
	pathSecurityEvents = "/api/v1/admin/security-events"
)

// fixedNow anchors the default search window so a test can assert what the
// handler asked for rather than what the wall clock happened to say.
var fixedNow = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

// fakeGuard answers from a permission set, so a test says "this caller holds
// audit.read" and every layer below agrees.
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
	return apperr.New(apperr.Forbidden, "PERMISSION_DENIED",
		"You do not have permission to perform this action.")
}

// fakeTrail records the query it was handed, which is how the window and limit
// defaults are asserted: they are the handler's job, and invisible in the
// response.
type fakeTrail struct {
	logs        []domain.LogEntry
	events      []domain.SecurityRecord
	hasMore     bool
	seenLog     domain.LogQuery
	seenSec     domain.SecurityQuery
	resolveErr  error
	resolved    domain.SecurityRecord
	seenNote    string
	seenActor   uuid.UUID
	seenEventID uuid.UUID
}

func newFakeTrail() *fakeTrail {
	actor := adminID
	role := contract.ActorRoleUser
	targetType := "user"
	targetID := learnerID.String()
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"

	return &fakeTrail{
		logs: []domain.LogEntry{{
			ID:            uuid.MustParse("0199d4f5-6071-7c83-ad01-234567890abc"),
			CreatedAt:     fixedNow.Add(-time.Hour),
			EventID:       uuid.MustParse("0199d4f5-6071-7c83-ad01-234567890abd"),
			ActorID:       &actor,
			ActorRole:     &role,
			Action:        "user.profile_updated",
			TargetType:    &targetType,
			TargetID:      &targetID,
			ChangedFields: []string{"display_name", "timezone"},
			Meta:          map[string]any{"source": "outbox"},
			TraceID:       &traceID,
		}},
		events: []domain.SecurityRecord{{
			ID:        openEvent,
			CreatedAt: fixedNow.Add(-2 * time.Hour),
			UpdatedAt: fixedNow.Add(-2 * time.Hour),
			EventID:   uuid.MustParse("0199c3e4-5f60-7b82-9cde-f01234567891"),
			Kind:      "rbac.access_denied",
			Severity:  contract.SeverityMedium,
			UserID:    &learnerID,
			Detail:    map[string]any{"permission": "user.suspend"},
			TraceID:   &traceID,
		}},
	}
}

func (f *fakeTrail) SearchLogs(
	_ context.Context, query domain.LogQuery,
) ([]domain.LogEntry, bool, error) {
	f.seenLog = query
	return f.logs, f.hasMore, nil
}

func (f *fakeTrail) SearchSecurityEvents(
	_ context.Context, query domain.SecurityQuery,
) ([]domain.SecurityRecord, bool, error) {
	f.seenSec = query
	return f.events, f.hasMore, nil
}

func (f *fakeTrail) ResolveSecurityEvent(
	_ context.Context, eventID, resolvedBy uuid.UUID, note string,
) (domain.SecurityRecord, error) {
	f.seenEventID, f.seenActor, f.seenNote = eventID, resolvedBy, note
	if f.resolveErr != nil {
		return domain.SecurityRecord{}, f.resolveErr
	}
	// The real service validates the note in the domain, and the status a
	// client sees for a blank one depends on that. A fake that skipped it would
	// let the handler tests pass while the deployed API answered differently.
	if _, err := domain.ValidateNote(note); err != nil {
		return domain.SecurityRecord{}, err
	}
	resolvedAt := fixedNow
	record := f.events[0]
	record.ResolvedAt = &resolvedAt
	record.ResolvedBy = &resolvedBy
	record.ResolutionNote = &note
	record.UpdatedAt = resolvedAt
	f.resolved = record
	return record, nil
}

func newServer(trail *fakeTrail, guard *fakeGuard) http.Handler {
	handler := audithttp.NewHandler(trail, guard, func() time.Time { return fixedNow })
	router := chi.NewRouter()
	router.Route("/api/v1", handler.Routes)
	return router
}

func do(
	t *testing.T, handler http.Handler, method, path, body string, actor *uuid.UUID,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	ctx := httpx.WithRequestID(request.Context(), testRequestID)
	if actor != nil {
		ctx = httpx.WithActor(ctx, httpx.Actor{UserID: *actor, Role: "admin"})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request.WithContext(ctx))
	return recorder
}

// TestEveryOperationRefusesAnUnauthenticatedCaller. The guard cannot answer for
// somebody who is not there, and an operation reached without an actor must
// not fall through to one that treats the nil uuid as a user.
func TestEveryOperationRefusesAnUnauthenticatedCaller(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead, contract.PermissionManage))

	cases := []struct{ method, path, body string }{
		{http.MethodGet, pathAuditLogs, ""},
		{http.MethodGet, pathSecurityEvents, ""},
		{http.MethodPost, pathSecurityEvents + "/" + openEvent.String() + "/resolve", `{"note":"x"}`},
	}
	for _, testCase := range cases {
		recorder := do(t, server, testCase.method, testCase.path, testCase.body, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", testCase.method, testCase.path, recorder.Code)
		}
	}
}

// TestEveryOperationCallsTheGuardWithItsOwnPermission is the structural check.
// A route reachable because the /admin middleware let it through, with no
// Require of its own, is the hole this asserts does not exist.
func TestEveryOperationCallsTheGuardWithItsOwnPermission(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method, path, body string
		want               string
	}{
		{http.MethodGet, pathAuditLogs, "", contract.PermissionRead},
		{http.MethodGet, pathSecurityEvents, "", contract.PermissionRead},
		{
			http.MethodPost,
			pathSecurityEvents + "/" + openEvent.String() + "/resolve",
			`{"note":"Triaged."}`, contract.PermissionManage,
		},
	}
	for _, testCase := range cases {
		guard := newFakeGuard(testCase.want)
		server := newServer(newFakeTrail(), guard)

		recorder := do(t, server, testCase.method, testCase.path, testCase.body, &adminID)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s %s = %d (body %s)", testCase.method, testCase.path, recorder.Code, recorder.Body)
		}
		if len(guard.seen) != 1 || guard.seen[0] != testCase.want {
			t.Errorf("%s %s asked for %v, want [%s]", testCase.method, testCase.path, guard.seen, testCase.want)
		}

		// And the same call without that permission is refused.
		refused := newServer(newFakeTrail(), newFakeGuard())
		denied := do(t, refused, testCase.method, testCase.path, testCase.body, &adminID)
		if denied.Code != http.StatusForbidden {
			t.Errorf("%s %s without the permission = %d, want 403", testCase.method, testCase.path, denied.Code)
		}
	}
}

// TestSearchDefaultsToABoundedWindow is the partition-pruning guarantee, seen
// from the outside: a caller who supplies nothing still gets a query the
// planner can prune.
func TestSearchDefaultsToABoundedWindow(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	server := newServer(trail, newFakeGuard(contract.PermissionRead))

	if recorder := do(t, server, http.MethodGet, pathAuditLogs, "", &adminID); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", recorder.Code, recorder.Body)
	}
	if !trail.seenLog.Window.End.Equal(fixedNow) {
		t.Errorf("window end = %s, want now", trail.seenLog.Window.End)
	}
	if span := trail.seenLog.Window.End.Sub(trail.seenLog.Window.Start); span != domain.DefaultWindow {
		t.Errorf("window span = %s, want the %s default", span, domain.DefaultWindow)
	}
	if trail.seenLog.Limit != domain.DefaultLimit {
		t.Errorf("limit = %d, want %d", trail.seenLog.Limit, domain.DefaultLimit)
	}
}

func TestSearchPassesEveryFilterThrough(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	server := newServer(trail, newFakeGuard(contract.PermissionRead))

	path := pathAuditLogs + "?actor_id=" + adminID.String() +
		"&action=user.profile_updated&target_type=user&target_id=" + learnerID.String() +
		"&from=2026-08-01T00:00:00Z&to=2026-08-09T00:00:00Z&limit=50"
	if recorder := do(t, server, http.MethodGet, path, "", &adminID); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", recorder.Code, recorder.Body)
	}

	query := trail.seenLog
	if query.ActorID == nil || *query.ActorID != adminID {
		t.Errorf("actor_id = %v", query.ActorID)
	}
	if query.Action == nil || *query.Action != "user.profile_updated" {
		t.Errorf("action = %v", query.Action)
	}
	if query.TargetType == nil || *query.TargetType != "user" {
		t.Errorf("target_type = %v", query.TargetType)
	}
	if query.TargetID == nil || *query.TargetID != learnerID.String() {
		t.Errorf("target_id = %v", query.TargetID)
	}
	if query.Limit != 50 {
		t.Errorf("limit = %d, want 50", query.Limit)
	}
	if !query.Window.Start.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("window start = %s", query.Window.Start)
	}
}

func TestSearchRejectsMalformedFilters(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead))

	cases := []struct {
		query  string
		status int
	}{
		{"actor_id=not-a-uuid", http.StatusUnprocessableEntity},
		{"action=NotAnAction", http.StatusUnprocessableEntity},
		{"from=yesterday", http.StatusUnprocessableEntity},
		{"limit=0", http.StatusUnprocessableEntity},
		{"limit=101", http.StatusUnprocessableEntity},
		{"limit=many", http.StatusUnprocessableEntity},
		{"from=2026-08-09T00:00:00Z&to=2026-08-01T00:00:00Z", http.StatusUnprocessableEntity},
		{"cursor=not-a-cursor", http.StatusBadRequest},
	}
	for _, testCase := range cases {
		recorder := do(t, server, http.MethodGet, pathAuditLogs+"?"+testCase.query, "", &adminID)
		if recorder.Code != testCase.status {
			t.Errorf("?%s = %d, want %d (body %s)", testCase.query, recorder.Code, testCase.status, recorder.Body)
		}
	}
}

func TestSecurityEventSearchRejectsMalformedFilters(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead))

	for _, query := range []string{"severity=catastrophic", "resolved=maybe", "kind=NotAKind", "user_id=nope"} {
		recorder := do(t, server, http.MethodGet, pathSecurityEvents+"?"+query, "", &adminID)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Errorf("?%s = %d, want 422 (body %s)", query, recorder.Code, recorder.Body)
		}
	}
}

func TestSecurityEventSearchPassesItsFiltersThrough(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	server := newServer(trail, newFakeGuard(contract.PermissionRead))

	path := pathSecurityEvents + "?severity=high&resolved=false&kind=rbac.access_denied&user_id=" +
		learnerID.String()
	if recorder := do(t, server, http.MethodGet, path, "", &adminID); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", recorder.Code, recorder.Body)
	}

	query := trail.seenSec
	if query.Severity == nil || *query.Severity != contract.SeverityHigh {
		t.Errorf("severity = %v", query.Severity)
	}
	if query.Resolved == nil || *query.Resolved {
		t.Errorf("resolved = %v, want the open queue", query.Resolved)
	}
	if query.Kind == nil || *query.Kind != "rbac.access_denied" {
		t.Errorf("kind = %v", query.Kind)
	}
	if query.UserID == nil || *query.UserID != learnerID {
		t.Errorf("user_id = %v", query.UserID)
	}
}

// TestNextCursorRoundTripsBackIntoTheSameQuery is what makes paging work at
// all: the token the response hands out has to decode into the position the
// next request needs.
func TestNextCursorRoundTripsBackIntoTheSameQuery(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	trail.hasMore = true
	server := newServer(trail, newFakeGuard(contract.PermissionRead))

	first := do(t, server, http.MethodGet, pathAuditLogs+"?limit=1", "", &adminID)
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", first.Code, first.Body)
	}

	var page struct {
		Page struct {
			NextCursor *string `json:"next_cursor"`
			HasMore    bool    `json:"has_more"`
			Limit      int     `json:"limit"`
		} `json:"page"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !page.Page.HasMore || page.Page.NextCursor == nil {
		t.Fatalf("page = %+v, want a cursor when another page follows", page.Page)
	}

	second := do(t, server,
		http.MethodGet, pathAuditLogs+"?limit=1&cursor="+*page.Page.NextCursor, "", &adminID)
	if second.Code != http.StatusOK {
		t.Fatalf("second page status = %d (body %s)", second.Code, second.Body)
	}
	if trail.seenLog.After == nil {
		t.Fatal("the cursor did not reach the query")
	}
	if trail.seenLog.After.ID != trail.logs[0].ID {
		t.Errorf("cursor id = %s, want the last row of the previous page %s",
			trail.seenLog.After.ID, trail.logs[0].ID)
	}
	if !trail.seenLog.After.CreatedAt.Equal(trail.logs[0].CreatedAt) {
		t.Errorf("cursor time = %s, want %s", trail.seenLog.After.CreatedAt, trail.logs[0].CreatedAt)
	}
}

func TestNoCursorOnTheLastPage(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead))

	recorder := do(t, server, http.MethodGet, pathAuditLogs, "", &adminID)
	if strings.Contains(recorder.Body.String(), "next_cursor") {
		t.Errorf("the last page offered a cursor: %s", recorder.Body)
	}
}

// TestAuditLogResponseCarriesNoIPHash. The column exists; the response must not
// expose it. It is a pseudonymous identifier that correlates a person's
// activity, and reading the trail does not need it.
func TestAuditLogResponseCarriesNoIPHash(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionRead))

	for _, path := range []string{pathAuditLogs, pathSecurityEvents} {
		recorder := do(t, server, http.MethodGet, path, "", &adminID)
		if strings.Contains(recorder.Body.String(), "ip_hash") {
			t.Errorf("%s exposed ip_hash: %s", path, recorder.Body)
		}
	}
}

func TestResolveRequiresAValidBody(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionManage))
	path := pathSecurityEvents + "/" + openEvent.String() + "/resolve"

	cases := []struct {
		body   string
		status int
	}{
		{`{"note":"Triaged."}`, http.StatusOK},
		{`{}`, http.StatusUnprocessableEntity},
		{`{"note":""}`, http.StatusUnprocessableEntity},
		{`{"note":123}`, http.StatusUnprocessableEntity},
		{`{"note":"ok","reason":"extra"}`, http.StatusUnprocessableEntity},
		{`not json`, http.StatusBadRequest},
		{`{"note":"a"}{"note":"b"}`, http.StatusBadRequest},
	}
	for _, testCase := range cases {
		recorder := do(t, server, http.MethodPost, path, testCase.body, &adminID)
		if recorder.Code != testCase.status {
			t.Errorf("body %s = %d, want %d (response %s)",
				testCase.body, recorder.Code, testCase.status, recorder.Body)
		}
	}
}

// TestResolveAttributesTheClosureToTheCaller — the actor comes from the token,
// not from the body, so an administrator cannot close an event in somebody
// else's name.
func TestResolveAttributesTheClosureToTheCaller(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	server := newServer(trail, newFakeGuard(contract.PermissionManage))

	path := pathSecurityEvents + "/" + openEvent.String() + "/resolve"
	recorder := do(t, server, http.MethodPost, path, `{"note":"  Known load test.  "}`, &adminID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", recorder.Code, recorder.Body)
	}
	if trail.seenActor != adminID {
		t.Errorf("resolved_by = %s, want the caller %s", trail.seenActor, adminID)
	}
	if trail.seenEventID != openEvent {
		t.Errorf("event id = %s, want %s", trail.seenEventID, openEvent)
	}
	if trail.seenNote != "  Known load test.  " {
		t.Errorf("note = %q; trimming belongs to the domain, not the handler", trail.seenNote)
	}
}

// TestResolveWithAMalformedIDIs404 rather than 400: saying "that is not a
// uuid" tells a caller probing the surface something about its shape.
func TestResolveWithAMalformedIDIs404(t *testing.T) {
	t.Parallel()

	server := newServer(newFakeTrail(), newFakeGuard(contract.PermissionManage))

	recorder := do(t, server, http.MethodPost,
		"/api/v1/admin/security-events/not-a-uuid/resolve", `{"note":"x"}`, &adminID)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}

func TestResolveSurfacesAConflict(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	trail.resolveErr = domain.ErrAlreadyResolved
	server := newServer(trail, newFakeGuard(contract.PermissionManage))

	recorder := do(t, server, http.MethodPost,
		pathSecurityEvents+"/"+openEvent.String()+"/resolve", `{"note":"x"}`, &adminID)
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (body %s)", recorder.Code, recorder.Body)
	}
}

// TestEmptyResultsRenderAsArraysNotNull. A client that has to handle both
// `[]` and `null` for the same field will eventually handle only one.
func TestEmptyResultsRenderAsArraysNotNull(t *testing.T) {
	t.Parallel()

	trail := newFakeTrail()
	trail.logs = nil
	trail.events = nil
	server := newServer(trail, newFakeGuard(contract.PermissionRead))

	for _, path := range []string{pathAuditLogs, pathSecurityEvents} {
		recorder := do(t, server, http.MethodGet, path, "", &adminID)
		var body struct {
			Items *[]json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if body.Items == nil {
			t.Errorf("%s rendered items as null: %s", path, recorder.Body)
		}
	}
}
