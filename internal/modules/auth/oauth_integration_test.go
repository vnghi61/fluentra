//go:build integration

// Google sign-in against a real PostgreSQL.
//
// The five linking branches are decided by rows: whether an identity exists for
// a subject, whether an address resolves to an account, and whether that account
// has ever proved it. A fake repository would answer whatever its author
// expected. The branch that matters most — a Google email matching an
// *unverified* local account — is an account-takeover path, and what has to be
// true about it is not "the call returned an error" but "the table is still
// empty", which is a question only the database can answer.
package auth_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/modules/auth/service/oauth/google"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

const (
	// oauthSubject is Google's `sub` for the identity under test. It is stable
	// for the life of a Google account and is what the identity is keyed on —
	// never the address.
	oauthSubject = "104219308972340982374"

	// oauthCode is what Google hands back on the redirect. Its value never
	// matters — the provider is a stub — but it has to be non-empty, because an
	// empty one exercises a path the real exchange refuses before we get here.
	oauthCode = "an-auth-code"
)

// oauthAdapter narrows the repository to service.OAuthRepo, as module.go does.
type oauthAdapter struct {
	*repository.Repository
}

func (a oauthAdapter) WithTx(tx pgx.Tx) service.OAuthRepo {
	return oauthAdapter{Repository: a.Repository.WithTx(tx)}
}

// oauthAccounts is the full service.Accounts over `core.users` in plain SQL.
//
// It is wider than the password suite's sqlAccounts because this card is the
// first that creates an account and marks it verified, and it is written out
// rather than borrowed from the `user` module for the reason seedUser is:
// importing that module's repository from here is the boundary crossing rule L1
// forbids, test or not.
type oauthAccounts struct{ t *testing.T }

func (a oauthAccounts) CreateAccount(ctx context.Context, account service.NewAccount) (uuid.UUID, error) {
	userID, err := id.NewUUIDv7(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		userID, account.Email); err != nil {
		return uuid.Nil, err
	}
	// context.WithoutCancel and not ctx: the cleanup runs after the test that
	// created this account has finished, so ctx is cancelled by then and the
	// delete would be a no-op that silently leaks the row into the next run.
	cleanupCtx := context.WithoutCancel(ctx)
	a.t.Cleanup(func() {
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM core.users WHERE id = $1`, userID)
	})
	return userID, nil
}

func (a oauthAccounts) FindByEmail(ctx context.Context, email string) (service.Account, bool, error) {
	var account service.Account
	err := pool.QueryRow(ctx, `
		SELECT id, email_verified_at IS NOT NULL, status::text
		FROM core.users WHERE email = $1`, email).Scan(&account.ID, &account.Verified, &account.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.Account{}, false, nil
	}
	if err != nil {
		return service.Account{}, false, err
	}
	return account, true, nil
}

func (a oauthAccounts) Recipient(ctx context.Context, userID uuid.UUID) (service.Contact, error) {
	var contact service.Contact
	err := pool.QueryRow(ctx, `SELECT email::text FROM core.users WHERE id = $1`, userID).Scan(&contact.Email)
	return contact, err
}

func (a oauthAccounts) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`UPDATE core.users SET email_verified_at = now() WHERE id = $1 AND email_verified_at IS NULL`, userID)
	return err
}

func (a oauthAccounts) PurgeUnverifiedBefore(context.Context, time.Time) (int, error) { return 0, nil }

// stubProvider stands in for Google. Everything it does is a decision Google
// makes for us in production — what the subject is, what the address is, and
// whether Google will vouch for it — so it is scripted rather than faked.
type stubProvider struct {
	mu sync.Mutex

	identity    google.Identity
	verifyErr   error
	exchangeErr error

	state string
	nonce string
}

func newStubProvider(email string) *stubProvider {
	return &stubProvider{
		identity: google.Identity{
			Subject: oauthSubject, Email: email, EmailVerified: true, Name: "A Learner",
		},
	}
}

func (s *stubProvider) AuthorizationURL(state, nonce, challenge string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state, s.nonce = state, nonce
	return "https://accounts.google.test/authorize?state=" + state + "&code_challenge=" + challenge
}

func (s *stubProvider) Exchange(context.Context, string, string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.exchangeErr != nil {
		return "", s.exchangeErr
	}
	return "an.id.token", nil
}

func (s *stubProvider) Verify(_ context.Context, _, nonce string) (google.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.verifyErr != nil {
		return google.Identity{}, s.verifyErr
	}
	// The nonce is checked here in production. Asserting it reached us at all is
	// what stops a callback completing with a nonce nobody issued.
	if nonce == "" {
		return google.Identity{}, errors.New("google id token is not valid: no nonce was supplied")
	}
	return s.identity, nil
}

func (s *stubProvider) issuedState() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

type oauthHarness struct {
	service  *service.OAuthService
	provider *stubProvider
	clock    *clock.Fake
	keys     domain.Keyring
}

func newOAuthHarness(t *testing.T, providerEmail string) *oauthHarness {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	fake := clock.NewFake(harnessNow)

	tokens, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{
			SigningKey: []byte(signingKey),
			Issuer:     "fluentra-test",
			Audience:   "fluentra-api-test",
			AccessTTL:  service.DefaultAccessTTL,
		},
		Clock: fake,
		NewID: id.NewUUIDv7,
	})
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	repo := repository.New(pool)
	events := eventWriter{Writer: outbox.NewWriter()}

	// The real refresh service, so a Google sign-in roots a real family and the
	// session it produces is one the rest of WP2 can act on.
	sessions := service.NewRefreshService(service.RefreshDeps{
		Pool:   pool,
		Repo:   refreshAdapter{Repository: repo},
		Tokens: tokens,
		Events: events,
		Keys:   keys,
		Clock:  fake,
		NewID:  id.NewUUIDv7,
		TTL:    refreshTTL,
	})

	provider := newStubProvider(providerEmail)
	return &oauthHarness{
		service: service.NewOAuthService(service.OAuthDeps{
			Pool:     pool,
			Repo:     oauthAdapter{Repository: repo},
			Provider: provider,
			Accounts: oauthAccounts{t: t},
			Sessions: sessions,
			Events:   events,
			Keys:     keys,
			Clock:    fake,
			NewID:    id.NewUUIDv7,
		}),
		provider: provider,
		clock:    fake,
		keys:     keys,
	}
}

// begin runs a real Start and returns the state the browser comes back with.
func (h *oauthHarness) begin(t *testing.T) string {
	t.Helper()

	if _, err := h.service.Start(context.Background(), ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	state := h.provider.issuedState()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM core.oauth_states WHERE state = $1`, state)
	})
	return state
}

func (h *oauthHarness) callback(t *testing.T) (service.SignedIn, error) {
	t.Helper()
	return h.service.Callback(context.Background(), service.CallbackInput{
		Code: oauthCode, State: h.begin(t), ClientIP: "203.0.113.7", UserAgent: "integration-suite",
	})
}

// ------------------------------------------------------------------ assertions

// identityRows counts the linked identities for a subject. Zero is the assertion
// every refusal in this file makes.
func identityRows(t *testing.T, subject string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.oauth_identities WHERE provider = 'google' AND subject = $1`,
		subject).Scan(&count); err != nil {
		t.Fatalf("count oauth identities: %v", err)
	}
	return count
}

func accountRows(t *testing.T, email string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.users WHERE email = $1`, email).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return count
}

// stateRefusalEvents counts the security events this module has raised for a
// refused OAuth state.
//
// A refused state names no account, so these cannot be found by user id the way
// securityEvents finds a refresh reuse — which is the point: there is nobody to
// attribute a forged callback to. Tests count the delta across the call instead.
func stateRefusalEvents(t *testing.T) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM ops.outbox_events
		WHERE aggregate = $1 AND event = $2 AND payload->>'kind' = $3`,
		contract.Aggregate,
		outbox.BareEventName(contract.Aggregate, contract.EventSecurityEvent),
		contract.SecurityKindOAuthStateInvalid).Scan(&count); err != nil {
		t.Fatalf("count state refusal events: %v", err)
	}
	return count
}

// seedVerifiedUser writes an account that has proved its address.
func seedVerifiedUser(t *testing.T, email string) uuid.UUID {
	t.Helper()

	userID := seedUser(t, email)
	if _, err := pool.Exec(context.Background(),
		`UPDATE core.users SET email_verified_at = now() WHERE id = $1`, userID); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	return userID
}

// seedCredential gives an account a password, so it has a second sign-in method.
// The hash is a real Argon2id PHC string because ck_credentials_hash_is_argon2id
// rejects anything else — the constraint exists to stop a plaintext password
// reaching the column, and it applies to fixtures too.
func seedCredential(t *testing.T, userID uuid.UUID) {
	t.Helper()

	hash, err := domain.NewHasher(domain.DefaultHashParams()).Hash("integration-suite-password")
	if err != nil {
		t.Fatalf("hash fixture password: %v", err)
	}
	credentialID, err := id.NewUUIDv7(context.Background())
	if err != nil {
		t.Fatalf("generate credential id: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO core.credentials (id, user_id, password_hash) VALUES ($1, $2, $3)`,
		credentialID, userID, hash); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
}

// ----------------------------------------------------------- the five branches

// TestAGoogleEmailMatchingAnUnverifiedLocalAccountWritesNoIdentity is the
// account-takeover path, and it is the reason this card exists.
//
// The sequence it prevents: somebody registers an address they do not own and
// never verifies it, the real owner later signs in with Google, and the account
// they land in is the attacker's — same rows, same password, now holding the
// victim's work. Registration proves nothing about an address; only the OTP
// does.
//
// The assertion that carries the weight is the row count, not the error. A 409
// with an identity written behind it is the same compromise as a 200, because
// the next request that finds the row completes the takeover.
func TestAGoogleEmailMatchingAnUnverifiedLocalAccountWritesNoIdentity(t *testing.T) {
	const email = "unverified-match@fluentra.test"

	harness := newOAuthHarness(t, email)
	// Registered, never verified: a *claim* on the address, not ownership of it.
	claimed := seedUser(t, email)
	seedCredential(t, claimed)

	_, err := harness.callback(t)

	assertCode(t, err, "OAUTH_ACCOUNT_CONFLICT")
	if rows := identityRows(t, oauthSubject); rows != 0 {
		t.Fatalf("%d identity rows exist after the refusal; the refusal is worth nothing if the "+
			"link it refused is in the table afterwards", rows)
	}

	var sessions int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.sessions WHERE user_id = $1`, claimed).Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Errorf("%d sessions were opened on the account that was refused", sessions)
	}
}

// TestGoogleSignInWithNoLocalAccountCreatesOneAlreadyVerified is BR-AUTH-19.
//
// Verified on creation because Google has performed exactly the check the OTP
// would have. Leaving it unverified would also mean the learner's *next* Google
// sign-in matched an unverified local account and was refused by the branch
// above — a first sign-in that quietly breaks the second.
func TestGoogleSignInWithNoLocalAccountCreatesOneAlreadyVerified(t *testing.T) {
	const email = "brand-new@fluentra.test"

	harness := newOAuthHarness(t, email)

	signedIn, err := harness.callback(t)
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}

	var verifiedAt *time.Time
	var userID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT id, email_verified_at FROM core.users WHERE email = $1`, email).
		Scan(&userID, &verifiedAt); err != nil {
		t.Fatalf("read the created account: %v", err)
	}
	if verifiedAt == nil {
		t.Error("the account was created unverified, so the learner's next Google sign-in " +
			"would match an unverified local account and be refused")
	}
	if signedIn.Session.UserID != userID {
		t.Errorf("signed in as %s but created %s", signedIn.Session.UserID, userID)
	}
	if rows := identityRows(t, oauthSubject); rows != 1 {
		t.Errorf("identity rows = %d, want exactly one linking the new account to Google", rows)
	}

	// No credential: a Google-only account has no password, which is why
	// core.credentials is a separate table rather than columns on core.users.
	var credentials int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM core.credentials WHERE user_id = $1`, userID).Scan(&credentials); err != nil {
		t.Fatalf("count credentials: %v", err)
	}
	if credentials != 0 {
		t.Errorf("%d credentials were written for an account that signed in with Google", credentials)
	}
}

// TestGoogleSignInLinksToAVerifiedLocalAccount: both sides have proved the same
// address, so they are the same person.
func TestGoogleSignInLinksToAVerifiedLocalAccount(t *testing.T) {
	const email = "verified-match@fluentra.test"

	harness := newOAuthHarness(t, email)
	userID := seedVerifiedUser(t, email)
	seedCredential(t, userID)

	signedIn, err := harness.callback(t)
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	if signedIn.Session.UserID != userID {
		t.Errorf("signed in as %s, want the matching verified account %s", signedIn.Session.UserID, userID)
	}

	var linkedTo uuid.UUID
	var emailHash []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT user_id, email_hash FROM core.oauth_identities WHERE provider = 'google' AND subject = $1`,
		oauthSubject).Scan(&linkedTo, &emailHash); err != nil {
		t.Fatalf("read the identity: %v", err)
	}
	if linkedTo != userID {
		t.Errorf("the identity is linked to %s, want %s", linkedTo, userID)
	}
	if len(emailHash) != 32 {
		t.Errorf("email_hash is %d bytes, want the keyed 32", len(emailHash))
	}
	if strings.Contains(string(emailHash), email) {
		t.Error("the stored email hash contains the address, which makes the table an address book")
	}
}

// TestGoogleSignInWithAKnownIdentityDoesNotConsultTheAddress.
//
// The identity is the subject. Re-deriving the account from an address Google
// may since have reassigned would mean whoever inherits a corporate mailbox
// inherits the Fluentra account with it.
func TestGoogleSignInWithAKnownIdentityDoesNotConsultTheAddress(t *testing.T) {
	const original = "known-identity@fluentra.test"

	harness := newOAuthHarness(t, original)
	userID := seedVerifiedUser(t, original)
	seedCredential(t, userID)

	if _, err := harness.callback(t); err != nil {
		t.Fatalf("the first sign-in failed: %v", err)
	}

	// Google now asserts a different address for the same subject — a learner
	// who changed it there. There is no local account with that address at all.
	harness.provider.identity.Email = "changed-at-google@fluentra.test"

	signedIn, err := harness.callback(t)
	if err != nil {
		t.Fatalf("the second sign-in failed: %v", err)
	}
	if signedIn.Session.UserID != userID {
		t.Errorf("signed in as %s, want the account the subject is linked to (%s)",
			signedIn.Session.UserID, userID)
	}
	if rows := identityRows(t, oauthSubject); rows != 1 {
		t.Errorf("identity rows = %d, want the one that already existed", rows)
	}
	if accounts := accountRows(t, "changed-at-google@fluentra.test"); accounts != 0 {
		t.Errorf("%d accounts were created for the new address; the subject already named one", accounts)
	}
}

// TestAnUnverifiedGoogleEmailIsRefusedBeforeAnythingIsMatched is BR-AUTH-15, and
// the ordering carries as much of the policy as the refusal does.
//
// The local account here is verified and would otherwise be linked to. It is
// never consulted: an address Google will not vouch for is worth no more than
// one typed into a form, and matching on it would let anyone able to make Google
// emit an unverified claim walk straight into the link-to-verified branch.
func TestAnUnverifiedGoogleEmailIsRefusedBeforeAnythingIsMatched(t *testing.T) {
	const email = "unverified-at-google@fluentra.test"

	harness := newOAuthHarness(t, email)
	seedVerifiedUser(t, email)
	harness.provider.identity.EmailVerified = false

	_, err := harness.callback(t)

	assertCode(t, err, "OAUTH_EMAIL_UNVERIFIED")
	if rows := identityRows(t, oauthSubject); rows != 0 {
		t.Errorf("%d identity rows were written for an address Google would not vouch for", rows)
	}
}

// ------------------------------------------------------------------- the state

// TestAForgedStateIsRefusedAndRaisesASecurityEvent: a callback carrying a value
// this server never issued is not a callback from a flow anybody started here.
func TestAForgedStateIsRefusedAndRaisesASecurityEvent(t *testing.T) {
	harness := newOAuthHarness(t, "forged-state@fluentra.test")
	before := stateRefusalEvents(t)

	_, err := harness.service.Callback(context.Background(), service.CallbackInput{
		Code: oauthCode, State: "a-state-this-server-never-issued",
	})

	assertCode(t, err, "OAUTH_STATE_INVALID")
	assertOneStateRefusalRecorded(t, before)
}

// TestAReusedStateIsRefusedAndRaisesASecurityEvent is single use, enforced by
// the same guarded UPDATE shape the refresh claim uses. It is what makes the
// state worth having at all (BR-AUTH-17).
func TestAReusedStateIsRefusedAndRaisesASecurityEvent(t *testing.T) {
	harness := newOAuthHarness(t, "reused-state@fluentra.test")
	state := harness.begin(t)

	first := service.CallbackInput{Code: oauthCode, State: state}
	if _, err := harness.service.Callback(context.Background(), first); err != nil {
		t.Fatalf("the first callback failed: %v", err)
	}

	before := stateRefusalEvents(t)
	_, err := harness.service.Callback(context.Background(), first)

	assertCode(t, err, "OAUTH_STATE_INVALID")
	assertOneStateRefusalRecorded(t, before)
}

// TestAnExpiredStateIsRefusedAndRaisesASecurityEvent closes the window on a
// consent screen left open. After ten minutes the row is no longer a credential.
func TestAnExpiredStateIsRefusedAndRaisesASecurityEvent(t *testing.T) {
	harness := newOAuthHarness(t, "expired-state@fluentra.test")
	state := harness.begin(t)
	before := stateRefusalEvents(t)

	harness.clock.Advance(service.DefaultOAuthStateTTL + time.Second)

	_, err := harness.service.Callback(context.Background(), service.CallbackInput{
		Code: oauthCode, State: state,
	})

	assertCode(t, err, "OAUTH_STATE_INVALID")
	assertOneStateRefusalRecorded(t, before)
}

// assertOneStateRefusalRecorded is the half that a refusal alone does not give.
//
// A refused state leaves no other trace — no session, no identity, no login
// attempt — so without the outbox row a campaign of them is invisible. One of
// these is ordinary; the rate is the signal, and there is no rate to see if
// nothing is written.
func assertOneStateRefusalRecorded(t *testing.T, before int) {
	t.Helper()

	if after := stateRefusalEvents(t); after != before+1 {
		t.Errorf("state refusal events went from %d to %d, want exactly one more: a refused "+
			"state leaves no other trace, so an unrecorded one is an invisible one", before, after)
	}
}

// TestTwoConcurrentCallbacksWithOneStateProduceExactlyOneSignIn is the guarded
// UPDATE under the concurrency it exists for.
//
// A read-then-write consume passes every sequential test in this file and lets
// both callbacks through here — which is two sessions from one authorization,
// and precisely the state single use is supposed to make impossible. The same
// shape, and the same test, as the refresh claim's.
func TestTwoConcurrentCallbacksWithOneStateProduceExactlyOneSignIn(t *testing.T) {
	harness := newOAuthHarness(t, "concurrent-state@fluentra.test")
	input := service.CallbackInput{Code: oauthCode, State: harness.begin(t)}

	var start sync.WaitGroup
	start.Add(1)

	results := make(chan error, 2)
	var finished sync.WaitGroup
	for range 2 {
		finished.Add(1)
		go func() {
			defer finished.Done()
			start.Wait()
			_, err := harness.service.Callback(context.Background(), input)
			results <- err
		}()
	}
	start.Done()
	finished.Wait()
	close(results)

	succeeded, refused := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrOAuthStateInvalid):
			refused++
		default:
			t.Fatalf("unexpected failure: %v", err)
		}
	}

	if succeeded != 1 || refused != 1 {
		t.Fatalf("%d callbacks succeeded and %d were refused, want exactly one of each: two "+
			"sessions from one authorization is what single use exists to prevent", succeeded, refused)
	}
	if rows := identityRows(t, oauthSubject); rows != 1 {
		t.Errorf("identity rows = %d, want one", rows)
	}
}

// ------------------------------------------------------------------ the token

// TestAnIDTokenFailingVerificationLeavesEveryTableEmpty is BR-AUTH-18's second
// half, and it asserts the tables rather than the error.
//
// "It returned an error" is not the property. The property is that there is
// nothing to clean up: no account, no identity, no session, no credential.
// Everything is checked before anything is written, so at the moment a check
// fails there is nothing to undo — which is a stronger guarantee than rolling
// back, and the only one available when the account belongs to another module's
// transaction.
func TestAnIDTokenFailingVerificationLeavesEveryTableEmpty(t *testing.T) {
	const email = "bad-token@fluentra.test"

	for name, failure := range map[string]error{
		"a bad signature":        errors.New("google id token is not valid: signature is invalid"),
		"a foreign issuer":       errors.New("google id token is not valid: issuer \"evil.test\""),
		"another app's audience": errors.New("google id token is not valid: audience"),
		"an expired token":       errors.New("google id token is not valid: token is expired"),
		"a replayed nonce":       errors.New("google id token is not valid: nonce does not match this flow"),
	} {
		t.Run(name, func(t *testing.T) {
			harness := newOAuthHarness(t, email)
			harness.provider.verifyErr = failure

			_, err := harness.callback(t)

			assertCode(t, err, "TOKEN_INVALID")
			if rows := identityRows(t, oauthSubject); rows != 0 {
				t.Errorf("%d identity rows exist after a token that failed verification", rows)
			}
			if accounts := accountRows(t, email); accounts != 0 {
				t.Errorf("%d accounts exist after a token that failed verification; a partial "+
					"account is exactly the state BR-AUTH-18 exists to prevent", accounts)
			}
		})
	}
}

// ------------------------------------------------------------------- unlinking

// TestUnlinkingTheOnlySignInMethodIsRefused is BR-AUTH-20.
//
// An account opened through Google has no password, so removing the identity
// would lock the learner out with no recovery path at all — `forgot-password`
// cannot help somebody who has no password to reset, and there would be no
// identity left to sign in with either.
func TestUnlinkingTheOnlySignInMethodIsRefused(t *testing.T) {
	const email = "google-only@fluentra.test"

	harness := newOAuthHarness(t, email)
	signedIn, err := harness.callback(t)
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	userID := signedIn.Session.UserID

	err = harness.service.Unlink(context.Background(), httpx.Actor{UserID: userID})

	assertCode(t, err, "LAST_SIGN_IN_METHOD")
	if rows := identityRows(t, oauthSubject); rows != 1 {
		t.Fatalf("the identity was removed despite the refusal, which is the lockout itself")
	}
}

// TestUnlinkingSucceedsWhenAPasswordRemains: the count is of ways in, and a
// password is one.
func TestUnlinkingSucceedsWhenAPasswordRemains(t *testing.T) {
	const email = "google-and-password@fluentra.test"

	harness := newOAuthHarness(t, email)
	userID := seedVerifiedUser(t, email)
	seedCredential(t, userID)

	if _, err := harness.callback(t); err != nil {
		t.Fatalf("Callback: %v", err)
	}
	if rows := identityRows(t, oauthSubject); rows != 1 {
		t.Fatalf("the identity was not linked, so there is nothing to unlink")
	}

	if err := harness.service.Unlink(context.Background(), httpx.Actor{UserID: userID}); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if rows := identityRows(t, oauthSubject); rows != 0 {
		t.Errorf("%d identity rows survived the unlink", rows)
	}
}

// TestUnlinkingNothingSucceeds keeps the operation idempotent: the caller's goal
// already holds, and 204 is what makes a retry safe.
func TestUnlinkingNothingSucceeds(t *testing.T) {
	const email = "nothing-linked@fluentra.test"

	harness := newOAuthHarness(t, email)
	userID := seedVerifiedUser(t, email)
	seedCredential(t, userID)

	if err := harness.service.Unlink(context.Background(), httpx.Actor{UserID: userID}); err != nil {
		t.Errorf("Unlink with nothing linked = %v, want nil", err)
	}
}

// -------------------------------------------------------------------- linking

// TestLinkingRefusesAGoogleAccountWithADifferentAddress is the takeover the
// conflict branch refuses, arrived at from the other side: linking an address
// the account's owner cannot receive mail at attaches a way in they do not
// control, to an account that may be full of their work.
func TestLinkingRefusesAGoogleAccountWithADifferentAddress(t *testing.T) {
	const email = "link-owner@fluentra.test"

	harness := newOAuthHarness(t, "somebody-else@fluentra.test")
	userID := seedVerifiedUser(t, email)
	seedCredential(t, userID)

	_, err := harness.service.Link(context.Background(), httpx.Actor{UserID: userID}, service.CallbackInput{
		Code: oauthCode, State: harness.begin(t),
	})

	assertCode(t, err, "OAUTH_EMAIL_MISMATCH")
	if rows := identityRows(t, oauthSubject); rows != 0 {
		t.Errorf("%d identity rows were written by a refused link", rows)
	}
}

// TestLinkingRefusesAnIdentityAlreadyOnAnotherAccount keeps one Google account
// to one Fluentra account — enforced here for a clear refusal and by
// uq_oauth_identities_subject underneath, which is what makes it true under
// concurrency rather than merely usually true.
func TestLinkingRefusesAnIdentityAlreadyOnAnotherAccount(t *testing.T) {
	const first = "link-first@fluentra.test"
	const second = "link-second@fluentra.test"

	harness := newOAuthHarness(t, first)
	firstUser := seedVerifiedUser(t, first)
	seedCredential(t, firstUser)
	secondUser := seedVerifiedUser(t, second)
	seedCredential(t, secondUser)

	if _, err := harness.callback(t); err != nil {
		t.Fatalf("the first sign-in failed: %v", err)
	}

	// The second account now presents the same Google identity, and its own
	// address, so the mismatch check cannot be what refuses it.
	harness.provider.identity.Email = second

	_, err := harness.service.Link(context.Background(), httpx.Actor{UserID: secondUser}, service.CallbackInput{
		Code: oauthCode, State: harness.begin(t),
	})

	assertCode(t, err, "OAUTH_ALREADY_LINKED")
	if rows := identityRows(t, oauthSubject); rows != 1 {
		t.Errorf("identity rows = %d, want the one belonging to the first account", rows)
	}
}
