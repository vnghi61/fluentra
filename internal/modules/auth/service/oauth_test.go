package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

const (
	googleSubject = "104219308972340982374"
	googleEmail   = "learner@example.com"

	// authCode is what Google hands back on the redirect. Its value never
	// matters here — the provider is a fake — but it has to be non-empty,
	// because an empty one would exercise a path the real exchange refuses.
	authCode = "an-auth-code"
)

// fakeOAuthRepo is an in-memory stand-in for `core.oauth_states` and
// `core.oauth_identities`.
//
// Every guard mirrors a clause in `db/queries/auth/oauth.sql`, and the comments
// say which. A fake more permissive than the SQL would pass tests the real
// system fails — the integration suite runs the same properties against Postgres
// and is what catches this drifting from the query it imitates.
type fakeOAuthRepo struct {
	mu         sync.Mutex
	states     map[string]domain.OAuthState
	identities map[uuid.UUID]domain.OAuthIdentity

	// passwords is which accounts hold a credential, so CountSignInMethods can
	// answer the way the SQL's two sub-selects do.
	passwords map[uuid.UUID]bool
}

func newFakeOAuthRepo() *fakeOAuthRepo {
	return &fakeOAuthRepo{
		states:     make(map[string]domain.OAuthState),
		identities: make(map[uuid.UUID]domain.OAuthIdentity),
		passwords:  make(map[uuid.UUID]bool),
	}
}

func (f *fakeOAuthRepo) CreateOAuthState(
	_ context.Context, id uuid.UUID, state, provider, nonce string,
	verifierHash []byte, redirectTo *string, now, expiresAt time.Time,
) (domain.OAuthState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row := domain.OAuthState{
		ID: id, State: state, Provider: provider, Nonce: nonce,
		PKCEVerifierHash: verifierHash, RedirectTo: redirectTo,
		CreatedAt: now, ExpiresAt: expiresAt,
	}
	f.states[state] = row
	return row, nil
}

// ConsumeOAuthState mirrors:
//
//	WHERE state = $1 AND provider = $2 AND consumed_at IS NULL AND expires_at > @now
func (f *fakeOAuthRepo) ConsumeOAuthState(_ context.Context, state, provider string, now time.Time) (
	domain.OAuthState, bool, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.states[state]
	if !ok || row.Provider != provider || row.ConsumedAt != nil || !row.ExpiresAt.After(now) {
		return domain.OAuthState{}, false, nil
	}
	consumedAt := now
	row.ConsumedAt = &consumedAt
	f.states[state] = row
	return row, true, nil
}

func (f *fakeOAuthRepo) CreateOAuthIdentity(
	_ context.Context, id, userID uuid.UUID, provider, subject string, emailHash []byte, now time.Time,
) (domain.OAuthIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// uq_oauth_identities_subject and uq_oauth_identities_user_provider. Both
	// surface as the same refusal, exactly as the repository maps them.
	for _, existing := range f.identities {
		if existing.Provider == provider && (existing.Subject == subject || existing.UserID == userID) {
			return domain.OAuthIdentity{}, domain.ErrOAuthAlreadyLinked
		}
	}

	row := domain.OAuthIdentity{
		ID: id, UserID: userID, Provider: provider,
		Subject: subject, EmailHash: emailHash, LinkedAt: now,
	}
	f.identities[id] = row
	return row, nil
}

func (f *fakeOAuthRepo) FindOAuthIdentityBySubject(_ context.Context, provider, subject string) (
	domain.OAuthIdentity, bool, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, row := range f.identities {
		if row.Provider == provider && row.Subject == subject {
			return row, true, nil
		}
	}
	return domain.OAuthIdentity{}, false, nil
}

func (f *fakeOAuthRepo) FindOAuthIdentityByUser(_ context.Context, userID uuid.UUID, provider string) (
	domain.OAuthIdentity, bool, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, row := range f.identities {
		if row.Provider == provider && row.UserID == userID {
			return row, true, nil
		}
	}
	return domain.OAuthIdentity{}, false, nil
}

func (f *fakeOAuthRepo) DeleteOAuthIdentity(_ context.Context, userID uuid.UUID, provider string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for id, row := range f.identities {
		if row.Provider == provider && row.UserID == userID {
			delete(f.identities, id)
			return true, nil
		}
	}
	return false, nil
}

// CountSignInMethods mirrors the two sub-selects: one row per credential plus
// one per linked identity.
func (f *fakeOAuthRepo) CountSignInMethods(_ context.Context, userID uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	methods := 0
	if f.passwords[userID] {
		methods++
	}
	for _, row := range f.identities {
		if row.UserID == userID {
			methods++
		}
	}
	return methods, nil
}

func (f *fakeOAuthRepo) DeleteExpiredOAuthStates(_ context.Context, cutoff time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	removed := 0
	for state, row := range f.states {
		if row.ExpiresAt.Before(cutoff) {
			delete(f.states, state)
			removed++
		}
	}
	return removed, nil
}

// WithTx returns the same store. What a real transaction buys — the unlink count
// and the delete being one decision — needs Postgres, and the integration suite
// is where that is proven.
func (f *fakeOAuthRepo) WithTx(pgx.Tx) service.OAuthRepo { return f }

func (f *fakeOAuthRepo) identityCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.identities)
}

// fakeProvider stands in for Google, and records what it was handed.
//
// The recording is the point for two of the tests: the verifier that reaches the
// token endpoint and the nonce that reaches verification are the two values the
// flow's security rests on, and neither is observable from the outside.
type fakeProvider struct {
	mu sync.Mutex

	// identity is what a successful verification asserts.
	identity google.Identity

	// exchangeErr and verifyErr fail the respective step when set.
	exchangeErr error
	verifyErr   error

	issuedState     string
	issuedNonce     string
	issuedChallenge string

	seenVerifier string
	seenNonce    string
	exchanges    int
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{
		identity: google.Identity{
			Subject: googleSubject, Email: googleEmail, EmailVerified: true, Name: "A Learner",
		},
	}
}

func (f *fakeProvider) AuthorizationURL(state, nonce, challenge string) string {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.issuedState, f.issuedNonce, f.issuedChallenge = state, nonce, challenge
	return "https://accounts.google.test/authorize?state=" + state
}

func (f *fakeProvider) Exchange(_ context.Context, _, verifier string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.exchanges++
	f.seenVerifier = verifier
	if f.exchangeErr != nil {
		return "", f.exchangeErr
	}
	return "an.id.token", nil
}

func (f *fakeProvider) Verify(_ context.Context, _, nonce string) (google.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.seenNonce = nonce
	if f.verifyErr != nil {
		return google.Identity{}, f.verifyErr
	}
	return f.identity, nil
}

func (f *fakeProvider) state() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issuedState
}

type oauthHarness struct {
	service  *service.OAuthService
	repo     *fakeOAuthRepo
	provider *fakeProvider
	accounts *fakeAccounts
	events   *fakeEventWriter
	clock    *clock.Fake
	keys     domain.Keyring
}

func newOAuthHarness(t *testing.T) *oauthHarness {
	t.Helper()

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	fakeClock := clock.NewFake(testNow)
	newID := func(context.Context) (uuid.UUID, error) { return uuid.New(), nil }

	// A real token service, for the reason the registration harness gives: a
	// stub could not notice a path that hands back a session nobody can present.
	tokens, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{
			SigningKey: []byte("oauth-test-signing-key-32-bytes-minimum"),
			Issuer:     claimIssuer,
			Audience:   claimAudience,
		},
		Clock: fakeClock,
		NewID: newID,
	})
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}

	harness := &oauthHarness{
		repo:     newFakeOAuthRepo(),
		provider: newFakeProvider(),
		accounts: newFakeAccounts(),
		events:   newFakeEventWriter(),
		clock:    fakeClock,
		keys:     keys,
	}

	harness.service = service.NewOAuthService(service.OAuthDeps{
		Pool:     &fakePool{},
		Repo:     harness.repo,
		Provider: harness.provider,
		Accounts: harness.accounts,
		Sessions: fakeSessions{tokens: tokens, clock: fakeClock, newID: newID},
		Events:   harness.events,
		Keys:     keys,
		Clock:    fakeClock,
		NewID:    newID,
	})
	return harness
}

// begin runs a real Start and returns the state the browser would come back
// with. Tests drive the callback through the same values production would.
func (h *oauthHarness) begin(t *testing.T) string {
	t.Helper()

	if _, err := h.service.Start(context.Background(), ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return h.provider.state()
}

// callback completes a flow that begin started.
func (h *oauthHarness) callback(t *testing.T, state string) (service.SignedIn, error) {
	t.Helper()
	return h.service.Callback(context.Background(), service.CallbackInput{Code: authCode, State: state})
}

// account adds a local account, and says whether it has proved its address.
func (h *oauthHarness) account(t *testing.T, email string, verified bool) uuid.UUID {
	t.Helper()

	userID, err := h.accounts.CreateAccount(context.Background(), service.NewAccount{Email: email})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if verified {
		if err := h.accounts.MarkEmailVerified(context.Background(), userID); err != nil {
			t.Fatalf("MarkEmailVerified: %v", err)
		}
	}
	// Every account made this way registered with a password, which is what
	// makes it a second sign-in method for the unlink tests.
	h.repo.passwords[userID] = true
	return userID
}

// securityEvents returns the security events raised, by kind.
func (h *oauthHarness) securityEvents(t *testing.T) []contract.SecurityEvent {
	t.Helper()

	h.events.mu.Lock()
	defer h.events.mu.Unlock()

	raised := make([]contract.SecurityEvent, 0, len(h.events.events))
	for _, recorded := range h.events.events {
		if recorded.Event != contract.EventSecurityEvent {
			continue
		}
		event, ok := recorded.Payload.(contract.SecurityEvent)
		if !ok {
			t.Fatalf("a security event carried %T rather than contract.SecurityEvent", recorded.Payload)
		}
		raised = append(raised, event)
	}
	return raised
}

// ---------------------------------------------------------------------------
// The five linking branches, against real rows.
// ---------------------------------------------------------------------------

// TestOAuthCallback_AnUnverifiedLocalMatchIsRefusedAndWritesNoIdentity is the
// account-takeover path, and it asserts the part that a refusal alone would not:
// that nothing was written.
//
// A 409 with an identity row behind it is the same compromise as a 200. The
// attacker's next request — a link, a retry, anything that finds the row — signs
// them into the victim's account. So the assertion here is on the table, not on
// the error.
func TestOAuthCallback_AnUnverifiedLocalMatchIsRefusedAndWritesNoIdentity(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	harness.account(t, googleEmail, false)

	_, err := harness.callback(t, harness.begin(t))

	if !errors.Is(err, domain.ErrOAuthAccountConflict) {
		t.Fatalf("error = %v, want OAUTH_ACCOUNT_CONFLICT", err)
	}
	if !apperr.Is(err, apperr.Conflict) {
		t.Errorf("error kind is not a conflict, so the handler would not answer 409")
	}
	if count := harness.repo.identityCount(); count != 0 {
		t.Fatalf("%d identity rows were written by a refused sign-in; the refusal is worth "+
			"nothing if the link it refused exists afterwards", count)
	}
}

func TestOAuthCallback_AKnownIdentitySignsInWithoutConsultingTheAddress(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	userID := harness.account(t, googleEmail, true)

	// Already linked, from an earlier sign-in.
	if _, err := harness.repo.CreateOAuthIdentity(context.Background(), uuid.New(), userID,
		domain.ProviderGoogle, googleSubject, harness.keys.SubjectHash(googleEmail), testNow); err != nil {
		t.Fatalf("CreateOAuthIdentity: %v", err)
	}

	// Google now asserts a different address for the same subject — a learner
	// who changed it there. The identity is the subject, so this still signs in
	// to the same account, and no second row appears.
	harness.provider.identity.Email = "moved@example.com"

	signedIn, err := harness.callback(t, harness.begin(t))
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	if signedIn.Session.UserID != userID {
		t.Errorf("signed in as %s, want the account the subject is linked to (%s)",
			signedIn.Session.UserID, userID)
	}
	if count := harness.repo.identityCount(); count != 1 {
		t.Errorf("identity rows = %d, want the one that already existed", count)
	}
}

func TestOAuthCallback_AVerifiedLocalMatchIsLinkedThenSignedIn(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	userID := harness.account(t, googleEmail, true)

	signedIn, err := harness.callback(t, harness.begin(t))
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	if signedIn.Session.UserID != userID {
		t.Errorf("signed in as %s, want the matching verified account %s", signedIn.Session.UserID, userID)
	}

	linked, found, err := harness.repo.FindOAuthIdentityByUser(
		context.Background(), userID, domain.ProviderGoogle)
	if err != nil || !found {
		t.Fatalf("the identity was not linked (found=%v, err=%v)", found, err)
	}
	if linked.Subject != googleSubject {
		t.Errorf("linked subject = %q, want Google's %q", linked.Subject, googleSubject)
	}
	// The address is stored keyed, never in the clear.
	if strings.Contains(string(linked.EmailHash), googleEmail) {
		t.Error("the stored email hash contains the address")
	}
}

// TestOAuthCallback_NoLocalAccountCreatesOneAlreadyVerified pins BR-AUTH-19.
//
// The account is verified on creation because Google has performed exactly the
// check the OTP would have. Leaving it unverified would send a code to an address
// the provider just proved, and — worse — would make the learner's *next* Google
// sign-in hit the conflict branch and be refused.
func TestOAuthCallback_NoLocalAccountCreatesOneAlreadyVerified(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)

	signedIn, err := harness.callback(t, harness.begin(t))
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}

	created, found, err := harness.accounts.FindByEmail(context.Background(), googleEmail)
	if err != nil || !found {
		t.Fatalf("no account was created (found=%v, err=%v)", found, err)
	}
	if !created.Verified {
		t.Error("the account was created unverified, so the learner's next Google sign-in " +
			"would match an unverified local account and be refused")
	}
	if created.ID != signedIn.Session.UserID {
		t.Errorf("signed in as %s but created %s", signedIn.Session.UserID, created.ID)
	}
	if harness.repo.identityCount() != 1 {
		t.Error("the new account has no linked identity, so nothing connects it to the Google account")
	}
}

// TestOAuthCallback_AnUnverifiedGoogleEmailIsRefusedBeforeAnyMatching pins
// BR-AUTH-15 and, more importantly, the ordering.
//
// The local account here is verified and would otherwise be linked to. It is not
// consulted at all, because an address a provider will not vouch for is worth no
// more than one typed into a form — and matching on it would let anyone who can
// make Google emit an unverified claim walk into the link-to-verified branch.
func TestOAuthCallback_AnUnverifiedGoogleEmailIsRefusedBeforeAnyMatching(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	harness.account(t, googleEmail, true)
	harness.provider.identity.EmailVerified = false

	_, err := harness.callback(t, harness.begin(t))

	if !errors.Is(err, domain.ErrOAuthEmailUnverified) {
		t.Fatalf("error = %v, want OAUTH_EMAIL_UNVERIFIED", err)
	}
	if !apperr.Is(err, apperr.Forbidden) {
		t.Errorf("error kind is not forbidden, so the handler would not answer 403")
	}
	if count := harness.repo.identityCount(); count != 0 {
		t.Errorf("%d identity rows were written for an address Google would not vouch for", count)
	}
}

// ---------------------------------------------------------------------------
// The state: forged, reused, expired.
// ---------------------------------------------------------------------------

// TestOAuthCallback_AForgedStateIsRefusedAndRecorded covers a callback carrying
// a value this server never issued — the plain CSRF attempt.
func TestOAuthCallback_AForgedStateIsRefusedAndRecorded(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)

	_, err := harness.callback(t, "a-state-this-server-never-issued")

	assertStateRefused(t, harness, err, 0)
	if harness.provider.exchanges != 0 {
		t.Error("a forged state reached the token exchange, so a forged callback costs us " +
			"an outbound request to Google")
	}
}

// TestOAuthCallback_AReusedStateIsRefusedAndRecorded is single use, which is the
// property that makes the state worth having at all (BR-AUTH-17).
func TestOAuthCallback_AReusedStateIsRefusedAndRecorded(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	state := harness.begin(t)

	if _, err := harness.callback(t, state); err != nil {
		t.Fatalf("the first callback failed: %v", err)
	}

	_, err := harness.callback(t, state)

	// One identity exists already: the first callback signed somebody in, which
	// is what makes this a *reused* state rather than a forged one.
	assertStateRefused(t, harness, err, 1)
}

// TestOAuthCallback_AnExpiredStateIsRefusedAndRecorded closes the window on a
// consent screen left open. Ten minutes is the whole life of an authorization
// request; after it the row is no longer a credential.
func TestOAuthCallback_AnExpiredStateIsRefusedAndRecorded(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	state := harness.begin(t)

	harness.clock.Advance(service.DefaultOAuthStateTTL + time.Second)

	_, err := harness.callback(t, state)

	assertStateRefused(t, harness, err, 0)
}

// assertStateRefused is the shared shape of all three: the same refusal, not
// distinguishable from one another, and each one recorded.
//
// They report one code deliberately — forged, spent and expired are the three
// shapes a CSRF attempt takes, and telling a prober which one they hit is
// telling them how the validation works. What the three must *not* share is
// silence: a refused state leaves no other trace, so without the event a
// campaign of them is invisible.
// identitiesBefore is what the table held before the refused call, so the
// reused-state case — whose first callback legitimately signed somebody in — can
// assert the refusal wrote nothing *further* rather than that the table is bare.
func assertStateRefused(t *testing.T, harness *oauthHarness, err error, identitiesBefore int) {
	t.Helper()

	if !errors.Is(err, domain.ErrOAuthStateInvalid) {
		t.Fatalf("error = %v, want OAUTH_STATE_INVALID", err)
	}

	raised := harness.securityEvents(t)
	if len(raised) != 1 {
		t.Fatalf("%d security events were raised, want exactly one", len(raised))
	}
	if raised[0].Kind != contract.SecurityKindOAuthStateInvalid {
		t.Errorf("event kind = %q, want %q", raised[0].Kind, contract.SecurityKindOAuthStateInvalid)
	}
	if raised[0].Severity != contract.SeverityMedium {
		t.Errorf("severity = %q, want medium: one of these is ordinary and the rate is the signal",
			raised[0].Severity)
	}
	if raised[0].UserID != uuid.Nil {
		t.Error("the event names an account, but a refused state identifies nobody — " +
			"anything from the request here is attacker-chosen")
	}
	if after := harness.repo.identityCount(); after != identitiesBefore {
		t.Errorf("identity rows went from %d to %d: a refused state wrote one", identitiesBefore, after)
	}
}

// ---------------------------------------------------------------------------
// The ID token, and what a failure must not leave behind.
// ---------------------------------------------------------------------------

// TestOAuthCallback_AnIDTokenFailingVerificationCreatesNothing is BR-AUTH-18's
// second half, and it asserts the tables rather than the error.
//
// "It returned an error" is not the property. The property is that there is
// nothing to clean up: no account, no identity, no session. Everything is
// checked before anything is written, so at the moment a check fails there is
// nothing to undo.
func TestOAuthCallback_AnIDTokenFailingVerificationCreatesNothing(t *testing.T) {
	t.Parallel()

	// Each case is one of the five checks in google.Verifier.Verify. They
	// collapse into one refusal here on purpose — which check a forged token
	// tripped is information about our validation.
	for name, failure := range map[string]error{
		"a bad signature":        errors.New("google id token is not valid: signature"),
		"a foreign issuer":       errors.New("google id token is not valid: issuer"),
		"another app's audience": errors.New("google id token is not valid: audience"),
		"an expired token":       errors.New("google id token is not valid: expired"),
		"a nonce from a flow":    errors.New("google id token is not valid: nonce does not match"),
		"an alg:none token":      errors.New("google id token is not valid: alg none"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			harness := newOAuthHarness(t)
			harness.provider.verifyErr = failure

			_, err := harness.callback(t, harness.begin(t))

			if !errors.Is(err, domain.ErrOAuthTokenInvalid) {
				t.Fatalf("error = %v, want TOKEN_INVALID", err)
			}
			if count := harness.repo.identityCount(); count != 0 {
				t.Errorf("%d identity rows exist after a token that failed verification", count)
			}
			if _, found, _ := harness.accounts.FindByEmail(context.Background(), googleEmail); found {
				t.Error("an account exists after a token that failed verification — " +
					"a partial account is the state BR-AUTH-18 exists to prevent")
			}
			if harness.accounts.createCalls != 0 {
				t.Error("account creation was attempted before the ID token was verified")
			}
		})
	}
}

// TestOAuthCallback_TheNonceCheckedIsTheOneTheRowIssued closes the replay of a
// real Google token obtained through some other flow.
//
// The nonce must come from the state row, never from the request: a nonce the
// caller supplied is a nonce the caller can match, which makes the check a
// formality.
func TestOAuthCallback_TheNonceCheckedIsTheOneTheRowIssued(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	state := harness.begin(t)
	issuedNonce := harness.provider.issuedNonce

	if _, err := harness.callback(t, state); err != nil {
		t.Fatalf("Callback: %v", err)
	}

	if harness.provider.seenNonce != issuedNonce {
		t.Errorf("verified against nonce %q, want the one stored with the state (%q)",
			harness.provider.seenNonce, issuedNonce)
	}
	if issuedNonce == state {
		t.Error("the nonce equals the state; they are checked in different places by different " +
			"mechanisms and one value cannot do both jobs")
	}
}

// TestOAuthCallback_TheVerifierSentToGoogleMatchesTheChallengeIssued is the PKCE
// pair surviving the round trip.
//
// The verifier is derived from the state rather than stored, so this is the test
// that would fail if the derivation, the key or the state column ever stopped
// agreeing — and the alternative failure is an opaque `invalid_grant` from
// Google that looks like the learner's fault.
func TestOAuthCallback_TheVerifierSentToGoogleMatchesTheChallengeIssued(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	state := harness.begin(t)
	challenge := harness.provider.issuedChallenge

	if _, err := harness.callback(t, state); err != nil {
		t.Fatalf("Callback: %v", err)
	}

	sent := harness.provider.seenVerifier
	if sent == "" {
		t.Fatal("no verifier reached the token exchange, so PKCE was not in force")
	}
	if sent == challenge {
		t.Fatal("the verifier equals the challenge, which is the `plain` method")
	}
	if want := harness.keys.PKCEFor(state); sent != want.Verifier || challenge != want.Challenge {
		t.Errorf("the pair sent to Google is not the pair derived from the state")
	}
}

// TestOAuthCallback_AnUnreachableGoogleIsUnavailableRatherThanInvalid keeps our
// outage from being reported as the learner's problem.
func TestOAuthCallback_AnUnreachableGoogleIsUnavailableRatherThanInvalid(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	// The shape google.Exchange produces when it could not reach the endpoint.
	harness.provider.exchangeErr = errUnreachableProvider

	_, err := harness.callback(t, harness.begin(t))

	if !apperr.Is(err, apperr.Unavailable) {
		t.Fatalf("error = %v, want a 503: telling a learner their sign-in was invalid when "+
			"Google was unreachable sends them to fix something that is not broken", err)
	}
}

// ---------------------------------------------------------------------------
// Linking and unlinking.
// ---------------------------------------------------------------------------

func TestOAuthLink_AttachesTheIdentityToTheSignedInAccount(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	userID := harness.account(t, googleEmail, true)

	linked, err := harness.service.Link(context.Background(), httpx.Actor{UserID: userID},
		service.CallbackInput{Code: authCode, State: harness.begin(t)})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if linked.Provider != domain.ProviderGoogle || linked.Email != googleEmail {
		t.Errorf("linked = %+v, want the google identity for %s", linked, googleEmail)
	}
	if harness.repo.identityCount() != 1 {
		t.Error("no identity row was written")
	}
}

// TestOAuthLink_AGoogleAccountWithADifferentAddressIsRefused is the takeover the
// callback's conflict branch refuses, arrived at from the other side.
//
// Linking an address the account's owner cannot receive mail at would attach a
// second way in that they do not control — and the account, unlike an unverified
// one, may be full of their work.
func TestOAuthLink_AGoogleAccountWithADifferentAddressIsRefused(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	userID := harness.account(t, "owner@example.com", true)
	// Google asserts somebody else's address, verified and all.
	harness.provider.identity.Email = "somebody-else@example.com"

	_, err := harness.service.Link(context.Background(), httpx.Actor{UserID: userID},
		service.CallbackInput{Code: authCode, State: harness.begin(t)})

	if !errors.Is(err, domain.ErrOAuthEmailMismatch) {
		t.Fatalf("error = %v, want OAUTH_EMAIL_MISMATCH", err)
	}
	if harness.repo.identityCount() != 0 {
		t.Error("a refused link wrote an identity row")
	}
}

// TestOAuthLink_AnIdentityAlreadyOnAnotherAccountIsRefused keeps one Google
// account to one Fluentra account. Without it, "sign in with Google" stops
// identifying anybody in particular.
func TestOAuthLink_AnIdentityAlreadyOnAnotherAccountIsRefused(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	first := harness.account(t, googleEmail, true)
	second := harness.account(t, "second@example.com", true)

	if _, err := harness.repo.CreateOAuthIdentity(context.Background(), uuid.New(), first,
		domain.ProviderGoogle, googleSubject, harness.keys.SubjectHash(googleEmail), testNow); err != nil {
		t.Fatalf("CreateOAuthIdentity: %v", err)
	}

	_, err := harness.service.Link(context.Background(), httpx.Actor{UserID: second},
		service.CallbackInput{Code: authCode, State: harness.begin(t)})

	if !errors.Is(err, domain.ErrOAuthAlreadyLinked) {
		t.Fatalf("error = %v, want OAUTH_ALREADY_LINKED", err)
	}
}

// TestOAuthUnlink_TheOnlySignInMethodIsRefused is BR-AUTH-20.
//
// An account opened through Google has no password, so removing the identity
// would lock the learner out with no recovery path — `forgot-password` cannot
// help somebody who has no password to reset.
func TestOAuthUnlink_TheOnlySignInMethodIsRefused(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)

	// A Google-only account: created by the callback, so it has no credential.
	signedIn, err := harness.callback(t, harness.begin(t))
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	userID := signedIn.Session.UserID

	err = harness.service.Unlink(context.Background(), httpx.Actor{UserID: userID})

	if !errors.Is(err, domain.ErrLastSignInMethod) {
		t.Fatalf("error = %v, want LAST_SIGN_IN_METHOD", err)
	}
	if !apperr.Is(err, apperr.Conflict) {
		t.Errorf("error kind is not a conflict, so the handler would not answer 409")
	}
	if _, found, _ := harness.repo.FindOAuthIdentityByUser(
		context.Background(), userID, domain.ProviderGoogle); !found {
		t.Error("the identity was removed despite the refusal, which is the lockout itself")
	}
}

func TestOAuthUnlink_RemovesTheLinkWhenAPasswordRemains(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	userID := harness.account(t, googleEmail, true)

	if _, err := harness.callback(t, harness.begin(t)); err != nil {
		t.Fatalf("Callback: %v", err)
	}

	if err := harness.service.Unlink(context.Background(), httpx.Actor{UserID: userID}); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if _, found, _ := harness.repo.FindOAuthIdentityByUser(
		context.Background(), userID, domain.ProviderGoogle); found {
		t.Error("the identity survived an unlink")
	}
}

// TestOAuthUnlink_UnlinkingNothingSucceeds keeps the operation idempotent: the
// caller's goal already holds, and answering 204 is what makes a retry safe.
func TestOAuthUnlink_UnlinkingNothingSucceeds(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)
	userID := harness.account(t, googleEmail, true)

	if err := harness.service.Unlink(context.Background(), httpx.Actor{UserID: userID}); err != nil {
		t.Fatalf("Unlink with nothing linked = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Start.
// ---------------------------------------------------------------------------

// TestOAuthStart_RevealsNothingTheCallbackWillCheck is why the response carries
// one URL and nothing else.
//
// A `state` the page can read is a `state` an attacker who can read the same
// page can replay, and a verifier the page can read defeats PKCE outright. The
// three values exist to be known only here.
func TestOAuthStart_RevealsNothingTheCallbackWillCheck(t *testing.T) {
	t.Parallel()

	harness := newOAuthHarness(t)

	started, err := harness.service.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	state := harness.provider.state()
	stored, found := harness.repo.states[state]
	if !found {
		t.Fatal("Start returned a URL for a state it never stored, so no callback could complete")
	}
	if strings.Contains(started.AuthorizationURL, harness.keys.PKCEFor(state).Verifier) {
		t.Error("the authorization URL carries the PKCE verifier, which is the `plain` method " +
			"and defeats the mechanism entirely")
	}
	if len(stored.PKCEVerifierHash) != 32 {
		t.Errorf("stored verifier digest is %d bytes, want the 32 the CHECK constraint requires",
			len(stored.PKCEVerifierHash))
	}
	if !stored.ExpiresAt.Equal(testNow.Add(service.DefaultOAuthStateTTL)) {
		t.Errorf("state expires at %s, want ten minutes after issue", stored.ExpiresAt)
	}
}

// TestOAuthStart_KeepsAnOpenRedirectOutOfTheRow.
//
// Storing the destination server-side stops it being rewritten in flight, which
// is what the schema comment says the column is for — but it does not make the
// caller's own value safe, because the caller is who started the flow. Only a
// same-site path survives.
func TestOAuthStart_KeepsAnOpenRedirectOutOfTheRow(t *testing.T) {
	t.Parallel()

	for name, redirect := range map[string]string{
		"an absolute url":     "https://evil.test/harvest",
		"a protocol-relative": "//evil.test/harvest",
		"a bare host":         "evil.test",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			harness := newOAuthHarness(t)
			if _, err := harness.service.Start(context.Background(), redirect); err != nil {
				t.Fatalf("Start: %v", err)
			}

			stored := harness.repo.states[harness.provider.state()]
			if stored.RedirectTo != nil {
				t.Errorf("stored redirect_to = %q, want it dropped", *stored.RedirectTo)
			}
		})
	}

	harness := newOAuthHarness(t)
	if _, err := harness.service.Start(context.Background(), "/lessons/3"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	stored := harness.repo.states[harness.provider.state()]
	if stored.RedirectTo == nil || *stored.RedirectTo != "/lessons/3" {
		t.Error("a same-site path was dropped, so the learner cannot be returned where they were")
	}
}

// errUnreachableProvider is shaped exactly as google.Exchange shapes a failure
// to reach the token endpoint: the sentinel, wrapped with the detail. The
// service classifies it through google.ErrUnavailable, so a fake that merely
// looked similar would be classified as a rejection and the test would pass for
// the wrong reason.
var errUnreachableProvider = fmt.Errorf("%w: dial tcp: connection refused", google.ErrProviderUnavailable)
