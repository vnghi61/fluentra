package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// The 404 boundary and the family revocation are proved against PostgreSQL in
// the module integration suite, because they are properties of what the SQL
// does. What is here is the cache — which the integration suite deliberately
// runs without, so that the rows it asserts on are the rows Postgres holds —
// and the failure branches a working database cannot produce.

type fakeSessionRepo struct {
	sessions map[uuid.UUID]domain.Session

	listErr      error
	getErr       error
	revokeErr    error
	familyErr    error
	getCalls     int
	familyCall   int
	revoked      []uuid.UUID
	untrustedAll int
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[uuid.UUID]domain.Session{}}
}

func (f *fakeSessionRepo) WithTx(pgx.Tx) service.SessionRepo { return f }

func (f *fakeSessionRepo) ListLiveSessions(_ context.Context, userID uuid.UUID) ([]domain.Session, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var live []domain.Session
	for _, session := range f.sessions {
		if session.UserID == userID && !session.Revoked() {
			live = append(live, session)
		}
	}
	return live, nil
}

// GetOwnedSession mirrors the SQL clause for clause: both the id and the owner
// are in the predicate, so a session that exists but belongs to somebody else is
// not found. A fake that looked up by id and compared the owner afterwards would
// be more permissive than the query and would pass a test the real system fails.
func (f *fakeSessionRepo) GetOwnedSession(_ context.Context, sessionID, userID uuid.UUID) (
	domain.Session, bool, error,
) {
	f.getCalls++
	if f.getErr != nil {
		return domain.Session{}, false, f.getErr
	}
	session, ok := f.sessions[sessionID]
	if !ok || session.UserID != userID {
		return domain.Session{}, false, nil
	}
	return session, true, nil
}

func (f *fakeSessionRepo) RevokeSession(_ context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	if f.revokeErr != nil {
		return false, f.revokeErr
	}
	session, ok := f.sessions[sessionID]
	if !ok || session.Revoked() {
		return false, nil
	}
	session.RevokedAt = &now
	f.sessions[sessionID] = session
	f.revoked = append(f.revoked, sessionID)
	return true, nil
}

func (f *fakeSessionRepo) RevokeAllSessionsForUser(_ context.Context, userID uuid.UUID, now time.Time) (int, error) {
	count := 0
	for id, session := range f.sessions {
		if session.UserID == userID && !session.Revoked() {
			session.RevokedAt = &now
			f.sessions[id] = session
			count++
		}
	}
	return count, nil
}

func (f *fakeSessionRepo) RevokeOtherSessionsForUser(
	_ context.Context, userID, keepSessionID uuid.UUID, now time.Time,
) (int, error) {
	count := 0
	for id, session := range f.sessions {
		if session.UserID == userID && id != keepSessionID && !session.Revoked() {
			session.RevokedAt = &now
			f.sessions[id] = session
			count++
		}
	}
	return count, nil
}

func (f *fakeSessionRepo) RevokeRefreshTokensForOtherSessions(
	_ context.Context, _, _ uuid.UUID, _ time.Time,
) (int, error) {
	if f.familyErr != nil {
		return 0, f.familyErr
	}
	f.familyCall++
	return 1, nil
}

func (f *fakeSessionRepo) RevokeRefreshTokensBySession(_ context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	if f.familyErr != nil {
		return 0, f.familyErr
	}
	f.familyCall++
	return 1, nil
}

func (f *fakeSessionRepo) UntrustAllDevicesForUser(
	_ context.Context, _ uuid.UUID, _ time.Time,
) (int, error) {
	f.untrustedAll++
	return 1, nil
}

func (f *fakeSessionRepo) RevokeRefreshTokensForUser(_ context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	if f.familyErr != nil {
		return 0, f.familyErr
	}
	f.familyCall++
	return 1, nil
}

// fakeOwnerCache counts reads and writes so a test can prove the second lookup
// did not reach the repository.
type fakeOwnerCache struct {
	entries map[string]uuid.UUID
	getErr  error
	setErr  error
	delErr  error
	deletes []string
}

func newFakeOwnerCache() *fakeOwnerCache {
	return &fakeOwnerCache{entries: map[string]uuid.UUID{}}
}

func (f *fakeOwnerCache) Get(_ context.Context, key string) (uuid.UUID, error) {
	if f.getErr != nil {
		return uuid.Nil, f.getErr
	}
	owner, ok := f.entries[key]
	if !ok {
		return uuid.Nil, errors.New("miss")
	}
	return owner, nil
}

func (f *fakeOwnerCache) Set(_ context.Context, key string, value uuid.UUID, _ time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.entries[key] = value
	return nil
}

func (f *fakeOwnerCache) Delete(_ context.Context, keys ...string) error {
	f.deletes = append(f.deletes, keys...)
	if f.delErr != nil {
		return f.delErr
	}
	for _, key := range keys {
		delete(f.entries, key)
	}
	return nil
}

// fakeAccessRevoker records the denylisting, and can refuse it.
type fakeAccessRevoker struct {
	calls int
	err   error
}

func (f *fakeAccessRevoker) RevokeNow(context.Context, httpx.Actor) error {
	f.calls++
	return f.err
}

type sessionServiceHarness struct {
	service  *service.SessionService
	repo     *fakeSessionRepo
	cache    *fakeOwnerCache
	revoker  *fakeAccessRevoker
	pool     *fakePool
	userID   uuid.UUID
	current  uuid.UUID
	otherOne uuid.UUID
}

// testEnv namespaces the cache keys these tests assert on, and testJTI is the
// token id a real actor carries. Both are constants because they appear in most
// of the cases below and a typo in either would assert nothing.
const (
	testEnv = "test"
	testJTI = "jti"

	// The key shape this module's AGENT.md §12 documents, environment included.
	sessionKeyPrefix = "fluentra:" + testEnv + ":auth:session:"
)

var sessionNow = time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)

func newSessionServiceHarness(t *testing.T) *sessionServiceHarness {
	t.Helper()

	repo := newFakeSessionRepo()
	cacheStore := newFakeOwnerCache()
	revoker := &fakeAccessRevoker{}
	pool := &fakePool{}

	userID, current, other := uuid.New(), uuid.New(), uuid.New()
	repo.sessions[current] = domain.Session{
		ID: current, UserID: userID, CreatedAt: sessionNow, LastSeenAt: sessionNow,
	}
	repo.sessions[other] = domain.Session{
		ID: other, UserID: userID, CreatedAt: sessionNow, LastSeenAt: sessionNow.Add(-time.Hour),
	}

	sessions := service.NewSessionService(service.SessionDeps{
		Pool:   pool,
		Repo:   repo,
		Tokens: revoker,
		Cache:  cacheStore,
		Clock:  clock.NewFake(sessionNow),
		Env:    testEnv,
	})

	return &sessionServiceHarness{
		service: sessions, repo: repo, cache: cacheStore, revoker: revoker, pool: pool,
		userID: userID, current: current, otherOne: other,
	}
}

// TestRevoke_CachesTheOwnerAndDropsItWhenTheSessionDies is the cache doing the
// job it exists for, and stopping when it should.
//
// The key is the one AGENT.md §12 documents, namespaced by environment so a
// staging deploy pointed at a shared Redis cannot answer a production question.
func TestRevoke_CachesTheOwnerAndDropsItWhenTheSessionDies(t *testing.T) {
	h := newSessionServiceHarness(t)
	actor := httpx.Actor{UserID: h.userID, SessionID: h.current, TokenID: testJTI}
	ctx := context.Background()

	if err := h.service.Revoke(ctx, actor, h.otherOne); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if h.repo.getCalls != 1 {
		t.Errorf("%d ownership reads, want 1", h.repo.getCalls)
	}

	key := sessionKeyPrefix + h.otherOne.String() + ":v1"
	if len(h.cache.deletes) != 1 || h.cache.deletes[0] != key {
		t.Errorf("cache deletes = %v, want [%s]", h.cache.deletes, key)
	}
	if _, still := h.cache.entries[key]; still {
		t.Error("the cache still answers for a session that has been revoked")
	}
}

// TestRevoke_AnsweredFromCacheDoesNotReachTheDatabase proves the cache is
// actually consulted — otherwise the previous test would pass against a service
// that writes entries nothing ever reads.
func TestRevoke_AnsweredFromCacheDoesNotReachTheDatabase(t *testing.T) {
	h := newSessionServiceHarness(t)
	actor := httpx.Actor{UserID: h.userID, SessionID: h.current, TokenID: testJTI}
	ctx := context.Background()

	h.cache.entries[sessionKeyPrefix+h.otherOne.String()+":v1"] = h.userID

	if err := h.service.Revoke(ctx, actor, h.otherOne); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if h.repo.getCalls != 0 {
		t.Errorf("%d ownership reads, want 0 — the cached answer was ignored", h.repo.getCalls)
	}

	// The revocation itself still went to the database. This is the line
	// between what is cached and what is not: ownership never changes, so a
	// cached answer is safe; liveness does, so it is never cached and the
	// guarded UPDATE remains the only thing that decides it.
	if len(h.repo.revoked) != 1 || h.repo.revoked[0] != h.otherOne {
		t.Errorf("revoked = %v, want the revocation to have reached Postgres", h.repo.revoked)
	}
}

// TestRevoke_ACachedOwnerCannotUnlockAnotherAccountsSession is the failure the
// cache could introduce if it were keyed or compared carelessly.
func TestRevoke_ACachedOwnerCannotUnlockAnotherAccountsSession(t *testing.T) {
	h := newSessionServiceHarness(t)
	stranger := uuid.New()
	ctx := context.Background()

	h.cache.entries[sessionKeyPrefix+h.otherOne.String()+":v1"] = h.userID

	err := h.service.Revoke(ctx, httpx.Actor{UserID: stranger, SessionID: uuid.New()}, h.otherOne)
	assertAuthCode(t, err, "RESOURCE_NOT_FOUND")

	if len(h.repo.revoked) != 0 {
		t.Error("a stranger revoked a session that the cache said belonged to somebody else")
	}
}

// TestRevoke_AnUnreachableCacheIsNotAFailure keeps a Redis outage from stopping
// a learner signing a device out. The answer is in Postgres either way.
func TestRevoke_AnUnreachableCacheIsNotAFailure(t *testing.T) {
	h := newSessionServiceHarness(t)
	h.cache.getErr = errors.New("dial tcp: connection refused")
	h.cache.setErr = errors.New("dial tcp: connection refused")
	h.cache.delErr = errors.New("dial tcp: connection refused")

	actor := httpx.Actor{UserID: h.userID, SessionID: h.current, TokenID: testJTI}
	if err := h.service.Revoke(context.Background(), actor, h.otherOne); err != nil {
		t.Fatalf("an unreachable cache failed the revocation: %v", err)
	}
	if len(h.repo.revoked) != 1 {
		t.Error("the revocation did not reach the database")
	}
}

// TestRevoke_ANegativeAnswerIsNotCached matters because a session created a
// moment later would otherwise be refused for the whole TTL.
func TestRevoke_ANegativeAnswerIsNotCached(t *testing.T) {
	h := newSessionServiceHarness(t)
	unknown := uuid.New()
	actor := httpx.Actor{UserID: h.userID, SessionID: h.current}

	err := h.service.Revoke(context.Background(), actor, unknown)
	assertAuthCode(t, err, "RESOURCE_NOT_FOUND")

	if _, cached := h.cache.entries[sessionKeyPrefix+unknown.String()+":v1"]; cached {
		t.Error("a not-found answer was cached, so a session created later would be refused for five minutes")
	}
}

// TestLogoutAndRevoke_DenylistTheAccessTokenOnlyForTheCallersOwnSession is the
// distinction the acceptance criterion turns on. Revoking some other device does
// not put us in possession of its access token; there is no id to deny, and it
// stops working at its own expiry.
func TestLogoutAndRevoke_DenylistTheAccessTokenOnlyForTheCallersOwnSession(t *testing.T) {
	h := newSessionServiceHarness(t)
	actor := httpx.Actor{UserID: h.userID, SessionID: h.current, TokenID: testJTI}
	ctx := context.Background()

	if err := h.service.Revoke(ctx, actor, h.otherOne); err != nil {
		t.Fatalf("Revoke another device: %v", err)
	}
	if h.revoker.calls != 0 {
		t.Errorf("%d denylist writes for another device's session, want 0", h.revoker.calls)
	}

	if err := h.service.Revoke(ctx, actor, h.current); err != nil {
		t.Fatalf("Revoke the current session: %v", err)
	}
	if h.revoker.calls != 1 {
		t.Errorf("%d denylist writes after revoking the current session, want 1", h.revoker.calls)
	}

	if err := h.service.Logout(ctx, actor); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if h.revoker.calls != 2 {
		t.Errorf("%d denylist writes after logout, want 2", h.revoker.calls)
	}
}

// TestLogout_SucceedsWhenTheDenylistRefuses is the fail-open decision, asserted
// where it is made. The durable half has already committed; refusing the logout
// would report failure for something that has, in every lasting way, happened.
func TestLogout_SucceedsWhenTheDenylistRefuses(t *testing.T) {
	h := newSessionServiceHarness(t)
	h.revoker.err = errors.New("dial tcp: connection refused")

	actor := httpx.Actor{UserID: h.userID, SessionID: h.current, TokenID: testJTI}
	if err := h.service.Logout(context.Background(), actor); err != nil {
		t.Fatalf("logout failed because the denylist was unreachable: %v", err)
	}
	if len(h.repo.revoked) != 1 {
		t.Error("the session was not revoked")
	}
}

// TestRevoke_ReportsAFailureRatherThanClaimingSuccess covers the direction the
// database errors have to fail in. Returning nil would tell the learner a device
// was signed out while it went on renewing.
func TestRevoke_ReportsAFailureRatherThanClaimingSuccess(t *testing.T) {
	for name, brk := range map[string]func(*sessionServiceHarness){
		"the ownership read fails":      func(h *sessionServiceHarness) { h.repo.getErr = errors.New("down") },
		"the family cannot be revoked":  func(h *sessionServiceHarness) { h.repo.familyErr = errors.New("down") },
		"the session cannot be revoked": func(h *sessionServiceHarness) { h.repo.revokeErr = errors.New("down") },
		"the transaction cannot open":   func(h *sessionServiceHarness) { h.pool.beginErr = errors.New("down") },
		"the transaction cannot commit": func(h *sessionServiceHarness) { h.pool.commitErr = errors.New("down") },
	} {
		t.Run(name, func(t *testing.T) {
			h := newSessionServiceHarness(t)
			brk(h)

			actor := httpx.Actor{UserID: h.userID, SessionID: h.current, TokenID: testJTI}
			if err := h.service.Revoke(context.Background(), actor, h.otherOne); err == nil {
				t.Fatal("a failed revocation was reported as a success")
			}
		})
	}
}

// TestList_MarksTheCallersOwnSessionAndNothingElse is the flag an interface needs
// to warn before signing the learner out of the device in their hand.
func TestList_MarksTheCallersOwnSessionAndNothingElse(t *testing.T) {
	h := newSessionServiceHarness(t)

	listed, err := h.service.List(context.Background(),
		httpx.Actor{UserID: h.userID, SessionID: h.current})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("%d sessions, want 2", len(listed))
	}

	current := 0
	for _, session := range listed {
		if session.Current {
			current++
			if session.ID != h.current {
				t.Errorf("session %s is marked current, want %s", session.ID, h.current)
			}
		}
	}
	if current != 1 {
		t.Errorf("%d sessions marked current, want exactly 1", current)
	}
}

func TestList_ReportsAFailureRatherThanAnEmptyList(t *testing.T) {
	h := newSessionServiceHarness(t)
	h.repo.listErr = errors.New("connection refused")

	// An empty list and an unreadable one mean opposite things to a learner
	// checking where they are signed in, and only one of them is safe to show.
	if _, err := h.service.List(context.Background(),
		httpx.Actor{UserID: h.userID, SessionID: h.current}); err == nil {
		t.Fatal("an unreadable list was reported as no sessions")
	}
}

// TestRevokeAll_DropsEveryCachedOwnerItRevoked stops the cache holding answers
// about rows that are now dead.
func TestRevokeAll_DropsEveryCachedOwnerItRevoked(t *testing.T) {
	h := newSessionServiceHarness(t)

	revoked, err := h.service.RevokeAll(context.Background(), h.userID)
	if err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}
	if revoked != 2 {
		t.Errorf("%d sessions revoked, want 2", revoked)
	}
	if len(h.cache.deletes) != 2 {
		t.Errorf("%d cache entries dropped, want one per revoked session", len(h.cache.deletes))
	}
	for _, key := range h.cache.deletes {
		if !strings.HasPrefix(key, sessionKeyPrefix) {
			t.Errorf("dropped an unexpected key: %s", key)
		}
	}
}

// TestRevokeAllExcept_KeepsTheDeviceTheChangeWasMadeFrom is what a password
// change relies on. Signing a learner out of the machine in front of them,
// immediately after they did the responsible thing, teaches them not to do it
// again — and the cache entry for the kept session must survive too, because
// the session has not died.
func TestRevokeAllExcept_KeepsTheDeviceTheChangeWasMadeFrom(t *testing.T) {
	h := newSessionServiceHarness(t)

	revoked, err := h.service.RevokeAllExcept(context.Background(), h.userID, h.current)
	if err != nil {
		t.Fatalf("RevokeAllExcept: %v", err)
	}
	if revoked != 1 {
		t.Errorf("%d sessions revoked, want 1 — the other device only", revoked)
	}

	if h.repo.sessions[h.current].Revoked() {
		t.Error("the kept session was revoked")
	}
	if !h.repo.sessions[h.otherOne].Revoked() {
		t.Error("the other session survived")
	}

	keptKey := sessionKeyPrefix + h.current.String() + ":v1"
	for _, dropped := range h.cache.deletes {
		if dropped == keptKey {
			t.Error("the kept session's cache entry was dropped, for a session that is still live")
		}
	}
}

// TestRevokeAll_AlsoUntrustsEveryDevice is BR-AUTH-25. A device that stayed
// trusted through a password reset would be a ninety-day window the attacker
// keeps, which is the opposite of what the learner asked for.
func TestRevokeAll_AlsoUntrustsEveryDevice(t *testing.T) {
	h := newSessionServiceHarness(t)

	if _, err := h.service.RevokeAll(context.Background(), h.userID); err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}
	if h.repo.untrustedAll == 0 {
		t.Error("a full revocation left every trusted device in place")
	}

	// A change keeps this session and still untrusts the browser it runs in:
	// the learner keeps the session, not the standing ninety-day permission.
	other := newSessionServiceHarness(t)
	if _, err := other.service.RevokeAllExcept(context.Background(), other.userID, other.current); err != nil {
		t.Fatalf("RevokeAllExcept: %v", err)
	}
	if other.repo.untrustedAll == 0 {
		t.Error("a password change left every trusted device in place")
	}
}
