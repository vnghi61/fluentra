//go:build integration

package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// sessionHarness wires the session service over the same pool and the same
// refresh service, because the property that matters most here is the one that
// crosses the two: revoking a session must stop its refresh family renewing.
type sessionHarness struct {
	*refreshHarness

	sessions *service.SessionService
	denylist *stubDenylist
}

// stubDenylist stands in for Redis. In-memory, because what these tests assert
// is that the token id was denied, not that Redis stored it.
type stubDenylist struct {
	mu     sync.Mutex
	denied map[string]bool
	err    error
}

func newStubDenylist() *stubDenylist { return &stubDenylist{denied: map[string]bool{}} }

func (s *stubDenylist) Deny(_ context.Context, tokenID string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if ttl > 0 {
		s.denied[tokenID] = true
	}
	return nil
}

func (s *stubDenylist) IsDenied(_ context.Context, tokenID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.denied[tokenID], s.err
}

func (s *stubDenylist) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// sessionAdapter narrows *repository.Repository to service.SessionRepo, the
// same covariance bridge module.go needs.
type sessionAdapter struct {
	*repository.Repository
}

func (a sessionAdapter) WithTx(tx pgx.Tx) service.SessionRepo {
	return sessionAdapter{Repository: a.Repository.WithTx(tx)}
}

func newSessionHarness(t *testing.T, email string) *sessionHarness {
	t.Helper()

	denylist := newStubDenylist()
	base := newRefreshHarness(t, email, denylist)

	sessions := service.NewSessionService(service.SessionDeps{
		Pool:   pool,
		Repo:   sessionAdapter{Repository: repository.New(pool)},
		Tokens: base.tokens,
		// Nil cache: these tests are about the rows. The cache and its
		// invalidation are exercised in the service unit suite, where a stub
		// can be made to hold a stale value on purpose.
		Cache: nil,
		Clock: base.clock,
	})

	return &sessionHarness{refreshHarness: base, sessions: sessions, denylist: denylist}
}

// actorOf builds the caller the middleware would have put in the context.
func actorOf(userID, sessionID uuid.UUID) httpx.Actor {
	return httpx.Actor{
		UserID: userID, SessionID: sessionID, Role: domain.RoleUser, TokenID: uuid.NewString(),
	}
}

// TestAnotherAccountsSessionIsANotFoundNotAForbidden is the trap this card is
// most likely to fall into, and the two answers are not interchangeable.
//
// 403 confirms the id names a real session, which turns this operation into an
// oracle: a caller with one account can walk ids and learn which exist. 404 says
// only that *the caller* has no such session — which is all they are entitled to
// know, and is equally true of an id that never existed. Both halves are
// asserted, because the defence is that the two are indistinguishable, not
// merely that one of them is refused.
func TestAnotherAccountsSessionIsANotFoundNotAForbidden(t *testing.T) {
	mine := newSessionHarness(t, "owner@fluentra.test")
	theirs := newSessionHarness(t, "stranger@fluentra.test")
	ctx := context.Background()

	victim := theirs.start(t)

	realButNotMine := mine.sessions.Revoke(ctx, actorOf(mine.userID, uuid.New()), victim.Session.SessionID)
	assertCode(t, realButNotMine, "RESOURCE_NOT_FOUND")

	neverExisted := mine.sessions.Revoke(ctx, actorOf(mine.userID, uuid.New()), uuid.New())
	assertCode(t, neverExisted, "RESOURCE_NOT_FOUND")

	if sessionIsRevoked(t, victim.Session.SessionID) {
		t.Fatal("one account revoked another account's session")
	}
	if _, err := theirs.service.Rotate(ctx, victim.RefreshToken.Reveal()); err != nil {
		t.Errorf("the victim's session stopped working: %v", err)
	}

	// The same id, presented by its actual owner, must work — otherwise this
	// test would pass against an implementation that refuses everything.
	if err := theirs.sessions.Revoke(ctx, actorOf(theirs.userID, uuid.New()),
		victim.Session.SessionID); err != nil {
		t.Fatalf("the owner could not revoke their own session: %v", err)
	}
}

// TestRevokingASessionStopsItsRefreshFamilyImmediately is the acceptance
// criterion, and "immediately" is the word doing the work. The access token
// survives to its own expiry by design (ADR-0007); the family must not. If it
// could still rotate, a revoked session would renew itself indefinitely and
// revocation would mean nothing at all.
func TestRevokingASessionStopsItsRefreshFamilyImmediately(t *testing.T) {
	h := newSessionHarness(t, "revoke-family@fluentra.test")
	ctx := context.Background()

	signedIn := h.start(t)
	actor := actorOf(h.userID, uuid.New())

	// It rotates before the revocation, so a failure afterwards is the
	// revocation's doing and not a token that never worked.
	rotated, err := h.service.Rotate(ctx, signedIn.RefreshToken.Reveal())
	if err != nil {
		t.Fatalf("Rotate before revocation: %v", err)
	}

	if err := h.sessions.Revoke(ctx, actor, signedIn.Session.SessionID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = h.service.Rotate(ctx, rotated.RefreshToken.Reveal())
	assertCode(t, err, "SESSION_REVOKED")

	if live := liveTokensInFamily(t, signedIn.Session.SessionID); live != 0 {
		t.Errorf("%d refresh tokens still exchangeable after revocation, want 0", live)
	}
	if !sessionIsRevoked(t, signedIn.Session.SessionID) {
		t.Error("the session row was not marked revoked")
	}

	// Revoking twice is not an error: a client retrying a dropped request
	// should not be told something went wrong.
	if err := h.sessions.Revoke(ctx, actor, signedIn.Session.SessionID); err != nil {
		t.Errorf("the second revocation was refused: %v", err)
	}
}

// TestTheSessionListShowsOnlyLiveSessionsOfTheCallersOwnAccount covers the read
// side of the same boundary, plus the `current` flag an interface needs in order
// to warn before signing the learner out of the device they are holding.
func TestTheSessionListShowsOnlyLiveSessionsOfTheCallersOwnAccount(t *testing.T) {
	mine := newSessionHarness(t, "list-owner@fluentra.test")
	theirs := newSessionHarness(t, "list-stranger@fluentra.test")
	ctx := context.Background()

	first := mine.start(t)
	second := mine.start(t)
	stranger := theirs.start(t)

	listed, err := mine.sessions.List(ctx, actorOf(mine.userID, first.Session.SessionID))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("%d sessions listed, want 2", len(listed))
	}

	seen := map[uuid.UUID]bool{}
	for _, session := range listed {
		seen[session.ID] = true
		if session.ID == stranger.Session.SessionID {
			t.Error("another account's session appeared in the list")
		}
		if session.ID == first.Session.SessionID && !session.Current {
			t.Error("the session making the request is not marked current")
		}
		if session.ID == second.Session.SessionID && session.Current {
			t.Error("a session other than the caller's is marked current")
		}
	}
	if !seen[first.Session.SessionID] || !seen[second.Session.SessionID] {
		t.Errorf("the list is missing one of the caller's own sessions: %v", seen)
	}

	// A revoked session leaves the list. The question it answers is "where am I
	// signed in now", not "where have I ever been".
	if err := mine.sessions.Revoke(ctx, actorOf(mine.userID, first.Session.SessionID),
		second.Session.SessionID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	listed, err = mine.sessions.List(ctx, actorOf(mine.userID, first.Session.SessionID))
	if err != nil {
		t.Fatalf("List after revocation: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != first.Session.SessionID {
		t.Errorf("the revoked session is still listed: %+v", listed)
	}
}

// TestLogoutDenylistsTheAccessTokenAndStopsTheFamily is what separates logout
// from revoking some other device: the caller is holding the token, so its id is
// known and it can be stopped now rather than at its expiry.
func TestLogoutDenylistsTheAccessTokenAndStopsTheFamily(t *testing.T) {
	h := newSessionHarness(t, "logout@fluentra.test")
	ctx := context.Background()

	signedIn := h.start(t)
	actor, err := h.tokens.Verify(ctx, signedIn.Session.AccessToken.Reveal())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if err := h.sessions.Logout(ctx, actor); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	// Now, not in fifteen minutes.
	_, err = h.tokens.Verify(ctx, signedIn.Session.AccessToken.Reveal())
	assertCode(t, err, "SESSION_REVOKED")

	_, err = h.service.Rotate(ctx, signedIn.RefreshToken.Reveal())
	assertCode(t, err, "SESSION_REVOKED")

	if !sessionIsRevoked(t, signedIn.Session.SessionID) {
		t.Error("logout left the session row live")
	}

	if err := h.sessions.Logout(ctx, actor); err != nil {
		t.Errorf("the second logout was refused: %v", err)
	}
}

// TestLogoutStillEndsTheSessionWhenTheDenylistIsUnreachable is the fail-open
// decision meeting its limit. An access token surviving a Redis outage is
// ADR-0007's accepted trade; the *session* surviving is not, because it lives in
// Postgres and has no reason to depend on Redis being up.
func TestLogoutStillEndsTheSessionWhenTheDenylistIsUnreachable(t *testing.T) {
	h := newSessionHarness(t, "logout-degraded@fluentra.test")
	ctx := context.Background()

	signedIn := h.start(t)
	actor, err := h.tokens.Verify(ctx, signedIn.Session.AccessToken.Reveal())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	h.denylist.fail(errors.New("dial tcp: connection refused"))

	if err := h.sessions.Logout(ctx, actor); err != nil {
		t.Fatalf("logout failed because the denylist was unreachable: %v", err)
	}
	if !sessionIsRevoked(t, signedIn.Session.SessionID) {
		t.Error("an unreachable denylist left the session live")
	}

	_, err = h.service.Rotate(ctx, signedIn.RefreshToken.Reveal())
	assertCode(t, err, "SESSION_REVOKED")
}

// TestRevokeAllEndsEverySessionTheAccountHas is the contract method `user` and
// `admin` consume on deletion and suspension. It is asserted here rather than
// left to those callers because neither module exists yet.
func TestRevokeAllEndsEverySessionTheAccountHas(t *testing.T) {
	h := newSessionHarness(t, "revoke-all@fluentra.test")
	other := newSessionHarness(t, "revoke-all-bystander@fluentra.test")
	ctx := context.Background()

	first := h.start(t)
	second := h.start(t)
	bystander := other.start(t)

	revoked, err := h.sessions.RevokeAll(ctx, h.userID)
	if err != nil {
		t.Fatalf("RevokeAll: %v", err)
	}
	if revoked != 2 {
		t.Errorf("%d sessions revoked, want 2", revoked)
	}

	for _, signedIn := range []service.SignedIn{first, second} {
		if !sessionIsRevoked(t, signedIn.Session.SessionID) {
			t.Errorf("session %s survived RevokeAll", signedIn.Session.SessionID)
		}
		if _, err := h.service.Rotate(ctx, signedIn.RefreshToken.Reveal()); err == nil {
			t.Errorf("session %s can still rotate after RevokeAll", signedIn.Session.SessionID)
		}
	}

	if sessionIsRevoked(t, bystander.Session.SessionID) {
		t.Error("RevokeAll reached another account")
	}
}
