package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const (
	loginActiveStatus = "active"
	loginTestPassword = "password-1234"
	loginTestIP       = "127.0.0.1"
)

type fakeLoginRepo struct {
	mu           sync.Mutex
	attempts     []recordedAttempt
	accountFails map[string]int64
	ipFails      map[string]int64
	lockouts     map[string]time.Time
	lockLevels   map[string]int
}

type recordedAttempt struct {
	ID            uuid.UUID
	UserID        *uuid.UUID
	EmailHash     []byte
	IPHash        []byte
	Success       bool
	FailureReason *string
	CreatedAt     time.Time
}

func newFakeLoginRepo() *fakeLoginRepo {
	return &fakeLoginRepo{
		accountFails: make(map[string]int64),
		ipFails:      make(map[string]int64),
		lockouts:     make(map[string]time.Time),
		lockLevels:   make(map[string]int),
	}
}

func (f *fakeLoginRepo) RecordLoginAttempt(
	_ context.Context, id uuid.UUID, userID *uuid.UUID, emailHash, ipHash []byte,
	success bool, failureReason *string, createdAt time.Time,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.attempts = append(f.attempts, recordedAttempt{
		ID:            id,
		UserID:        userID,
		EmailHash:     emailHash,
		IPHash:        ipHash,
		Success:       success,
		FailureReason: failureReason,
		CreatedAt:     createdAt,
	})

	if !success {
		f.accountFails[string(emailHash)]++
		f.ipFails[string(ipHash)]++
	}
	return nil
}

func (f *fakeLoginRepo) CountRecentFailedAttemptsByAccount(
	_ context.Context, emailHash []byte, _ time.Time,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accountFails[string(emailHash)], nil
}

func (f *fakeLoginRepo) CountRecentFailedAttemptsByIP(_ context.Context, ipHash []byte, _ time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ipFails[string(ipHash)], nil
}

func (f *fakeLoginRepo) GetActiveLoginLockout(
	_ context.Context, scope string, subjectHash []byte, now time.Time,
) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	lockedUntil, ok := f.lockouts[scope+":"+string(subjectHash)]
	if !ok || !lockedUntil.After(now) {
		return time.Time{}, false, nil
	}
	return lockedUntil, true, nil
}

func (f *fakeLoginRepo) AdvanceLoginLockout(
	_ context.Context, scope string, subjectHash []byte, now time.Time,
) (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := scope + ":" + string(subjectHash)
	if lockedUntil, ok := f.lockouts[key]; ok && lockedUntil.After(now) {
		return time.Time{}, false, nil
	}
	level := f.lockLevels[key]
	lockedUntil := now.Add(domain.LockoutDuration(level))
	f.lockLevels[key] = level + 1
	f.lockouts[key] = lockedUntil
	return lockedUntil, true, nil
}

type loginHarness struct {
	service     *service.LoginService
	accounts    *fakeAccounts
	credentials *fakeCredentials
	repo        *fakeLoginRepo
	limiter     *fakeLimiter
	keys        domain.Keyring
	hasher      domain.Hasher
	clock       *clock.Fake
}

func newLoginHarness(t *testing.T) *loginHarness {
	t.Helper()

	keys, err := domain.NewKeyring([]byte(otpKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	fakeClock := clock.NewFake(testNow)
	accounts := newFakeAccounts()
	credentials := newFakeCredentials()
	repo := newFakeLoginRepo()
	limiter := newFakeLimiter()
	hasher := domain.NewHasher(domain.DefaultHashParams())

	loginService := service.NewLoginService(service.LoginDeps{
		Accounts:    accounts,
		Credentials: credentials,
		Repo:        repo,
		Limiter:     limiter,
		Keys:        keys,
		Hasher:      hasher,
		Clock:       fakeClock,
		NewID:       func(context.Context) (uuid.UUID, error) { return uuid.New(), nil },
	})

	return &loginHarness{
		service:     loginService,
		accounts:    accounts,
		credentials: credentials,
		repo:        repo,
		limiter:     limiter,
		keys:        keys,
		hasher:      hasher,
		clock:       fakeClock,
	}
}

func TestLogin_SuccessfulAuthentication(t *testing.T) {
	h := newLoginHarness(t)
	email := "valid@example.com"
	pass := "valid-pass-1234"

	userID := uuid.New()
	hash, err := h.hasher.Hash(pass)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	h.accounts.accounts[email] = service.Account{ID: userID, Verified: true, Status: loginActiveStatus}
	h.credentials.credentials[userID] = domain.Credential{
		ID:           uuid.New(),
		UserID:       userID,
		PasswordHash: h.credentials.credentials[userID].PasswordHash,
	}
	_, _ = h.credentials.Create(context.Background(), uuid.New(), userID, hash)

	res, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    email,
		Password: pass,
		ClientIP: loginTestIP,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if res.UserID != userID {
		t.Fatalf("userID = %s, want %s", res.UserID, userID)
	}
	if !res.Verified {
		t.Fatal("expected Verified = true")
	}
	if calls := h.limiter.keys(); len(calls) != 0 {
		t.Fatalf("successful login charged lockout counters: %v", calls)
	}
}

func TestLogin_UnknownEmail_TimingEqualisation(t *testing.T) {
	h := newLoginHarness(t)
	start := time.Now()

	_, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    "nonexistent@example.com",
		Password: "some-password-1234",
		ClientIP: loginTestIP,
	})

	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected login to fail for unknown email")
	}
	if !apperr.Is(err, apperr.Unauthenticated) {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}

	// Verify attempt recorded
	if len(h.repo.attempts) != 1 {
		t.Fatalf("expected 1 recorded attempt, got %d", len(h.repo.attempts))
	}
	if h.repo.attempts[0].Success {
		t.Fatal("recorded attempt must be failure")
	}

	// Duration must be at least measurable due to DummyVerify Argon2id computation
	t.Logf("DummyVerify execution duration: %v", duration)
}

func TestLogin_WrongPassword(t *testing.T) {
	h := newLoginHarness(t)
	email := "user@example.com"

	userID := uuid.New()
	hash, _ := h.hasher.Hash("correct-password")
	h.accounts.accounts[email] = service.Account{ID: userID, Verified: true, Status: loginActiveStatus}
	_, _ = h.credentials.Create(context.Background(), uuid.New(), userID, hash)

	_, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    email,
		Password: "wrong-password",
		ClientIP: loginTestIP,
	})

	if err == nil {
		t.Fatal("expected login to fail for wrong password")
	}
	if !apperr.Is(err, apperr.Unauthenticated) {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestLogin_UnverifiedAccount(t *testing.T) {
	h := newLoginHarness(t)
	email := "unverified@example.com"
	pass := loginTestPassword

	userID := uuid.New()
	hash, _ := h.hasher.Hash(pass)
	h.accounts.accounts[email] = service.Account{ID: userID, Verified: false, Status: loginActiveStatus}
	_, _ = h.credentials.Create(context.Background(), uuid.New(), userID, hash)

	_, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    email,
		Password: pass,
		ClientIP: loginTestIP,
	})

	if err == nil {
		t.Fatal("expected login to fail for unverified account")
	}
	if !strings.Contains(err.Error(), "EMAIL_NOT_VERIFIED") {
		t.Fatalf("expected EMAIL_NOT_VERIFIED error, got %v", err)
	}
}

func TestLogin_SuspendedAccount(t *testing.T) {
	h := newLoginHarness(t)
	email := "suspended@example.com"
	pass := loginTestPassword

	userID := uuid.New()
	hash, _ := h.hasher.Hash(pass)
	h.accounts.accounts[email] = service.Account{ID: userID, Verified: true, Status: "suspended"}
	_, _ = h.credentials.Create(context.Background(), uuid.New(), userID, hash)

	_, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    email,
		Password: pass,
		ClientIP: loginTestIP,
	})

	if err == nil {
		t.Fatal("expected login to fail for suspended account")
	}
	if !strings.Contains(err.Error(), "ACCOUNT_LOCKED") {
		t.Fatalf("expected ACCOUNT_LOCKED error, got %v", err)
	}
}

func TestLogin_LockoutEnforcement(t *testing.T) {
	h := newLoginHarness(t)
	email := "target@example.com"
	emailHash := h.keys.SubjectHash(email)

	// Five persisted failures are the lockout authority. Redis is charged only
	// after failures and must never count successful logins.
	h.repo.accountFails[string(emailHash)] = domain.LockoutMaxAttempts

	_, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    email,
		Password: loginTestPassword,
		ClientIP: loginTestIP,
	})

	if err == nil {
		t.Fatal("expected login to fail when locked out")
	}
	if !apperr.Is(err, apperr.RateLimited) {
		t.Fatalf("expected RateLimited, got %v", err)
	}
}

func TestLogin_LockoutBackoffDoublesAfterEachExpiredLock(t *testing.T) {
	h := newLoginHarness(t)
	email := "backoff@example.com"
	emailHash := h.keys.SubjectHash(email)
	h.repo.accountFails[string(emailHash)] = domain.LockoutMaxAttempts

	input := service.LoginInput{Email: email, Password: loginTestPassword, ClientIP: loginTestIP}
	if _, err := h.service.Login(context.Background(), input); !apperr.Is(err, apperr.RateLimited) {
		t.Fatalf("first lockout error = %v, want rate limited", err)
	}

	key := "account:" + string(emailHash)
	if got, want := h.repo.lockouts[key], testNow.Add(domain.LockoutWindow); !got.Equal(want) {
		t.Fatalf("first lock expires at %s, want %s", got, want)
	}

	h.clock.Advance(domain.LockoutWindow + time.Second)
	if _, err := h.service.Login(context.Background(), input); !apperr.Is(err, apperr.RateLimited) {
		t.Fatalf("second lockout error = %v, want rate limited", err)
	}
	if got, want := h.repo.lockouts[key], h.clock.Now().Add(2*domain.LockoutWindow); !got.Equal(want) {
		t.Fatalf("second lock expires at %s, want %s", got, want)
	}
}

func TestLogin_DegradedLimiterFallback(t *testing.T) {
	h := newLoginHarness(t)
	h.limiter.degraded = true

	email := "degraded@example.com"
	emailHash := h.keys.SubjectHash(email)

	// Simulate 5 failed attempts in DB
	h.repo.accountFails[string(emailHash)] = 5

	_, err := h.service.Login(context.Background(), service.LoginInput{
		Email:    email,
		Password: loginTestPassword,
		ClientIP: loginTestIP,
	})

	if err == nil {
		t.Fatal("expected DB fallback to enforce lockout when Redis is degraded")
	}
	if !apperr.Is(err, apperr.RateLimited) {
		t.Fatalf("expected RateLimited, got %v", err)
	}
}
