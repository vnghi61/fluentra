package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

// The enumeration safety, the revocation and the thirty-minute window are proved
// against PostgreSQL in the module integration suite. What is here is the set of
// branches a working database cannot produce: an account with no password at
// all, a collaborator that fails, and the ordering question of whether a weak
// new password costs the learner their code.

type fakePasswordAccounts struct {
	accounts map[string]service.Account
	findErr  error
	contact  service.Contact
	whoErr   error
}

func (f *fakePasswordAccounts) FindByEmail(_ context.Context, email string) (service.Account, bool, error) {
	if f.findErr != nil {
		return service.Account{}, false, f.findErr
	}
	account, ok := f.accounts[email]
	return account, ok, nil
}

func (f *fakePasswordAccounts) Recipient(context.Context, uuid.UUID) (service.Contact, error) {
	if f.whoErr != nil {
		return service.Contact{}, f.whoErr
	}
	return f.contact, nil
}

type fakePasswordCredentials struct {
	hashes     map[uuid.UUID]string
	getErr     error
	replaceErr error
	replaced   int
}

func (f *fakePasswordCredentials) GetByUserID(_ context.Context, userID uuid.UUID) (domain.Credential, error) {
	if f.getErr != nil {
		return domain.Credential{}, f.getErr
	}
	hash, ok := f.hashes[userID]
	if !ok {
		return domain.Credential{}, domain.ErrCredentialNotFound
	}
	return domain.Credential{UserID: userID, PasswordHash: secret.New(hash)}, nil
}

func (f *fakePasswordCredentials) ReplaceHash(
	_ context.Context, userID uuid.UUID, passwordHash string,
) (domain.Credential, error) {
	if f.replaceErr != nil {
		return domain.Credential{}, f.replaceErr
	}
	f.hashes[userID] = passwordHash
	f.replaced++
	return domain.Credential{UserID: userID}, nil
}

func (f *fakePasswordCredentials) WithTx(pgx.Tx) service.Credentials { return credentialsView{f} }

// credentialsView satisfies the wider Credentials interface the transactional
// write goes through. Create is not reachable from this service and says so.
type credentialsView struct{ *fakePasswordCredentials }

func (c credentialsView) Create(context.Context, uuid.UUID, uuid.UUID, string) (domain.Credential, error) {
	panic("the password service does not create credentials")
}

func (c credentialsView) WithTx(pgx.Tx) service.Credentials { return c }

type fakeSessionEnder struct {
	all       int
	except    int
	keptID    uuid.UUID
	allErr    error
	exceptErr error
}

func (f *fakeSessionEnder) RevokeAll(context.Context, uuid.UUID) (int, error) {
	if f.allErr != nil {
		return 0, f.allErr
	}
	f.all++
	return 4, nil
}

func (f *fakeSessionEnder) RevokeAllExcept(_ context.Context, _, keep uuid.UUID) (int, error) {
	if f.exceptErr != nil {
		return 0, f.exceptErr
	}
	f.except++
	f.keptID = keep
	return 2, nil
}

type passwordServiceHarness struct {
	service     *service.PasswordService
	accounts    *fakePasswordAccounts
	credentials *fakePasswordCredentials
	sessions    *fakeSessionEnder
	events      *fakeEventWriter
	userID      uuid.UUID
}

const passwordHarnessEmail = "learner@fluentra.test"

func newPasswordServiceHarness(t *testing.T) *passwordServiceHarness {
	t.Helper()

	fakeClock := clock.NewFake(sessionNow)
	userID := uuid.New()

	// Cheap parameters: these tests care which password verifies, not how long
	// deriving it takes.
	hasher := domain.NewHasher(domain.HashParams{
		MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	hash, err := hasher.Hash(currentPassphrase)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	accounts := &fakePasswordAccounts{
		accounts: map[string]service.Account{passwordHarnessEmail: {ID: userID, Verified: true}},
		contact:  service.Contact{Email: passwordHarnessEmail, DisplayName: "Learner", Locale: "en"},
	}
	credentials := &fakePasswordCredentials{hashes: map[uuid.UUID]string{userID: hash}}
	sessions := &fakeSessionEnder{}
	events := newFakeEventWriter()

	keys, err := domain.NewKeyring([]byte("password-service-test-hmac-key-32b--"))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	passwords := service.NewPasswordService(service.PasswordDeps{
		Pool:     &fakePool{},
		Accounts: accounts,
		//nolint:exhaustruct // the fake is the whole surface this service uses
		Credentials: credentials,
		Challenges: service.NewChallengeService(service.ChallengeDeps{
			Repo: newFakeRepository(), Keys: keys, Clock: fakeClock,
			NewID: func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
			Env:   testEnv,
		}),
		Sessions: sessions,
		Hasher:   hasher,
		Policy:   domain.Policy{},
		Events:   events,
		Clock:    fakeClock,
		NewID:    func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	})

	return &passwordServiceHarness{
		service: passwords, accounts: accounts, credentials: credentials,
		sessions: sessions, events: events, userID: userID,
	}
}

const (
	currentPassphrase = "the current passphrase"   // gitleaks:allow
	nextPassphrase    = "a perfectly fine new one" // gitleaks:allow
)

// TestChange_AnAccountWithNoPasswordGetsTheSameRefusalAsAWrongOne is the branch
// a Google-only account takes (P2.10 creates them). Telling the two apart would
// say whether an account has a password at all, which is a fact about how
// somebody signs in that a caller holding a stolen token should not be handed.
func TestChange_AnAccountWithNoPasswordGetsTheSameRefusalAsAWrongOne(t *testing.T) {
	h := newPasswordServiceHarness(t)
	ctx := context.Background()

	noPassword := httpx.Actor{UserID: uuid.New(), SessionID: uuid.New()}
	wrongPassword := httpx.Actor{UserID: h.userID, SessionID: uuid.New()}

	_, missingErr := h.service.Change(ctx, noPassword, service.ChangeInput{
		CurrentPassword: currentPassphrase, NewPassword: nextPassphrase,
	})
	_, wrongErr := h.service.Change(ctx, wrongPassword, service.ChangeInput{
		CurrentPassword: "not the current one", NewPassword: nextPassphrase,
	})

	assertAuthCode(t, missingErr, "INVALID_CREDENTIALS")
	assertAuthCode(t, wrongErr, "INVALID_CREDENTIALS")
	if missingErr.Error() != wrongErr.Error() {
		t.Errorf("the two refusals differ:\n no password: %v\n wrong:       %v", missingErr, wrongErr)
	}
	if h.credentials.replaced != 0 {
		t.Error("a refused change still wrote a credential")
	}
	if h.sessions.except != 0 {
		t.Error("a refused change still revoked sessions")
	}
}

// TestReset_AWeakNewPasswordDoesNotCostTheLearnerTheirCode is an ordering
// question with a real consequence. The policy runs before the challenge is
// consumed, so somebody whose first choice trips the breach corpus can try
// another one with the code they already have — rather than being sent back to
// their inbox for a second email because the first attempt spent it.
func TestReset_AWeakNewPasswordDoesNotCostTheLearnerTheirCode(t *testing.T) {
	h := newPasswordServiceHarness(t)
	ctx := context.Background()

	issued, err := h.service.Forgot(ctx, passwordHarnessEmail)
	if err != nil {
		t.Fatalf("Forgot: %v", err)
	}

	_, err = h.service.Reset(ctx, service.ResetInput{
		ChallengeID: issued.Challenge.ID, Code: issued.Code.Reveal(), Password: "short",
	})
	assertAuthCode(t, err, "PASSWORD_TOO_WEAK")

	// The same code, a password that passes. If the weak attempt had consumed
	// the challenge this would fail, and the learner would be waiting on
	// another email for a mistake they can fix in the form in front of them.
	changed, err := h.service.Reset(ctx, service.ResetInput{
		ChallengeID: issued.Challenge.ID, Code: issued.Code.Reveal(), Password: nextPassphrase,
	})
	if err != nil {
		t.Fatalf("the second attempt with the same code was refused: %v", err)
	}
	if changed.SessionsRevoked != 4 {
		t.Errorf("sessions_revoked = %d, want the count the revoker reported", changed.SessionsRevoked)
	}
}

// TestForgot_DeliversOnlyForAnAddressWithAnAccount pins the one thing that does
// differ between the two paths, and the one place it is allowed to: an address
// with no account has no recipient, so no delivery is requested. The challenge
// is still written, which is what keeps the response indistinguishable.
func TestForgot_DeliversOnlyForAnAddressWithAnAccount(t *testing.T) {
	h := newPasswordServiceHarness(t)
	ctx := context.Background()

	known, err := h.service.Forgot(ctx, passwordHarnessEmail)
	if err != nil {
		t.Fatalf("Forgot known: %v", err)
	}
	unknown, err := h.service.Forgot(ctx, "nobody@fluentra.test")
	if err != nil {
		t.Fatalf("Forgot unknown: %v", err)
	}

	if known.Challenge.ID == uuid.Nil || unknown.Challenge.ID == uuid.Nil {
		t.Fatal("one of the two paths issued no challenge")
	}
	if unknown.Code.Reveal() != "" {
		t.Error("a code was handed back for an address with no account")
	}
	if len(h.events.events) != 1 {
		t.Fatalf("%d delivery requests, want 1 — only the known address has a recipient", len(h.events.events))
	}
}

// TestPasswordOperations_ReportFailuresRatherThanClaimingSuccess covers the
// direction each collaborator has to fail in. Reporting success for a password
// that was not written, or for sessions that were not revoked, tells a learner
// they are safe when they are not.
func TestPasswordOperations_ReportFailuresRatherThanClaimingSuccess(t *testing.T) {
	actor := func(h *passwordServiceHarness) httpx.Actor {
		return httpx.Actor{UserID: h.userID, SessionID: uuid.New()}
	}

	for name, testCase := range map[string]struct {
		brk func(*passwordServiceHarness)
		run func(*passwordServiceHarness) error
	}{
		"the account lookup fails": {
			brk: func(h *passwordServiceHarness) { h.accounts.findErr = errors.New("down") },
			run: func(h *passwordServiceHarness) error {
				_, err := h.service.Forgot(context.Background(), passwordHarnessEmail)
				return err
			},
		},
		"the recipient cannot be read": {
			brk: func(h *passwordServiceHarness) { h.accounts.whoErr = errors.New("down") },
			run: func(h *passwordServiceHarness) error {
				_, err := h.service.Forgot(context.Background(), passwordHarnessEmail)
				return err
			},
		},
		"the credential cannot be written": {
			brk: func(h *passwordServiceHarness) { h.credentials.replaceErr = errors.New("down") },
			run: func(h *passwordServiceHarness) error {
				_, err := h.service.Change(context.Background(), actor(h), service.ChangeInput{
					CurrentPassword: currentPassphrase, NewPassword: nextPassphrase,
				})
				return err
			},
		},
		"the sessions cannot be revoked": {
			brk: func(h *passwordServiceHarness) { h.sessions.exceptErr = errors.New("down") },
			run: func(h *passwordServiceHarness) error {
				_, err := h.service.Change(context.Background(), actor(h), service.ChangeInput{
					CurrentPassword: currentPassphrase, NewPassword: nextPassphrase,
				})
				return err
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newPasswordServiceHarness(t)
			testCase.brk(h)
			if err := testCase.run(h); err == nil {
				t.Fatal("a failure was reported as a success")
			}
		})
	}
}

// TestChange_KeepsTheSessionTheChangeWasMadeFrom is the one parameter that
// distinguishes a change from a reset, checked where it is passed rather than
// only where it lands.
func TestChange_KeepsTheSessionTheChangeWasMadeFrom(t *testing.T) {
	h := newPasswordServiceHarness(t)
	sessionID := uuid.New()

	changed, err := h.service.Change(context.Background(),
		httpx.Actor{UserID: h.userID, SessionID: sessionID},
		service.ChangeInput{CurrentPassword: currentPassphrase, NewPassword: nextPassphrase})
	if err != nil {
		t.Fatalf("Change: %v", err)
	}

	if h.sessions.all != 0 {
		t.Error("a change revoked every session, which is what a reset does")
	}
	if h.sessions.keptID != sessionID {
		t.Errorf("kept session = %s, want the actor's %s", h.sessions.keptID, sessionID)
	}
	if changed.SessionsRevoked != 2 {
		t.Errorf("sessions_revoked = %d, want the count the revoker reported", changed.SessionsRevoked)
	}
}
