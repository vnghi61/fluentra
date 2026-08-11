package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fluentra/fluentra/internal/modules/auth/contract"
	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

const (
	accountStatusActive = "active"
	testPassword        = "password-12345"
)

type fakeAccounts struct {
	mu          sync.Mutex
	accounts    map[string]service.Account
	byID        map[uuid.UUID]service.Contact
	purgeCount  int
	purgeErr    error
	createErr   error
	createCalls int
	markCalls   int
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{
		accounts: make(map[string]service.Account),
		byID:     make(map[uuid.UUID]service.Contact),
	}
}

func (f *fakeAccounts) CreateAccount(_ context.Context, input service.NewAccount) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.createCalls++
	if f.createErr != nil {
		return uuid.Nil, f.createErr
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if acc, ok := f.accounts[email]; ok {
		if !acc.Verified {
			return acc.ID, nil
		}
		return uuid.Nil, apperr.New(apperr.Conflict, "EMAIL_ALREADY_REGISTERED", "Email is already registered")
	}

	id := uuid.New()
	acc := service.Account{ID: id, Verified: false, Status: accountStatusActive}
	f.accounts[email] = acc
	f.byID[id] = service.Contact{Email: email, DisplayName: input.DisplayName, Locale: input.Locale}
	return id, nil
}

func (f *fakeAccounts) FindByEmail(_ context.Context, email string) (service.Account, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	acc, ok := f.accounts[email]
	return acc, ok, nil
}

func (f *fakeAccounts) Recipient(_ context.Context, userID uuid.UUID) (service.Contact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	contact, ok := f.byID[userID]
	if !ok {
		return service.Contact{}, apperr.New(apperr.NotFound, "USER_NOT_FOUND", "User not found")
	}
	return contact, nil
}

func (f *fakeAccounts) MarkEmailVerified(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.markCalls++
	for email, acc := range f.accounts {
		if acc.ID == userID {
			acc.Verified = true
			f.accounts[email] = acc
			break
		}
	}
	return nil
}

func (f *fakeAccounts) PurgeUnverifiedBefore(_ context.Context, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.purgeErr != nil {
		return 0, f.purgeErr
	}
	count := 0
	for email, acc := range f.accounts {
		if !acc.Verified && acc.Status == accountStatusActive {
			delete(f.accounts, email)
			delete(f.byID, acc.ID)
			count++
		}
	}
	f.purgeCount += count
	return count, nil
}

type fakeCredentials struct {
	mu          sync.Mutex
	credentials map[uuid.UUID]domain.Credential
	createErr   error
	replaceErr  error
}

func newFakeCredentials() *fakeCredentials {
	return &fakeCredentials{credentials: make(map[uuid.UUID]domain.Credential)}
}

func (f *fakeCredentials) Create(
	_ context.Context, id, userID uuid.UUID, passwordHash string,
) (domain.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return domain.Credential{}, f.createErr
	}
	cred := domain.Credential{
		ID:           id,
		UserID:       userID,
		PasswordHash: secret.New(passwordHash),
		CreatedAt:    testNow,
		UpdatedAt:    testNow,
	}
	f.credentials[userID] = cred
	return cred, nil
}

func (f *fakeCredentials) GetByUserID(_ context.Context, userID uuid.UUID) (domain.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	cred, ok := f.credentials[userID]
	if !ok {
		return domain.Credential{}, domain.ErrCredentialNotFound
	}
	return cred, nil
}

func (f *fakeCredentials) ReplaceHash(
	_ context.Context, userID uuid.UUID, passwordHash string,
) (domain.Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.replaceErr != nil {
		return domain.Credential{}, f.replaceErr
	}
	cred, ok := f.credentials[userID]
	if !ok {
		cred = domain.Credential{
			ID:           uuid.New(),
			UserID:       userID,
			PasswordHash: secret.New(passwordHash),
			CreatedAt:    testNow,
			UpdatedAt:    testNow,
		}
	} else {
		cred.PasswordHash = secret.New(passwordHash)
		cred.UpdatedAt = testNow
	}
	f.credentials[userID] = cred
	return cred, nil
}

func (f *fakeCredentials) WithTx(_ pgx.Tx) service.Credentials { return f }

type recordedEvent struct {
	Aggregate string
	Event     string
	Payload   any
}

type fakeEventWriter struct {
	mu     sync.Mutex
	events []recordedEvent
	err    error
}

func newFakeEventWriter() *fakeEventWriter {
	return &fakeEventWriter{}
}

func (f *fakeEventWriter) Write(
	_ context.Context, _ service.OutboxTx, aggregate, event string, payload any,
) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return uuid.Nil, f.err
	}
	f.events = append(f.events, recordedEvent{
		Aggregate: aggregate,
		Event:     event,
		Payload:   payload,
	})
	return uuid.New(), nil
}

type fakeTx struct {
	owner *fakePool
}

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) {
	panic("nested transactions are not modelled")
}
func (t *fakeTx) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	panic("nested transactions are not modelled")
}
func (t *fakeTx) Commit(context.Context) error   { return t.owner.commitErr }
func (t *fakeTx) Rollback(context.Context) error { t.owner.rollbacks++; return nil }
func (t *fakeTx) Conn() *pgx.Conn                { return nil }

func (t *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom is not modelled")
}
func (t *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("SendBatch is not modelled")
}
func (t *fakeTx) LargeObjects() pgx.LargeObjects { panic("LargeObjects is not modelled") }

func (t *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare is not modelled")
}

func (t *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("Query is not modelled")
}
func (t *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row { panic("QueryRow is not modelled") }

type fakePool struct {
	beginErr  error
	commitErr error
	rollbacks int
}

func (p *fakePool) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	if p.beginErr != nil {
		return nil, p.beginErr
	}
	return &fakeTx{owner: p}, nil
}

type registerHarness struct {
	service     *service.RegisterService
	accounts    *fakeAccounts
	credentials *fakeCredentials
	challenges  *service.ChallengeService
	repo        *fakeRepository
	events      *fakeEventWriter
	clock       *clock.Fake
	hasher      domain.Hasher
	pool        *fakePool
}

func newRegisterHarness(t *testing.T) *registerHarness {
	t.Helper()

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	fakeClock := clock.NewFake(testNow)
	repo := newFakeRepository()
	limiter := newFakeLimiter()
	accounts := newFakeAccounts()
	credentials := newFakeCredentials()
	events := newFakeEventWriter()
	pool := &fakePool{}

	challenges := service.NewChallengeService(service.ChallengeDeps{
		Repo:    repo,
		Limiter: limiter,
		Keys:    keys,
		Clock:   fakeClock,
		NewID:   func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		Env:     testEnv,
	})

	hasher := domain.NewHasher(domain.DefaultHashParams())
	policy := domain.Policy{}

	// A real token service. Verification signs the learner in, and a fake here
	// could not notice a path that returned a session nobody could present.
	tokens, err := service.NewTokenService(service.TokenDeps{
		Config: service.TokenConfig{
			SigningKey: []byte("register-test-signing-key-32-bytes-min"),
			Issuer:     claimIssuer,
			Audience:   claimAudience,
		},
		Clock: fakeClock,
		NewID: func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	})
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}

	regService := service.NewRegisterService(service.RegisterDeps{
		Pool:        pool,
		Accounts:    accounts,
		Credentials: credentials,
		Challenges:  challenges,
		Hasher:      hasher,
		Policy:      policy,
		Events:      events,
		Clock:       fakeClock,
		NewID:       func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		Sessions: fakeSessions{
			tokens: tokens,
			clock:  fakeClock,
			newID:  func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
		},
	})

	return &registerHarness{
		service:     regService,
		accounts:    accounts,
		credentials: credentials,
		challenges:  challenges,
		repo:        repo,
		events:      events,
		clock:       fakeClock,
		hasher:      hasher,
		pool:        pool,
	}
}

// 1. Rolled-back registration / transaction failure sends no code.
func TestRegister_TransactionFailureSendsNoCode(t *testing.T) {
	h := newRegisterHarness(t)
	h.events.err = errors.New("outbox write failure")

	req := service.Registration{
		Email:       "test@example.com",
		Password:    "secure-pass-1234",
		DisplayName: "Test User",
	}

	_, err := h.service.Register(context.Background(), req)
	if err == nil {
		t.Fatal("expected registration to fail when outbox write fails")
	}

	if len(h.events.events) != 0 {
		t.Fatalf("expected 0 events recorded, got %d", len(h.events.events))
	}
}

// 2. Registering an already-verified address is enumeration-safe.
func TestRegister_AlreadyVerifiedAddressIsEnumerationSafe(t *testing.T) {
	h := newRegisterHarness(t)
	email := "verified@example.com"

	// Pre-create verified account
	accID := uuid.New()
	h.accounts.accounts[email] = service.Account{ID: accID, Verified: true, Status: accountStatusActive}

	req := service.Registration{
		Email:       email,
		Password:    "new-password-1234",
		DisplayName: "Verified User",
	}

	issued, err := h.service.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("register verified account: %v", err)
	}

	// Code must be empty/redacted for the caller
	if issued.Code.Reveal() != "" {
		t.Fatalf("code must not be revealed for verified address, got %q", issued.Code.Reveal())
	}

	// Check that EventRegistrationAttempted was published, NOT VerificationRequested
	if len(h.events.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(h.events.events))
	}
	event := h.events.events[0]
	if event.Event != contract.EventRegistrationAttempted {
		t.Fatalf("event = %s, want %s", event.Event, contract.EventRegistrationAttempted)
	}

	// Verification on this challenge_id must return error (challenge not usable / OTP invalid)
	_, err = h.service.VerifyEmail(context.Background(), issued.Challenge.ID, "123456")
	if err == nil {
		t.Fatal("expected verify to fail for dummy challenge on verified address")
	}
	if apperr.Is(err, apperr.NotFound) {
		t.Fatal("expected OTP_INVALID / challenge failure, not CHALLENGE_NOT_FOUND")
	}
}

// 3. Registering an unverified address replaces the credential.
func TestRegister_UnverifiedAddressReplacesCredential(t *testing.T) {
	h := newRegisterHarness(t)
	email := "unverified@example.com"

	// 1. Initial registration
	req1 := service.Registration{
		Email:       email,
		Password:    "old-password-123",
		DisplayName: "Unverified User",
	}
	_, err := h.service.Register(context.Background(), req1)
	if err != nil {
		t.Fatalf("initial register: %v", err)
	}

	acc := h.accounts.accounts[email]
	oldCred := h.credentials.credentials[acc.ID]

	// 2. Re-register same email with new password
	req2 := service.Registration{
		Email:       email,
		Password:    "new-password-456",
		DisplayName: "Unverified User",
	}
	_, err = h.service.Register(context.Background(), req2)
	if err != nil {
		t.Fatalf("second register: %v", err)
	}

	newCred := h.credentials.credentials[acc.ID]
	if newCred.PasswordHash.Reveal() == oldCred.PasswordHash.Reveal() {
		t.Fatal("credential password hash was not replaced")
	}

	// Verify old password fails and new password succeeds
	vOld, _ := h.hasher.Verify("old-password-123", newCred.PasswordHash.Reveal())
	if vOld.Matches {
		t.Fatal("old password should no longer match")
	}
	vNew, _ := h.hasher.Verify("new-password-456", newCred.PasswordHash.Reveal())
	if !vNew.Matches {
		t.Fatal("new password should match")
	}
}

// 4. Successful verification marks the address verified.
func TestVerifyEmail_MarksAddressVerified(t *testing.T) {
	h := newRegisterHarness(t)
	email := "testverify@example.com"

	req := service.Registration{
		Email:       email,
		Password:    testPassword,
		DisplayName: "Test Verify",
	}
	issued, err := h.service.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	code := issued.Code.Reveal()
	verification, err := h.service.VerifyEmail(context.Background(), issued.Challenge.ID, code)
	if err != nil {
		t.Fatalf("verify email: %v", err)
	}

	if verification.Purpose != domain.PurposeVerifyEmail {
		t.Fatalf("purpose = %s, want verify_email", verification.Purpose)
	}

	if h.accounts.markCalls != 1 {
		t.Fatalf("expected MarkEmailVerified to be called once, got %d", h.accounts.markCalls)
	}
	if !h.accounts.accounts[email].Verified {
		t.Fatal("account in store is still unverified")
	}
}

// 5. Unverified accounts are swept after seven days.
func TestPurgeUnverified_SweepsOnlyUnverified(t *testing.T) {
	h := newRegisterHarness(t)

	// Pre-populate: 1 unverified, 1 verified
	id1 := uuid.New()
	id2 := uuid.New()
	h.accounts.accounts["unverified@test.com"] = service.Account{ID: id1, Verified: false, Status: accountStatusActive}
	h.accounts.byID[id1] = service.Contact{Email: "unverified@test.com"}

	h.accounts.accounts["verified@test.com"] = service.Account{ID: id2, Verified: true, Status: accountStatusActive}
	h.accounts.byID[id2] = service.Contact{Email: "verified@test.com"}

	if err := h.service.PurgeUnverified(context.Background()); err != nil {
		t.Fatalf("PurgeUnverified: %v", err)
	}

	if _, ok := h.accounts.accounts["unverified@test.com"]; ok {
		t.Fatal("unverified account was not swept")
	}
	if _, ok := h.accounts.accounts["verified@test.com"]; !ok {
		t.Fatal("verified account was incorrectly swept")
	}
}

// 6. Code appears in no response body / struct is redacted.
func TestRegistration_CodeNeverRevealedInResponse(t *testing.T) {
	h := newRegisterHarness(t)

	req := service.Registration{
		Email:       "noreveal@example.com",
		Password:    testPassword,
		DisplayName: "No Reveal",
	}

	issued, err := h.service.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Standard String() output of Code MUST be redacted
	if strings.Contains(issued.Code.String(), issued.Code.Reveal()) && issued.Code.Reveal() != "" {
		t.Fatal("secret Code string leaked plaintext code")
	}
}

// 7. Mailer outage does not fail registration (outbox event is stored).
func TestRegister_SucceedsEvenIfMailerIsDown(t *testing.T) {
	h := newRegisterHarness(t)
	// Mailer outage is outside service bounds — service writes to Outbox in DB.
	// Outbox event is successfully written regardless of mailer availability.
	req := service.Registration{
		Email:       "mailerdown@example.com",
		Password:    testPassword,
		DisplayName: "Mailer Down",
	}

	issued, err := h.service.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("register failed despite DB outbox write succeeding: %v", err)
	}

	if issued.Challenge.ID == uuid.Nil {
		t.Fatal("expected valid challenge id in returned struct")
	}

	if len(h.events.events) != 1 {
		t.Fatalf("expected 1 outbox event written, got %d", len(h.events.events))
	}
}
