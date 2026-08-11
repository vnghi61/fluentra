package service_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// The two keys a rotation involves, plus one that was never ours.
const (
	currentKey  = "current-jwt-signing-key-at-least-32-bytes"
	previousKey = "previous-jwt-signing-key-at-least-32-byt"
	foreignKey  = "somebody-elses-signing-key-32-bytes-long"
)

// Named around gosec: a constant whose name contains "token" trips G101's
// hardcoded-credential heuristic, and naming around a false positive is
// cheaper than a nolint a reader then has to evaluate.
const (
	claimIssuer   = "fluentra-test"
	claimAudience = "fluentra-api-test"
)

var tokenNow = time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)

// stubRoles answers the one question the token service asks.
type stubRoles struct {
	role string
	err  error
}

func (s stubRoles) RoleOf(context.Context, uuid.UUID) (string, error) { return s.role, s.err }

// stubDenylist is an in-memory revocation list with a switch for making it
// unreachable, which is the case the fail-open decision turns on.
type stubDenylist struct {
	denied map[string]bool
	err    error
	calls  int
}

func newStubDenylist() *stubDenylist { return &stubDenylist{denied: map[string]bool{}} }

func (s *stubDenylist) Deny(_ context.Context, tokenID string, ttl time.Duration) error {
	if s.err != nil {
		return s.err
	}
	if ttl > 0 {
		s.denied[tokenID] = true
	}
	return nil
}

func (s *stubDenylist) IsDenied(_ context.Context, tokenID string) (bool, error) {
	s.calls++
	if s.err != nil {
		return false, s.err
	}
	return s.denied[tokenID], nil
}

type tokenHarness struct {
	service  *service.TokenService
	clock    *clock.Fake
	denylist *stubDenylist
	roles    *stubRoles
}

func newTokenHarness(t *testing.T, configure func(*service.TokenConfig)) *tokenHarness {
	t.Helper()

	config := service.TokenConfig{
		SigningKey: []byte(currentKey),
		Issuer:     claimIssuer,
		Audience:   claimAudience,
		AccessTTL:  service.DefaultAccessTTL,
	}
	if configure != nil {
		configure(&config)
	}

	fakeClock := clock.NewFake(tokenNow)
	denylist := newStubDenylist()
	roles := &stubRoles{role: domain.RoleUser}

	tokens, err := service.NewTokenService(service.TokenDeps{
		Config:   config,
		Roles:    roles,
		Denylist: denylist,
		Clock:    fakeClock,
		NewID:    func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	})
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}
	return &tokenHarness{service: tokens, clock: fakeClock, denylist: denylist, roles: roles}
}

func TestNewTokenService_RefusesAKeyTooShortToSignWith(t *testing.T) {
	// A deployment that left JWT_SIGNING_KEY at a placeholder must fail to
	// start rather than issue forgeable tokens.
	if _, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{SigningKey: []byte("too-short")},
		Clock:  clock.NewFake(tokenNow),
		NewID:  func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	}); err == nil {
		t.Fatal("a nine-byte signing key was accepted")
	}

	// A short *previous* key is a half-finished rotation, not a deliberate
	// absence. Ignoring it silently would leave tokens signed with the real old
	// key unverifiable.
	if _, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{SigningKey: []byte(currentKey), PreviousKey: []byte("short")},
		Clock:  clock.NewFake(tokenNow),
		NewID:  func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	}); err == nil {
		t.Fatal("a five-byte previous key was accepted")
	}
}

func TestIssue_ThenVerifyRoundTripsTheActor(t *testing.T) {
	h := newTokenHarness(t, nil)
	userID, sessionID := uuid.New(), uuid.New()

	session, err := h.service.Issue(context.Background(), userID, sessionID)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if session.TokenType != service.TokenTypeBearer {
		t.Errorf("token type = %q, want Bearer", session.TokenType)
	}
	if session.ExpiresIn != int(service.DefaultAccessTTL.Seconds()) {
		t.Errorf("expires_in = %d, want %d", session.ExpiresIn, int(service.DefaultAccessTTL.Seconds()))
	}

	actor, err := h.service.Verify(context.Background(), session.AccessToken.Reveal())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if actor.UserID != userID {
		t.Errorf("user id = %s, want %s", actor.UserID, userID)
	}
	if actor.SessionID != sessionID {
		t.Errorf("session id = %s, want %s", actor.SessionID, sessionID)
	}
	if actor.Role != domain.RoleUser {
		t.Errorf("role = %q, want %q", actor.Role, domain.RoleUser)
	}
	if actor.TokenID == "" {
		t.Error("the token carries no jti, so nothing could ever revoke it")
	}
}

// TestIssue_ClaimsCarryNoPersonalData is the acceptance criterion, checked
// against the decoded payload rather than against the struct that built it.
//
// A JWT is base64, not encryption: every claim is readable by anyone holding
// the token, including whoever stole it. Asserting on the wire bytes is the
// only version of this test that would notice a claim added by a future change.
func TestIssue_ClaimsCarryNoPersonalData(t *testing.T) {
	h := newTokenHarness(t, nil)

	session, err := h.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	payload := decodeClaims(t, session.AccessToken.Reveal())

	// Nothing that identifies a person outside our own database.
	for _, forbidden := range []string{"email", "name", "display_name", "locale", "timezone", "@"} {
		if strings.Contains(strings.ToLower(payload), forbidden) {
			t.Errorf("the token payload contains %q: %s", forbidden, payload)
		}
	}

	// And the claims that must be there, so this cannot pass by emitting none.
	for _, required := range []string{`"sub"`, `"sid"`, `"role"`, `"jti"`, `"iat"`, `"exp"`, `"aud"`, `"iss"`} {
		if !strings.Contains(payload, required) {
			t.Errorf("the token payload is missing %s: %s", required, payload)
		}
	}
}

// TestVerify_AcceptsThePreviousKeyButNotAForeignOne is the rotation criterion.
// Both halves matter: accepting the retired key is what stops a rotation
// signing everybody out, and refusing an unknown one is the whole point of
// signing.
func TestVerify_AcceptsThePreviousKeyButNotAForeignOne(t *testing.T) {
	// A token minted before the rotation: signed with what is now the previous
	// key.
	old := newTokenHarness(t, func(c *service.TokenConfig) { c.SigningKey = []byte(previousKey) })
	session, err := old.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue with the old key: %v", err)
	}

	rotated := newTokenHarness(t, func(c *service.TokenConfig) {
		c.SigningKey = []byte(currentKey)
		c.PreviousKey = []byte(previousKey)
	})
	if _, err := rotated.service.Verify(context.Background(), session.AccessToken.Reveal()); err != nil {
		t.Fatalf("a token signed with the previous key was rejected after rotation: %v", err)
	}

	// The same deployment must still refuse a key that was never ours.
	foreign := newTokenHarness(t, func(c *service.TokenConfig) { c.SigningKey = []byte(foreignKey) })
	forged, err := foreign.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue with the foreign key: %v", err)
	}
	_, err = rotated.service.Verify(context.Background(), forged.AccessToken.Reveal())
	assertAuthCode(t, err, "TOKEN_INVALID")

	// And without a previous key configured, the old token stops working — which
	// is what makes dropping JWT_PREVIOUS_KEY the end of a rotation.
	current := newTokenHarness(t, nil)
	_, err = current.service.Verify(context.Background(), session.AccessToken.Reveal())
	assertAuthCode(t, err, "TOKEN_INVALID")
}

// TestVerify_TellsExpiredApartFromInvalid is the criterion the client acts on:
// one means refresh, the other means sign in again.
func TestVerify_TellsExpiredApartFromInvalid(t *testing.T) {
	h := newTokenHarness(t, nil)
	session, err := h.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Inside the window, plus the leeway, it still works.
	h.clock.Advance(service.DefaultAccessTTL)
	if _, err := h.service.Verify(context.Background(), session.AccessToken.Reveal()); err != nil {
		t.Fatalf("a token inside the clock leeway was rejected: %v", err)
	}

	// Past the leeway it is expired, and says so.
	h.clock.Advance(service.ClockLeeway + time.Second)
	_, err = h.service.Verify(context.Background(), session.AccessToken.Reveal())
	assertAuthCode(t, err, "TOKEN_EXPIRED")

	malformed := map[string]string{
		"empty":                 "",
		"not a token":           "hunter2",
		"two segments":          "aGVhZGVy.cGF5bG9hZA",
		"tampered signature":    session.AccessToken.Reveal() + "x",
		"tampered payload":      tamperPayload(session.AccessToken.Reveal()),
		"unsigned (alg = none)": unsignedToken(t),
	}
	for name, raw := range malformed {
		t.Run(name, func(t *testing.T) {
			_, err := h.service.Verify(context.Background(), raw)
			assertAuthCode(t, err, "TOKEN_INVALID")
		})
	}
}

// TestVerify_RefusesATokenForAnotherAudienceOrIssuer covers the two claims that
// stop a token minted by a different Fluentra deployment — staging, say — from
// working here.
func TestVerify_RefusesATokenForAnotherAudienceOrIssuer(t *testing.T) {
	elsewhere := newTokenHarness(t, func(c *service.TokenConfig) {
		c.Issuer = "some-other-fluentra"
		c.Audience = "some-other-audience"
	})
	session, err := elsewhere.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = newTokenHarness(t, nil).service.Verify(context.Background(), session.AccessToken.Reveal())
	assertAuthCode(t, err, "TOKEN_INVALID")
}

// TestVerify_RejectsADenylistedToken is explicit logout: the signature is fine
// and it has not expired, but it has been taken away.
func TestVerify_RejectsADenylistedToken(t *testing.T) {
	h := newTokenHarness(t, nil)
	session, err := h.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	actor, err := h.service.Verify(context.Background(), session.AccessToken.Reveal())
	if err != nil {
		t.Fatalf("Verify before revocation: %v", err)
	}

	if err := h.service.Revoke(context.Background(), actor, tokenNow.Add(service.DefaultAccessTTL)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = h.service.Verify(context.Background(), session.AccessToken.Reveal())
	assertAuthCode(t, err, "SESSION_REVOKED")
}

// TestVerify_AcceptsTheTokenWhenTheDenylistIsUnreachable is the one fail-open
// decision in token validation, and ADR-0007 wrote it down before this code
// existed: "a revoked user can act for up to 15 minutes unless the denylist is
// consulted". Failing closed would turn a Redis outage into a total
// authentication outage for every signed-in learner.
//
// It is asserted explicitly rather than folded into a table because it is the
// direction a future change is most likely to reverse in the name of safety.
func TestVerify_AcceptsTheTokenWhenTheDenylistIsUnreachable(t *testing.T) {
	h := newTokenHarness(t, nil)
	session, err := h.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	h.denylist.err = errors.New("dial tcp: connection refused")

	if _, err := h.service.Verify(context.Background(), session.AccessToken.Reveal()); err != nil {
		t.Fatalf("an unreachable denylist rejected a valid token: %v", err)
	}
	if h.denylist.calls == 0 {
		t.Error("the denylist was never consulted, so this proves nothing")
	}
}

// TestIssue_FailsRatherThanDowngradeTheRole matters because the alternative is
// silent. Minting a `user` token for an administrator whose roles could not be
// read would strip their access, and it would surface hours later as a
// permissions bug rather than now as a failed login.
func TestIssue_FailsRatherThanDowngradeTheRole(t *testing.T) {
	h := newTokenHarness(t, nil)
	h.roles.err = errors.New("database unavailable")

	if _, err := h.service.Issue(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("a token was issued despite the role lookup failing")
	}
}

func TestIssue_CarriesTheAdminRoleWhenTheAccountHoldsIt(t *testing.T) {
	h := newTokenHarness(t, nil)
	h.roles.role = domain.RoleAdmin

	session, err := h.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	actor, err := h.service.Verify(context.Background(), session.AccessToken.Reveal())
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if actor.Role != domain.RoleAdmin {
		t.Errorf("role = %q, want admin", actor.Role)
	}
}

func TestHighestRole(t *testing.T) {
	cases := map[string]struct {
		roles []string
		want  string
	}{
		"none":         {nil, domain.RoleUser},
		"user only":    {[]string{domain.RoleUser}, domain.RoleUser},
		"admin only":   {[]string{domain.RoleAdmin}, domain.RoleAdmin},
		"both":         {[]string{domain.RoleUser, domain.RoleAdmin}, domain.RoleAdmin},
		"both, sorted": {[]string{domain.RoleAdmin, domain.RoleUser}, domain.RoleAdmin},
		"unknown":      {[]string{"moderator"}, domain.RoleUser},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := domain.HighestRole(testCase.roles); got != testCase.want {
				t.Errorf("HighestRole(%v) = %q, want %q", testCase.roles, got, testCase.want)
			}
		})
	}
}

// TestSession_AccessTokenIsNotPrintedByAccident is the wrapper doing its job on
// a bearer credential. The Session struct travels through log lines and test
// failure messages on its way to the transport layer.
func TestSession_AccessTokenIsNotPrintedByAccident(t *testing.T) {
	h := newTokenHarness(t, nil)
	session, err := h.service.Issue(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if rendered := formatValue(session); strings.Contains(rendered, session.AccessToken.Reveal()) {
		t.Errorf("formatting a Session printed the access token: %s", rendered)
	}
}

// decodeClaims returns the token's payload segment as JSON text.
func decodeClaims(t *testing.T, raw string) string {
	t.Helper()

	segments := strings.Split(raw, ".")
	if len(segments) != 3 {
		t.Fatalf("token has %d segments, want 3", len(segments))
	}
	decoded, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	return string(decoded)
}

// tamperPayload rewrites one character of the claims segment, leaving the
// signature in place — the shape a forgery attempt actually takes.
func tamperPayload(raw string) string {
	segments := strings.Split(raw, ".")
	if len(segments) != 3 || segments[1] == "" {
		return raw
	}
	body := []byte(segments[1])
	if body[0] == 'A' {
		body[0] = 'B'
	} else {
		body[0] = 'A'
	}
	return segments[0] + "." + string(body) + "." + segments[2]
}

// unsignedToken builds an `alg: none` token, the classic algorithm-substitution
// attack. It must be refused, and it is refused by pinning the accepted method
// rather than by trusting the header.
func unsignedToken(t *testing.T) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(
		`{"sub":"` + uuid.New().String() + `","sid":"` + uuid.New().String() +
			`","role":"admin","jti":"x","iss":"` + claimIssuer + `","aud":"` + claimAudience +
			`","exp":99999999999}`))
	return header + "." + claims + "."
}

func assertAuthCode(t *testing.T, err error, wanted string) {
	t.Helper()

	if err == nil {
		t.Fatalf("no error, want %s", wanted)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want an *apperr.Error with code %s", err, wanted)
	}
	if appErr.Code != wanted {
		t.Fatalf("code = %q, want %q", appErr.Code, wanted)
	}
}

// formatValue renders a value the way an accidental log line would.
func formatValue(value any) string { return fmt.Sprintf("%+v %v", value, value) }
