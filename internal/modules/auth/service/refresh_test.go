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
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// The rotation rules themselves are proved against PostgreSQL, because they are
// rules about what a guarded UPDATE does under concurrency and a fake would
// answer whatever its author expected. See internal/modules/auth's integration
// suite.
//
// What is here is the half Postgres cannot reach: the branches that only open
// when a collaborator fails. A database that goes away mid-rotation, an entropy
// source that cannot fill a token, an id generator that errors. Each of those
// has exactly one correct direction to fail in, and none of them can be
// provoked from the integration suite without breaking the database on purpose.

// fakeRefreshRepo records what it was asked to do and can be made to fail one
// call at a time.
type fakeRefreshRepo struct {
	tokens   map[string]domain.SessionToken
	sessions map[uuid.UUID]bool // id -> revoked

	claim  func() (domain.SessionToken, bool, error)
	findEr error

	createSessionErr error
	createTokenErr   error
	touchErr         error
	revokeFamilyErr  error
	revokeSessionErr error

	revokedFamilies []uuid.UUID
	revokedSessions []uuid.UUID
	createdTokens   int
	touches         int

	// What CreateSession was handed, so a test can assert the address was
	// digested rather than stored.
	lastIPHash        []byte
	lastUserAgentHash []byte
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{
		tokens:   map[string]domain.SessionToken{},
		sessions: map[uuid.UUID]bool{},
	}
}

func (f *fakeRefreshRepo) WithTx(pgx.Tx) service.RefreshRepo { return f }

func (f *fakeRefreshRepo) CreateSession(
	_ context.Context, id, _ uuid.UUID, ipHash, userAgentHash []byte, _ time.Time,
) error {
	if f.createSessionErr != nil {
		return f.createSessionErr
	}
	f.sessions[id] = false
	f.lastIPHash, f.lastUserAgentHash = ipHash, userAgentHash
	return nil
}

func (f *fakeRefreshRepo) TouchSession(_ context.Context, _ uuid.UUID, _ time.Time) error {
	if f.touchErr != nil {
		return f.touchErr
	}
	f.touches++
	return nil
}

func (f *fakeRefreshRepo) RevokeSession(_ context.Context, sessionID uuid.UUID, _ time.Time) (bool, error) {
	if f.revokeSessionErr != nil {
		return false, f.revokeSessionErr
	}
	f.revokedSessions = append(f.revokedSessions, sessionID)
	already := f.sessions[sessionID]
	f.sessions[sessionID] = true
	return !already, nil
}

func (f *fakeRefreshRepo) CreateRefreshToken(
	_ context.Context, id uuid.UUID, tokenHash []byte, familyID, sessionID uuid.UUID, now, expiresAt time.Time,
) (domain.RefreshToken, error) {
	if f.createTokenErr != nil {
		return domain.RefreshToken{}, f.createTokenErr
	}
	f.createdTokens++
	token := domain.RefreshToken{
		ID: id, TokenHash: tokenHash, FamilyID: familyID, SessionID: sessionID,
		IssuedAt: now, ExpiresAt: expiresAt,
	}
	f.tokens[string(tokenHash)] = domain.SessionToken{RefreshToken: token}
	return token, nil
}

func (f *fakeRefreshRepo) ClaimRefreshToken(_ context.Context, _ []byte, _ time.Time) (
	domain.SessionToken, bool, error,
) {
	if f.claim != nil {
		return f.claim()
	}
	return domain.SessionToken{}, false, nil
}

func (f *fakeRefreshRepo) FindRefreshToken(_ context.Context, tokenHash []byte) (
	domain.SessionToken, bool, error,
) {
	if f.findEr != nil {
		return domain.SessionToken{}, false, f.findEr
	}
	token, found := f.tokens[string(tokenHash)]
	return token, found, nil
}

func (f *fakeRefreshRepo) RevokeRefreshFamily(_ context.Context, familyID uuid.UUID, _ time.Time) (int, error) {
	if f.revokeFamilyErr != nil {
		return 0, f.revokeFamilyErr
	}
	f.revokedFamilies = append(f.revokedFamilies, familyID)
	return 1, nil
}

type refreshHarness struct {
	service *service.RefreshService
	repo    *fakeRefreshRepo
	pool    *fakePool
	events  *fakeEventWriter
	clock   *clock.Fake
}

// The two refusals this service can return, named because goconst counts three
// uses as a constant waiting to be extracted and because a typo in a string
// literal here would assert nothing.
const (
	codeTokenInvalid   = "TOKEN_INVALID"
	codeSessionRevoked = "SESSION_REVOKED"
)

var refreshNow = time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)

func newRefreshServiceHarness(t *testing.T, configure func(*service.RefreshDeps)) *refreshHarness {
	t.Helper()

	fakeClock := clock.NewFake(refreshNow)
	repo := newFakeRefreshRepo()
	pool := &fakePool{}
	events := newFakeEventWriter()

	keys, err := domain.NewKeyring([]byte("refresh-service-test-hmac-key-32b---"))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	tokens, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{
			SigningKey: []byte("refresh-test-signing-key-at-least-32b"),
			Issuer:     claimIssuer,
			Audience:   claimAudience,
		},
		Clock: fakeClock,
		NewID: func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	})
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}

	deps := service.RefreshDeps{
		Pool:   pool,
		Repo:   repo,
		Tokens: tokens,
		Events: events,
		Keys:   keys,
		Clock:  fakeClock,
		NewID:  func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	}
	if configure != nil {
		configure(&deps)
	}

	return &refreshHarness{
		service: service.NewRefreshService(deps),
		repo:    repo,
		pool:    pool,
		events:  events,
		clock:   fakeClock,
	}
}

func TestNewRefreshService_FallsBackToTheDocumentedIdleWindow(t *testing.T) {
	h := newRefreshServiceHarness(t, func(d *service.RefreshDeps) { d.TTL = 0 })

	signedIn, err := h.service.Start(context.Background(), service.StartInput{UserID: uuid.New()})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Zero means the default rather than "expires immediately", which is the
	// direction a missing config value must not fail in: a token that expires
	// on issue signs every learner out on their next request.
	want := refreshNow.Add(service.DefaultRefreshTTL)
	if !signedIn.RefreshExpiresAt.Equal(want) {
		t.Errorf("expiry = %s, want %s", signedIn.RefreshExpiresAt, want)
	}
}

// TestStart_DigestsTheClientAddressAndNeverStoresIt is the privacy property of
// the session row, asserted on the value handed to the repository rather than
// on the column — this is the last point at which the address still exists in
// the clear, so it is where storing it by accident would happen.
func TestStart_DigestsTheClientAddressAndNeverStoresIt(t *testing.T) {
	h := newRefreshServiceHarness(t, nil)

	const address = "203.0.113.7"
	const agent = "a-browser"

	if _, err := h.service.Start(context.Background(), service.StartInput{
		UserID: uuid.New(), ClientIP: address, UserAgent: agent,
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(h.repo.sessions) != 1 {
		t.Fatalf("%d sessions created, want 1", len(h.repo.sessions))
	}
	if h.repo.createdTokens != 1 {
		t.Errorf("%d refresh tokens created, want 1", h.repo.createdTokens)
	}

	for name, pair := range map[string]struct {
		stored []byte
		plain  string
	}{
		"ip_hash":         {h.repo.lastIPHash, address},
		"user_agent_hash": {h.repo.lastUserAgentHash, agent},
	} {
		if len(pair.stored) != 32 {
			t.Errorf("%s is %d bytes, want the 32 the CHECK constraint requires", name, len(pair.stored))
		}
		if strings.Contains(string(pair.stored), pair.plain) {
			t.Errorf("%s contains the value it is supposed to hide", name)
		}
	}

	// An absent address is a null column, not a digest of the empty string —
	// which would be one shared value that every such session collides on and
	// that P2.6 would then present as "the same origin".
	bare := newRefreshServiceHarness(t, nil)
	if _, err := bare.service.Start(context.Background(), service.StartInput{UserID: uuid.New()}); err != nil {
		t.Fatalf("Start without an address: %v", err)
	}
	if bare.repo.lastIPHash != nil || bare.repo.lastUserAgentHash != nil {
		t.Error("an unknown address was digested into a value every anonymous session would share")
	}
}

// TestRotate_FailsWhenTheSessionCannotBeTouched keeps the rotation atomic. The
// claim has already spent the old token by then, so committing the new one
// while the session's last_seen_at silently stayed behind would leave the row
// looking idle for a learner who is demonstrably not.
func TestRotate_FailsWhenTheSessionCannotBeTouched(t *testing.T) {
	h := newRefreshServiceHarness(t, nil)
	h.repo.touchErr = errors.New("connection reset")

	presented, digest, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	h.repo.claim = func() (domain.SessionToken, bool, error) {
		return domain.SessionToken{
			RefreshToken: domain.RefreshToken{
				TokenHash: digest, FamilyID: uuid.New(), SessionID: uuid.New(),
				ExpiresAt: refreshNow.Add(time.Hour),
			},
			UserID: uuid.New(),
		}, true, nil
	}

	if _, err := h.service.Rotate(context.Background(), presented); err == nil {
		t.Fatal("the rotation reported success despite the session write failing")
	}
	if h.pool.rollbacks != 1 {
		t.Errorf("%d rollbacks, want the transaction rolled back exactly once", h.pool.rollbacks)
	}
}

// TestStart_FailsRatherThanSignInWithoutARefreshToken pins the direction. A
// session row with no token, or a token the caller never received, would leave
// the learner signed in for fifteen minutes and then permanently out — the
// failure would surface as an expiry, long after the cause.
func TestStart_FailsRatherThanSignInWithoutARefreshToken(t *testing.T) {
	cases := map[string]func(*refreshHarness){
		"the session row cannot be written": func(h *refreshHarness) {
			h.repo.createSessionErr = errors.New("connection reset")
		},
		"the token row cannot be written": func(h *refreshHarness) {
			h.repo.createTokenErr = errors.New("unique violation")
		},
		"the transaction cannot be opened": func(h *refreshHarness) {
			h.pool.beginErr = errors.New("pool exhausted")
		},
		"the transaction cannot be committed": func(h *refreshHarness) {
			h.pool.commitErr = errors.New("connection closed")
		},
	}
	for name, brk := range cases {
		t.Run(name, func(t *testing.T) {
			h := newRefreshServiceHarness(t, nil)
			brk(h)

			signedIn, err := h.service.Start(context.Background(), service.StartInput{UserID: uuid.New()})
			if err == nil {
				t.Fatal("a session was opened despite the write failing")
			}
			if signedIn.RefreshToken.Reveal() != "" {
				t.Error("a refresh token was handed out by a failed sign-in")
			}
		})
	}
}

// TestStart_FailsWhenTheEntropySourceCannotFillTheToken is the branch that
// matters most and can only be reached from here: crypto/rand does not fail on
// demand. A short read must not be padded into a weaker token.
func TestStart_FailsWhenTheEntropySourceCannotFillTheToken(t *testing.T) {
	h := newRefreshServiceHarness(t, func(d *service.RefreshDeps) {
		d.Entropy = strings.NewReader("far too few bytes")
	})

	if _, err := h.service.Start(context.Background(), service.StartInput{UserID: uuid.New()}); err == nil {
		t.Fatal("a session was opened with a token the entropy source could not fill")
	}
	if h.repo.createdTokens != 0 {
		t.Error("a token row was written for a token that was never drawn")
	}
}

func TestStart_FailsWhenTheIDGeneratorFails(t *testing.T) {
	h := newRefreshServiceHarness(t, func(d *service.RefreshDeps) {
		d.NewID = func(context.Context) (uuid.UUID, error) { return uuid.Nil, errors.New("no entropy") }
	})

	if _, err := h.service.Start(context.Background(), service.StartInput{UserID: uuid.New()}); err == nil {
		t.Fatal("a session was opened without an id")
	}
}

// TestRotate_RefusesAValueThatIsNotOneOfOursWithoutTouchingTheDatabase is the
// cheap guard in front of the expensive one. A scanner sending arbitrary
// cookies must not turn into one query per request.
func TestRotate_RefusesAValueThatIsNotOneOfOursWithoutTouchingTheDatabase(t *testing.T) {
	h := newRefreshServiceHarness(t, nil)
	h.repo.claim = func() (domain.SessionToken, bool, error) {
		t.Error("the database was consulted for a value that is not the right shape")
		return domain.SessionToken{}, false, nil
	}

	for _, value := range []string{"", "not-base64!!", "aGVsbG8"} {
		_, err := h.service.Rotate(context.Background(), value)
		assertAuthCode(t, err, codeTokenInvalid)
	}
}

// TestRotate_TellsTheFourReasonsAClaimCanMissApart is the decision table in
// `refuse`, and each row has a different consequence: only one of them burns a
// family, and only one of them files a report.
func TestRotate_TellsTheFourReasonsAClaimCanMissApart(t *testing.T) {
	spent := refreshNow.Add(-time.Minute)

	cases := map[string]struct {
		stored     *domain.SessionToken
		wantCode   string
		wantRevoke bool
		wantEvents int
	}{
		"no such token": {
			stored: nil, wantCode: codeTokenInvalid, wantRevoke: false, wantEvents: 0,
		},
		"already spent": {
			stored: &domain.SessionToken{
				RefreshToken: domain.RefreshToken{
					FamilyID: uuid.New(), SessionID: uuid.New(),
					ExpiresAt: refreshNow.Add(time.Hour), UsedAt: &spent,
				},
				UserID: uuid.New(),
			},
			wantCode: codeSessionRevoked, wantRevoke: true, wantEvents: 1,
		},
		"already revoked": {
			stored: &domain.SessionToken{
				RefreshToken: domain.RefreshToken{
					FamilyID: uuid.New(), SessionID: uuid.New(),
					ExpiresAt: refreshNow.Add(time.Hour), RevokedAt: &spent,
				},
			},
			wantCode: codeSessionRevoked, wantRevoke: false, wantEvents: 0,
		},
		"expired but never used": {
			stored: &domain.SessionToken{
				RefreshToken: domain.RefreshToken{
					FamilyID: uuid.New(), SessionID: uuid.New(),
					ExpiresAt: refreshNow.Add(-time.Second),
				},
			},
			wantCode: codeTokenInvalid, wantRevoke: false, wantEvents: 0,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			h := newRefreshServiceHarness(t, nil)

			presented, digest, err := domain.NewRefreshToken(nil)
			if err != nil {
				t.Fatalf("NewRefreshToken: %v", err)
			}
			if testCase.stored != nil {
				stored := *testCase.stored
				stored.TokenHash = digest
				h.repo.tokens[string(digest)] = stored
			}

			_, err = h.service.Rotate(context.Background(), presented)
			assertAuthCode(t, err, testCase.wantCode)

			if revoked := len(h.repo.revokedFamilies) > 0; revoked != testCase.wantRevoke {
				t.Errorf("family revoked = %v, want %v", revoked, testCase.wantRevoke)
			}
			if got := len(h.events.events); got != testCase.wantEvents {
				t.Errorf("%d security events raised, want %d", got, testCase.wantEvents)
			}
		})
	}
}

// TestRotate_ReuseRevokesTheSessionAsWellAsTheFamily is the part a family
// revocation alone would miss: the access token names the session, and P2.6's
// logout and session list both key on it. A burnt family with a live session is
// a session the learner can still see and an admin can still be told is active.
func TestRotate_ReuseRevokesTheSessionAsWellAsTheFamily(t *testing.T) {
	h := newRefreshServiceHarness(t, nil)
	spent := refreshNow.Add(-time.Minute)
	sessionID, familyID, userID := uuid.New(), uuid.New(), uuid.New()

	presented, digest, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	h.repo.tokens[string(digest)] = domain.SessionToken{
		RefreshToken: domain.RefreshToken{
			TokenHash: digest, FamilyID: familyID, SessionID: sessionID,
			ExpiresAt: refreshNow.Add(time.Hour), UsedAt: &spent,
		},
		UserID: userID,
	}

	_, err = h.service.Rotate(context.Background(), presented)
	assertAuthCode(t, err, codeSessionRevoked)

	if len(h.repo.revokedFamilies) != 1 || h.repo.revokedFamilies[0] != familyID {
		t.Errorf("revoked families = %v, want [%s]", h.repo.revokedFamilies, familyID)
	}
	if len(h.repo.revokedSessions) != 1 || h.repo.revokedSessions[0] != sessionID {
		t.Errorf("revoked sessions = %v, want [%s]", h.repo.revokedSessions, sessionID)
	}
	if len(h.events.events) != 1 {
		t.Fatalf("%d security events raised, want 1", len(h.events.events))
	}
}

// TestRotate_ReportsAFailedRevocationRatherThanRefusingQuietly is the direction
// this one has to fail in. Returning SESSION_REVOKED when the revocation did
// not happen would tell the client the family was burnt while the stolen token
// still worked — a refusal that reads like a defence and is not one.
func TestRotate_ReportsAFailedRevocationRatherThanRefusingQuietly(t *testing.T) {
	spent := refreshNow.Add(-time.Minute)

	for name, brk := range map[string]func(*refreshHarness){
		"the family cannot be revoked":  func(h *refreshHarness) { h.repo.revokeFamilyErr = errors.New("deadlock") },
		"the session cannot be revoked": func(h *refreshHarness) { h.repo.revokeSessionErr = errors.New("deadlock") },
		"the event cannot be written":   func(h *refreshHarness) { h.events.err = errors.New("outbox full") },
	} {
		t.Run(name, func(t *testing.T) {
			h := newRefreshServiceHarness(t, nil)
			brk(h)

			presented, digest, err := domain.NewRefreshToken(nil)
			if err != nil {
				t.Fatalf("NewRefreshToken: %v", err)
			}
			h.repo.tokens[string(digest)] = domain.SessionToken{
				RefreshToken: domain.RefreshToken{
					TokenHash: digest, FamilyID: uuid.New(), SessionID: uuid.New(),
					ExpiresAt: refreshNow.Add(time.Hour), UsedAt: &spent,
				},
			}

			_, err = h.service.Rotate(context.Background(), presented)
			if err == nil {
				t.Fatal("a failed revocation was reported as a successful refusal")
			}
			var appErr *apperr.Error
			if errors.As(err, &appErr) && appErr.Code == codeSessionRevoked {
				t.Error("the client was told the session was revoked when the revocation failed")
			}
		})
	}
}

// TestRotate_AnUnreadableRowIsAnErrorNotARefusal keeps a database outage from
// being reported as a bad credential, which would send every signed-in learner
// to the login form during an incident they could otherwise have ridden out.
func TestRotate_AnUnreadableRowIsAnErrorNotARefusal(t *testing.T) {
	h := newRefreshServiceHarness(t, nil)
	h.repo.findEr = errors.New("connection refused")

	presented, _, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}

	_, err = h.service.Rotate(context.Background(), presented)
	if err == nil {
		t.Fatal("an unreadable row was reported as a successful rotation")
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Code == codeTokenInvalid {
		t.Error("a database outage was reported to the client as an invalid token")
	}
}

// TestRotate_ASuccessfulClaimRotatesAndTouchesTheSession is the happy path at
// this level: the integration suite proves the claim is atomic, and this proves
// what the service does once it has won one.
func TestRotate_ASuccessfulClaimRotatesAndTouchesTheSession(t *testing.T) {
	h := newRefreshServiceHarness(t, nil)
	sessionID, familyID, userID := uuid.New(), uuid.New(), uuid.New()

	presented, digest, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	h.repo.claim = func() (domain.SessionToken, bool, error) {
		return domain.SessionToken{
			RefreshToken: domain.RefreshToken{
				TokenHash: digest, FamilyID: familyID, SessionID: sessionID,
				ExpiresAt: refreshNow.Add(time.Hour),
			},
			UserID: userID,
		}, true, nil
	}

	signedIn, err := h.service.Rotate(context.Background(), presented)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if signedIn.RefreshToken.Reveal() == presented {
		t.Error("rotation handed back the token it was given")
	}
	if signedIn.Session.SessionID != sessionID {
		t.Errorf("session id = %s, want the claimed token's %s", signedIn.Session.SessionID, sessionID)
	}
	// The access token must name the account, not the session. Passing the
	// wrong one compiles cleanly — both are uuid.UUID — and produces a token
	// whose `sub` is a session id that no permission check will ever match.
	if signedIn.Session.UserID != userID {
		t.Errorf("access token subject = %s, want the account %s", signedIn.Session.UserID, userID)
	}
	if h.repo.touches != 1 {
		t.Errorf("%d session touches, want 1", h.repo.touches)
	}
	if len(h.repo.revokedFamilies) != 0 {
		t.Error("an ordinary rotation revoked a family")
	}
}
