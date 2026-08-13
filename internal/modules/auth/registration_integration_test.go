//go:build integration

// The first leg of the WP2 gate: register → OTP → signed in, against a real
// PostgreSQL.
//
// Every step of it was already covered — the challenge subsystem has its own
// integration suite, `TestVerifyEmail_MarksAddressVerified` proves the account is
// marked, and the contract suite proves the `AuthSession` shape — but nothing
// walked the whole path against the database. That gap is worth closing rather
// than reasoning around, because the parts that carry it are the ones the unit
// tests replace with fakes: the credential and the challenge committing in one
// transaction, the outbox row that delivers the code, and the session rows the
// learner is signed in with at the end.
package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// registrationPassphrase is long enough for the policy and is not a secret in
// any deployment.
const registrationPassphrase = "a-long-enough-passphrase" // gitleaks:allow

// learnerDisplayName is what registration and Google sign-in both open an
// account with. The anti-impersonation deny-list rejects `admin`, `support` and
// friends, so a fixture name is not a free choice.
const learnerDisplayName = "A Learner"

type registrationHarness struct {
	service  *service.RegisterService
	tokens   *service.TokenService
	sessions *service.RefreshService
}

func newRegistrationHarness(t *testing.T) *registrationHarness {
	t.Helper()
	if pool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	// The real clock. This suite asserts that rows appear and a token verifies,
	// none of which depends on controlling time — and a fake clock here would
	// make the JWT's own expiry checking meaningless.
	base := newRefreshHarness(t, "registration-journey-unused@fluentra.test", nil)
	repo := repository.New(pool)

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	// Cheap Argon2id parameters, for the reason the password suite gives: this
	// is about which rows exist, not about how long a derivation takes, and the
	// production cost is asserted in the domain suite.
	hasher := domain.NewHasher(domain.HashParams{
		MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})

	challenges := service.NewChallengeService(service.ChallengeDeps{
		Repo:  challengeAdapter{Repository: repo},
		Keys:  keys,
		Clock: base.clock,
		NewID: id.NewUUIDv7,
		Env:   "test",
	})

	return &registrationHarness{
		service: service.NewRegisterService(service.RegisterDeps{
			Pool:        pool,
			Accounts:    writableAccounts{t: t},
			Credentials: credentialAdapter{Repository: repo},
			Challenges:  challenges,
			Hasher:      hasher,
			// An empty policy: no breach checker, so nothing here reaches out to
			// HIBP. The policy's own rules have a domain suite.
			Policy:   domain.Policy{},
			Events:   eventWriter{Writer: outbox.NewWriter()},
			Clock:    base.clock,
			NewID:    id.NewUUIDv7,
			Sessions: base.service,
		}),
		tokens:   base.tokens,
		sessions: base.service,
	}
}

// TestRegisterThenVerifyLeavesTheLearnerSignedIn is the WP2 gate's first leg,
// end to end.
//
// The last assertion is the one that makes it a *journey* rather than three
// tests in a row: the refresh token handed back by verification is spent for a
// new one. A learner who has just typed a code from their inbox is signed in and
// stays signed in — if verification returned a session that could not be
// renewed, every one of them would be signed out fifteen minutes later and every
// test above this line would still pass.
func TestRegisterThenVerifyLeavesTheLearnerSignedIn(t *testing.T) {
	const email = "journey@fluentra.test"

	harness := newRegistrationHarness(t)
	ctx := context.Background()

	// ---- register -------------------------------------------------------
	issued, err := harness.service.Register(ctx, service.Registration{
		Email:       email,
		Password:    registrationPassphrase,
		DisplayName: learnerDisplayName,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	userID := accountID(t, email)
	if emailVerifiedAt(t, userID) != nil {
		t.Error("the account is verified before anybody proved the address")
	}
	if credentialsFor(t, userID) != 1 {
		t.Error("registration stored no credential, so the password could never be used")
	}

	// The code reaches the learner through the outbox and nowhere else. Without
	// this row, registration has issued a challenge nobody can ever complete.
	if events := verificationEventsFor(t, issued.Challenge.ID); events != 1 {
		t.Fatalf("%d auth.verification_requested rows for this challenge, want exactly one", events)
	}

	// ---- verify ---------------------------------------------------------
	verification, err := harness.service.VerifyEmail(ctx, issued.Challenge.ID, issued.Code.Reveal())
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if verification.Purpose != domain.PurposeVerifyEmail {
		t.Errorf("purpose = %s, want verify_email", verification.Purpose)
	}
	if emailVerifiedAt(t, userID) == nil {
		t.Error("the address was proved and the account is still unverified")
	}

	// ---- signed in ------------------------------------------------------
	//
	// Not "a session struct came back" — the rows exist, the access token
	// verifies, and the refresh token can be spent.
	actor, err := harness.tokens.Verify(ctx, verification.Session.AccessToken.Reveal())
	if err != nil {
		t.Fatalf("the access token handed to the learner does not verify: %v", err)
	}
	if actor.UserID != userID {
		t.Errorf("the token names %s, want the account just verified (%s)", actor.UserID, userID)
	}
	if sessionsFor(t, userID) != 1 {
		t.Error("verification signed the learner in without opening a session row")
	}
	if liveRefreshTokensFor(t, userID) != 1 {
		t.Error("no refresh token was issued, so the sign-in dies with the access token")
	}

	rotated, err := harness.sessions.Rotate(ctx, verification.RefreshToken.Reveal())
	if err != nil {
		t.Fatalf("the refresh token from verification cannot be spent: %v — a learner who "+
			"registered would be signed out at the first renewal", err)
	}
	if rotated.Session.UserID != userID {
		t.Errorf("rotation returned a session for %s, want %s", rotated.Session.UserID, userID)
	}
}

// TestTheOTPCodeReachesTheOutboxAndNothingElse keeps BR-AUTH-10's promise where
// it is easiest to break by accident.
//
// The code is a credential. It is stored only as an HMAC, and the single place
// it exists in the clear is the outbox payload the mailer consumes — which is a
// known, recorded exposure with a filed fix. What must never happen is a second
// copy: in the challenge row, in the session, or in the account.
func TestTheOTPCodeReachesTheOutboxAndNothingElse(t *testing.T) {
	const email = "journey-code@fluentra.test"

	harness := newRegistrationHarness(t)
	ctx := context.Background()

	issued, err := harness.service.Register(ctx, service.Registration{
		Email:       email,
		Password:    registrationPassphrase,
		DisplayName: learnerDisplayName,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	code := issued.Code.Reveal()
	if code == "" {
		t.Fatal("no code was issued")
	}

	var storedHash []byte
	if err := pool.QueryRow(ctx,
		`SELECT code_hash FROM core.auth_challenges WHERE id = $1`, issued.Challenge.ID).
		Scan(&storedHash); err != nil {
		t.Fatalf("read the challenge: %v", err)
	}
	if strings.Contains(string(storedHash), code) {
		t.Error("the challenge row contains the code in the clear")
	}
	if len(storedHash) != 32 {
		t.Errorf("code_hash is %d bytes, want an HMAC-SHA256", len(storedHash))
	}

	verification, err := harness.service.VerifyEmail(ctx, issued.Challenge.ID, code)
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}

	// The session the learner is handed carries no trace of the code that
	// produced it.
	if strings.Contains(verification.Session.AccessToken.Reveal(), code) {
		t.Error("the access token contains the OTP code")
	}
	if strings.Contains(verification.RefreshToken.Reveal(), code) {
		t.Error("the refresh token contains the OTP code")
	}
}

// TestASecondRegistrationOfAnUnverifiedAddressReplacesThePassword is the
// account-takeover path on the *local* side, and it is the mirror of the Google
// one P2.10 closed.
//
// Whoever claims an address first must not keep the password after its real
// owner registers and verifies. It has a unit test; this proves it against the
// stored hash rather than against a fake's bookkeeping.
func TestASecondRegistrationOfAnUnverifiedAddressReplacesThePassword(t *testing.T) {
	const email = "journey-claimed@fluentra.test"
	const claimantPassword = "the-first-claimants-passphrase" // gitleaks:allow

	harness := newRegistrationHarness(t)
	ctx := context.Background()

	if _, err := harness.service.Register(ctx, service.Registration{
		Email: email, Password: claimantPassword, DisplayName: "A Claimant",
	}); err != nil {
		t.Fatalf("the first registration failed: %v", err)
	}

	// The real owner registers the same address, which is still unverified.
	issued, err := harness.service.Register(ctx, service.Registration{
		Email: email, Password: registrationPassphrase, DisplayName: "The Owner",
	})
	if err != nil {
		t.Fatalf("the second registration failed: %v", err)
	}

	userID := accountID(t, email)
	if credentialsFor(t, userID) != 1 {
		t.Error("the second registration left two credentials for one account")
	}
	if !passwordVerifies(t, userID, registrationPassphrase) {
		t.Error("the stored password is not the one the second registrant chose")
	}
	if passwordVerifies(t, userID, claimantPassword) {
		t.Error("the first claimant's password still opens the account — whoever claimed the " +
			"address first keeps it after its real owner verifies, which is the takeover")
	}

	// And the owner completes the journey normally.
	if _, err := harness.service.VerifyEmail(ctx, issued.Challenge.ID, issued.Code.Reveal()); err != nil {
		t.Fatalf("VerifyEmail after the replacement: %v", err)
	}
	if emailVerifiedAt(t, userID) == nil {
		t.Error("the account is still unverified after a successful verification")
	}
}

// ------------------------------------------------------------------ readers

func accountID(t *testing.T, email string) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM core.users WHERE email = $1`, email).Scan(&userID); err != nil {
		t.Fatalf("no account for %s: %v", email, err)
	}
	return userID
}

func emailVerifiedAt(t *testing.T, userID uuid.UUID) *string {
	t.Helper()

	var verifiedAt *string
	if err := pool.QueryRow(context.Background(),
		`SELECT email_verified_at::text FROM core.users WHERE id = $1`, userID).Scan(&verifiedAt); err != nil {
		t.Fatalf("read email_verified_at: %v", err)
	}
	return verifiedAt
}

func credentialsFor(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	return countRows(t, `SELECT count(*) FROM core.credentials WHERE user_id = $1`, userID)
}

func sessionsFor(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	return countRows(t, `SELECT count(*) FROM core.sessions WHERE user_id = $1 AND revoked_at IS NULL`, userID)
}

// liveRefreshTokensFor counts what the learner could still spend: neither used
// nor revoked.
func liveRefreshTokensFor(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	return countRows(t, `
		SELECT count(*) FROM core.refresh_tokens r
		JOIN core.sessions s ON s.id = r.session_id
		WHERE s.user_id = $1 AND r.used_at IS NULL AND r.revoked_at IS NULL`, userID)
}

// verificationEventsFor counts the outbox rows that deliver a challenge's code.
// The aggregate prefix is stripped by the writer, so the stored name is bare.
func verificationEventsFor(t *testing.T, challengeID uuid.UUID) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM ops.outbox_events
		WHERE aggregate = $1 AND event = $2 AND payload->>'challenge_id' = $3`,
		contract.Aggregate,
		outbox.BareEventName(contract.Aggregate, contract.EventVerificationRequested),
		challengeID.String()).Scan(&count); err != nil {
		t.Fatalf("count verification events: %v", err)
	}
	return count
}

func countRows(t *testing.T, query string, args ...any) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
