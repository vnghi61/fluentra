//go:build integration

package auth_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/id"
	"github.com/fluentra/fluentra/internal/shared/outbox"
)

// resetCode reads the code out of the outbox row the request wrote.
//
// It is the only place it can be read: the challenge row stores an HMAC, and
// the response body deliberately carries the handle and not the code (see the
// note on contract.VerificationRequested for why the payload has it at all).
// Going through the outbox is also what makes this test prove the delivery path
// exists rather than only that a row was written.
func resetCode(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	var code string
	err := pool.QueryRow(context.Background(), `
		SELECT payload->>'code' FROM ops.outbox_events
		WHERE aggregate = 'auth' AND payload->>'user_id' = $1
		ORDER BY event_id DESC LIMIT 1`, userID.String()).Scan(&code)
	if err != nil {
		t.Fatalf("read the code from the outbox: %v", err)
	}
	if code == "" {
		t.Fatal("the outbox row carries no code, so nothing could be delivered")
	}
	return code
}

// TestForgotPasswordAnswersTheSameWayForAnAddressWithNoAccount is the whole
// reason this operation returns 202 unconditionally (BR-AUTH-26).
//
// "We sent you an email" versus "no such account" is precisely the question an
// attacker holding a list of addresses is asking, and answering it turns the
// reset flow into an account-enumeration oracle. The defence is that an unknown
// address has a real challenge issued against it — the same work, the same
// shape, a code nobody is ever given — so there is nothing in the response to
// tell the two apart.
func TestForgotPasswordAnswersTheSameWayForAnAddressWithNoAccount(t *testing.T) {
	h := newPasswordHarness(t, "forgot-known@fluentra.test")
	ctx := context.Background()

	known, err := h.passwords.Forgot(ctx, "forgot-known@fluentra.test")
	if err != nil {
		t.Fatalf("Forgot for a known address: %v", err)
	}
	unknown, err := h.passwords.Forgot(ctx, "nobody-here@fluentra.test")
	if err != nil {
		t.Fatalf("Forgot for an unknown address: %v", err)
	}

	// Both produce a real handle. An empty one for the unknown address would be
	// the oracle wearing a different hat.
	if known.Challenge.ID == uuid.Nil || unknown.Challenge.ID == uuid.Nil {
		t.Fatalf("one of the two got no challenge: known=%s unknown=%s",
			known.Challenge.ID, unknown.Challenge.ID)
	}
	if known.Challenge.Purpose != domain.PurposePasswordReset {
		t.Errorf("purpose = %q, want password_reset", known.Challenge.Purpose)
	}
	if unknown.Challenge.Purpose != known.Challenge.Purpose {
		t.Error("the two challenges differ in purpose")
	}
	if known.Challenge.AttemptsRemaining() != unknown.Challenge.AttemptsRemaining() {
		t.Error("the two challenges differ in attempts remaining")
	}

	// And the difference that would matter most: verifying the unknown
	// address's challenge must fail the way a wrong code fails, not the way a
	// missing challenge fails. OTP_INVALID and CHALLENGE_NOT_FOUND are the two
	// answers that would give the game away.
	_, err = h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: unknown.Challenge.ID, Code: "000000", Password: newPassphrase,
	})
	assertCode(t, err, "OTP_INVALID")
}

// TestASecondResetRequestKillsTheFirstCode is the acceptance criterion, and the
// property it protects is that an old email in an inbox stops being a way in.
func TestASecondResetRequestKillsTheFirstCode(t *testing.T) {
	h := newPasswordHarness(t, "second-request@fluentra.test")
	ctx := context.Background()

	first, err := h.passwords.Forgot(ctx, h.email)
	if err != nil {
		t.Fatalf("first Forgot: %v", err)
	}
	firstCode := resetCode(t, h.userID)

	second, err := h.passwords.Forgot(ctx, h.email)
	if err != nil {
		t.Fatalf("second Forgot: %v", err)
	}
	secondCode := resetCode(t, h.userID)

	if firstCode == secondCode {
		t.Fatal("the second request reissued the same code, so nothing was invalidated")
	}

	// The first code is dead, presented against either handle.
	_, err = h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: first.Challenge.ID, Code: firstCode, Password: newPassphrase,
	})
	if err == nil {
		t.Fatal("the superseded code still reset the password")
	}

	// The second one works.
	changed, err := h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: second.Challenge.ID, Code: secondCode, Password: newPassphrase,
	})
	if err != nil {
		t.Fatalf("the current code was refused: %v", err)
	}
	if changed.ChangedAt.IsZero() {
		t.Error("the reset reported no timestamp")
	}
}

// TestResetRevokesEverySessionAndTheNewPasswordIsTheOnlyOneThatWorks is
// BR-AUTH-05 end to end. A reset is what a learner reaches for when they think
// somebody else is in their account; leaving that somebody signed in would
// defeat the operation they just performed.
func TestResetRevokesEverySessionAndTheNewPasswordIsTheOnlyOneThatWorks(t *testing.T) {
	h := newPasswordHarness(t, "reset-revokes@fluentra.test")
	ctx := context.Background()

	first := h.start(t)
	second := h.start(t)

	issued, err := h.passwords.Forgot(ctx, h.email)
	if err != nil {
		t.Fatalf("Forgot: %v", err)
	}
	changed, err := h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: issued.Challenge.ID, Code: resetCode(t, h.userID), Password: newPassphrase,
	})
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if changed.SessionsRevoked != 2 {
		t.Errorf("sessions_revoked = %d, want 2", changed.SessionsRevoked)
	}
	for _, signedIn := range []service.SignedIn{first, second} {
		if !sessionIsRevoked(t, signedIn.Session.SessionID) {
			t.Errorf("session %s survived the reset", signedIn.Session.SessionID)
		}
		if _, err := h.service.Rotate(ctx, signedIn.RefreshToken.Reveal()); err == nil {
			t.Errorf("session %s can still rotate after the reset", signedIn.Session.SessionID)
		}
	}

	// The stored credential is the new one, and only the new one.
	assertPasswordIs(t, h.userID, newPassphrase)
	assertPasswordIsNot(t, h.userID, originalPassphrase)
}

// TestAResetCodeIsSingleUseAndDiesAtThirtyMinutes covers both halves of the
// window this card widened. Ten minutes is right for a code typed on the screen
// that asked for it; thirty is right for one that has to survive an inbox.
func TestAResetCodeIsSingleUseAndDiesAtThirtyMinutes(t *testing.T) {
	h := newPasswordHarness(t, "reset-window@fluentra.test")
	ctx := context.Background()

	issued, err := h.passwords.Forgot(ctx, h.email)
	if err != nil {
		t.Fatalf("Forgot: %v", err)
	}
	if window := issued.Challenge.ExpiresAt.Sub(harnessNow); window != resetTTL {
		t.Errorf("reset window = %s, want %s", window, resetTTL)
	}

	code := resetCode(t, h.userID)
	if _, err := h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: issued.Challenge.ID, Code: code, Password: newPassphrase,
	}); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Single use: the same code, the same handle, immediately afterwards.
	_, err = h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: issued.Challenge.ID, Code: code, Password: "another passphrase entirely",
	})
	if err == nil {
		t.Fatal("a consumed reset code worked a second time")
	}
	assertPasswordIs(t, h.userID, newPassphrase)

	// And a fresh challenge left to age past the window is refused.
	aged, err := h.passwords.Forgot(ctx, h.email)
	if err != nil {
		t.Fatalf("Forgot again: %v", err)
	}
	agedCode := resetCode(t, h.userID)
	h.clock.Advance(resetTTL + time.Second)

	_, err = h.passwords.Reset(ctx, service.ResetInput{
		ChallengeID: aged.Challenge.ID, Code: agedCode, Password: "yet another passphrase",
	})
	assertCode(t, err, "OTP_EXPIRED")
}

// TestChangePasswordKeepsThisDeviceAndSignsOutTheRest is the difference from a
// reset. Signing a learner out of the machine in front of them, immediately
// after they did the responsible thing, teaches them not to do it again.
func TestChangePasswordKeepsThisDeviceAndSignsOutTheRest(t *testing.T) {
	h := newPasswordHarness(t, "change@fluentra.test")
	ctx := context.Background()

	current := h.start(t)
	other := h.start(t)
	actor := httpx.Actor{UserID: h.userID, SessionID: current.Session.SessionID, TokenID: uuid.NewString()}

	changed, err := h.passwords.Change(ctx, actor, service.ChangeInput{
		CurrentPassword: originalPassphrase, NewPassword: newPassphrase,
	})
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if changed.SessionsRevoked != 1 {
		t.Errorf("sessions_revoked = %d, want 1 — the other device only", changed.SessionsRevoked)
	}

	if sessionIsRevoked(t, current.Session.SessionID) {
		t.Error("the device the change was made from was signed out")
	}
	if !sessionIsRevoked(t, other.Session.SessionID) {
		t.Error("the other device stayed signed in")
	}

	// The device that made the change can still renew, which is what "kept"
	// has to mean for it to be worth anything.
	if _, err := h.service.Rotate(ctx, current.RefreshToken.Reveal()); err != nil {
		t.Errorf("the kept session cannot rotate: %v", err)
	}
	if _, err := h.service.Rotate(ctx, other.RefreshToken.Reveal()); err == nil {
		t.Error("the revoked session can still rotate")
	}

	assertPasswordIs(t, h.userID, newPassphrase)
}

// TestChangePasswordRefusesTheWrongCurrentPasswordAndChangesNothing is the check
// that stops a stolen access token being enough to take an account over. A token
// proves the session was opened by somebody who knew the password; it does not
// prove the person holding it now does.
func TestChangePasswordRefusesTheWrongCurrentPasswordAndChangesNothing(t *testing.T) {
	h := newPasswordHarness(t, "change-wrong@fluentra.test")
	ctx := context.Background()

	current := h.start(t)
	other := h.start(t)
	actor := httpx.Actor{UserID: h.userID, SessionID: current.Session.SessionID, TokenID: uuid.NewString()}

	_, err := h.passwords.Change(ctx, actor, service.ChangeInput{
		CurrentPassword: "not the current password", NewPassword: newPassphrase,
	})
	assertCode(t, err, "INVALID_CREDENTIALS")

	// Nothing moved: not the credential, and not the other device's session.
	assertPasswordIs(t, h.userID, originalPassphrase)
	if sessionIsRevoked(t, other.Session.SessionID) {
		t.Error("a refused change still signed the other device out")
	}
}

// ------------------------------------------------------------------ harness

// The passwords these tests move between. Both clear the twelve-character
// policy; neither is a credential anywhere.
const (
	originalPassphrase = "the original passphrase" // gitleaks:allow
	newPassphrase      = "a perfectly fine passphrase"
)

// resetTTL is the window this card gives a password_reset challenge, and the
// number the harness configures. The tests assert against it rather than
// against a literal so that changing the configuration moves both together.
const resetTTL = 30 * time.Minute

type passwordHarness struct {
	*refreshHarness

	passwords *service.PasswordService
	email     string
}

// sqlAccounts is the slice of the `user` module these tests need, over plain
// SQL. Importing the user module's repository from here would be the boundary
// crossing rule L1 forbids, test or not — the same reason seedUser writes its
// row by hand.
type sqlAccounts struct{}

func (sqlAccounts) FindByEmail(ctx context.Context, email string) (service.Account, bool, error) {
	var account service.Account
	err := pool.QueryRow(ctx, `
		SELECT id, email_verified_at IS NOT NULL, status::text
		FROM core.users WHERE email = $1`, email).Scan(&account.ID, &account.Verified, &account.Status)
	if err != nil {
		return service.Account{}, false, nil //nolint:nilerr // no row means no account, which is not a failure
	}
	return account, true, nil
}

func (sqlAccounts) Recipient(ctx context.Context, userID uuid.UUID) (service.Contact, error) {
	var contact service.Contact
	err := pool.QueryRow(ctx, `SELECT email::text FROM core.users WHERE id = $1`, userID).Scan(&contact.Email)
	if err != nil {
		return service.Contact{}, err
	}
	contact.DisplayName, contact.Locale = "Learner", "en"
	return contact, nil
}

func newPasswordHarness(t *testing.T, email string) *passwordHarness {
	t.Helper()

	base := newRefreshHarness(t, email, newStubDenylist())
	repo := repository.New(pool)

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	// Cheap Argon2id parameters. These tests are about which password verifies,
	// not about how long deriving it takes, and the production cost is asserted
	// in the domain suite.
	hasher := domain.NewHasher(domain.HashParams{
		MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})

	challenges := service.NewChallengeService(service.ChallengeDeps{
		Repo:  challengeAdapter{Repository: repo},
		Keys:  keys,
		Clock: base.clock,
		NewID: id.NewUUIDv7,
		Config: service.Config{
			TTLByPurpose: map[service.Purpose]time.Duration{domain.PurposePasswordReset: resetTTL},
		},
		Env: "test",
	})

	sessions := service.NewSessionService(service.SessionDeps{
		Pool:   pool,
		Repo:   sessionAdapter{Repository: repo},
		Tokens: base.tokens,
		Clock:  base.clock,
	})

	passwords := service.NewPasswordService(service.PasswordDeps{
		Pool:        pool,
		Accounts:    sqlAccounts{},
		Credentials: credentialAdapter{Repository: repo},
		Challenges:  challenges,
		Sessions:    sessions,
		Hasher:      hasher,
		Policy:      domain.Policy{},
		Events:      eventWriter{Writer: outbox.NewWriter()},
		Clock:       base.clock,
		NewID:       id.NewUUIDv7,
	})

	// The account needs a password before it can change one.
	hash, err := hasher.Hash(originalPassphrase)
	if err != nil {
		t.Fatalf("hash the starting password: %v", err)
	}
	credentialID, err := id.NewUUIDv7(context.Background())
	if err != nil {
		t.Fatalf("generate credential id: %v", err)
	}
	if _, err := repo.Create(context.Background(), credentialID, base.userID, hash); err != nil {
		t.Fatalf("seed the credential: %v", err)
	}

	return &passwordHarness{refreshHarness: base, passwords: passwords, email: email}
}

type challengeAdapter struct {
	*repository.Repository
}

func (a challengeAdapter) WithTx(tx pgx.Tx) service.Repository {
	return challengeAdapter{Repository: a.Repository.WithTx(tx)}
}

type credentialAdapter struct {
	*repository.Repository
}

func (a credentialAdapter) WithTx(tx pgx.Tx) service.Credentials {
	return credentialAdapter{Repository: a.Repository.WithTx(tx)}
}

// assertPasswordIs reads the stored hash back and verifies the plaintext
// against it, rather than trusting that the write returned no error.
func assertPasswordIs(t *testing.T, userID uuid.UUID, password string) {
	t.Helper()
	if !passwordVerifies(t, userID, password) {
		t.Errorf("the stored credential does not accept the password it should")
	}
}

func assertPasswordIsNot(t *testing.T, userID uuid.UUID, password string) {
	t.Helper()
	if passwordVerifies(t, userID, password) {
		t.Errorf("the stored credential still accepts a password it should not")
	}
}

func passwordVerifies(t *testing.T, userID uuid.UUID, password string) bool {
	t.Helper()

	stored, err := repository.New(pool).GetByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("read the credential: %v", err)
	}
	hasher := domain.NewHasher(domain.HashParams{
		MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	result, err := stored.Verify(hasher, password)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return result.Matches
}

// TestForgotPasswordTakesComparableTimeForAKnownAndUnknownAddress is the other
// half of BR-AUTH-26. A response that says the same thing but arrives in a
// tenth of the time is the same oracle, read off a stopwatch.
//
// The bound is deliberately loose. What this catches is the regression that
// actually happens — an early return for an address with no account, which is
// two orders of magnitude faster, not twenty percent — and a tight bound on a
// shared CI database would flake on nothing more than a busy neighbour. The
// structural assertion in TestForgotPasswordAnswersTheSameWayForAnAddressWithNoAccount
// is what proves the two paths do the same work; this proves nobody deleted it.
func TestForgotPasswordTakesComparableTimeForAKnownAndUnknownAddress(t *testing.T) {
	h := newPasswordHarness(t, "timing-known@fluentra.test")
	ctx := context.Background()

	const samples = 15
	known := make([]time.Duration, 0, samples)
	unknown := make([]time.Duration, 0, samples)

	for i := range samples {
		start := time.Now()
		if _, err := h.passwords.Forgot(ctx, h.email); err != nil {
			t.Fatalf("Forgot known (%d): %v", i, err)
		}
		known = append(known, time.Since(start))

		start = time.Now()
		if _, err := h.passwords.Forgot(ctx, fmt.Sprintf("nobody-%d@fluentra.test", i)); err != nil {
			t.Fatalf("Forgot unknown (%d): %v", i, err)
		}
		unknown = append(unknown, time.Since(start))
	}

	knownMedian, unknownMedian := median(known), median(unknown)
	t.Logf("median known=%s unknown=%s", knownMedian, unknownMedian)

	if unknownMedian < knownMedian/4 {
		t.Errorf("an unknown address answers in %s against %s for a known one — "+
			"fast enough to enumerate accounts with a stopwatch", unknownMedian, knownMedian)
	}
}

func median(durations []time.Duration) time.Duration {
	sorted := slices.Clone(durations)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}
