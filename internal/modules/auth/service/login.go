package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

const invalidCredentialsReason = "invalid_credentials"

// LoginInput is what a caller presents on POST /auth/login.
type LoginInput struct {
	Email          string
	Password       string
	RememberDevice bool
	DeviceID       string
	ClientIP       string
}

// LoginResult is returned on successful authentication.
type LoginResult struct {
	UserID   uuid.UUID
	Verified bool
	Rehashed bool
}

// LoginRepo is the repository interface required by LoginService.
type LoginRepo interface {
	RecordLoginAttempt(
		ctx context.Context, id uuid.UUID, userID *uuid.UUID, emailHash, ipHash []byte,
		success bool, failureReason *string, createdAt time.Time,
	) error
	CountRecentFailedAttemptsByAccount(ctx context.Context, emailHash []byte, since time.Time) (int64, error)
	CountRecentFailedAttemptsByIP(ctx context.Context, ipHash []byte, since time.Time) (int64, error)
	GetActiveLoginLockout(ctx context.Context, scope string, subjectHash []byte, now time.Time) (time.Time, bool, error)
	AdvanceLoginLockout(ctx context.Context, scope string, subjectHash []byte, now time.Time) (time.Time, bool, error)
}

// LoginDeps groups dependencies for LoginService.
type LoginDeps struct {
	Accounts    Accounts
	Credentials Credentials
	Repo        LoginRepo
	Limiter     cache.Limiter
	Keys        domain.Keyring
	Hasher      domain.Hasher
	Clock       clock.Clock
	NewID       func(ctx context.Context) (uuid.UUID, error)
}

// LoginService handles authentication, lockout enforcement, and timing equalisation.
type LoginService struct {
	accounts    Accounts
	credentials Credentials
	repo        LoginRepo
	limiter     cache.Limiter
	keys        domain.Keyring
	hasher      domain.Hasher
	clock       clock.Clock
	newID       func(ctx context.Context) (uuid.UUID, error)
}

// NewLoginService constructs a LoginService.
func NewLoginService(deps LoginDeps) *LoginService {
	return &LoginService{
		accounts:    deps.Accounts,
		credentials: deps.Credentials,
		repo:        deps.Repo,
		limiter:     deps.Limiter,
		keys:        deps.Keys,
		hasher:      deps.Hasher,
		clock:       deps.Clock,
		newID:       deps.NewID,
	}
}

// Login authenticates a user by email and password, enforcing timing equalisation and lockouts.
func (s *LoginService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	now := s.clock.Now().UTC()
	email := strings.ToLower(strings.TrimSpace(input.Email))

	emailHash := s.keys.SubjectHash(email)
	var ipHash []byte
	if input.ClientIP != "" {
		ipHash = s.keys.SubjectHash(input.ClientIP)
	}

	// 1. Lockout check. Login attempts in Postgres are the authoritative
	// record: cache.Limiter.Allow mutates its counter, so using it as a
	// preflight check would count successful logins as failures.
	if err := s.checkLockout(ctx, emailHash, ipHash, now); err != nil {
		return LoginResult{}, err
	}

	// 2. Lookup account
	acc, found, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		return LoginResult{}, fmt.Errorf("find account by email: %w", err)
	}

	if !found {
		// Unknown email: execute DummyVerify to equalise CPU timing (BR-AUTH-02)
		_, _ = s.hasher.DummyVerify(input.Password)
		s.recordFailed(ctx, nil, emailHash, ipHash, invalidCredentialsReason, now)
		s.chargeLockout(ctx, emailHash, ipHash)
		return LoginResult{}, apperr.New(apperr.Unauthenticated, "INVALID_CREDENTIALS", "Invalid email or password")
	}

	// 3. Get credential for existing user
	cred, err := s.credentials.GetByUserID(ctx, acc.ID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			_, _ = s.hasher.DummyVerify(input.Password)
			s.recordFailed(ctx, &acc.ID, emailHash, ipHash, invalidCredentialsReason, now)
			s.chargeLockout(ctx, emailHash, ipHash)
			return LoginResult{}, apperr.New(apperr.Unauthenticated, "INVALID_CREDENTIALS", "Invalid email or password")
		}
		return LoginResult{}, fmt.Errorf("get credential: %w", err)
	}

	// 4. Constant-time Argon2id verification
	verification, err := s.hasher.Verify(input.Password, cred.PasswordHash.Reveal())
	if err != nil || !verification.Matches {
		s.recordFailed(ctx, &acc.ID, emailHash, ipHash, invalidCredentialsReason, now)
		s.chargeLockout(ctx, emailHash, ipHash)
		return LoginResult{}, apperr.New(apperr.Unauthenticated, "INVALID_CREDENTIALS", "Invalid email or password")
	}

	// 5. Check account status (suspended / unverified)
	if acc.Status == "suspended" {
		s.recordFailed(ctx, &acc.ID, emailHash, ipHash, "account_suspended", now)
		return LoginResult{}, apperr.New(apperr.Unauthenticated, "ACCOUNT_LOCKED", "Account is suspended")
	}

	if !acc.Verified {
		s.recordFailed(ctx, &acc.ID, emailHash, ipHash, "email_not_verified", now)
		return LoginResult{}, apperr.New(apperr.Unauthenticated, "EMAIL_NOT_VERIFIED", "Email is not verified")
	}

	// 6. Transparent Argon2id parameter upgrade if hash parameters changed
	rehashed := false
	if verification.NeedsRehash {
		if newHash, err := s.hasher.Hash(input.Password); err == nil {
			if _, replaceErr := s.credentials.ReplaceHash(ctx, acc.ID, newHash); replaceErr == nil {
				rehashed = true
				slog.InfoContext(ctx, "upgraded argon2id password hash parameters on login", "module", "auth", "user_id", acc.ID)
			}
		}
	}

	// 7. Successful login record
	s.recordSuccess(ctx, acc.ID, emailHash, ipHash, now)

	return LoginResult{
		UserID:   acc.ID,
		Verified: acc.Verified,
		Rehashed: rehashed,
	}, nil
}

func (s *LoginService) checkLockout(ctx context.Context, emailHash, ipHash []byte, now time.Time) error {
	if locked, err := s.activeLockout(ctx, "account", emailHash, now); err == nil && locked {
		return lockedError()
	}
	if len(ipHash) > 0 {
		if locked, err := s.activeLockout(ctx, "ip", ipHash, now); err == nil && locked {
			return lockedError()
		}
	}

	cutoff := now.Add(-domain.LockoutWindow)
	accountFailures, accountErr := s.repo.CountRecentFailedAttemptsByAccount(ctx, emailHash, cutoff)
	if accountErr == nil && accountFailures >= domain.LockoutMaxAttempts {
		return s.startLockout(ctx, "account", emailHash, now)
	}

	// An unavailable attempt store must not turn an outage into a login outage.
	// When ClientIP is unknown, skip its bucket rather than charging every such
	// request to one shared value.
	if len(ipHash) > 0 {
		ipFailures, ipErr := s.repo.CountRecentFailedAttemptsByIP(ctx, ipHash, cutoff)
		if ipErr == nil && ipFailures >= domain.LockoutMaxAttempts {
			return s.startLockout(ctx, "ip", ipHash, now)
		}
	}

	return nil
}

func (s *LoginService) activeLockout(
	ctx context.Context, scope string, subjectHash []byte, now time.Time,
) (bool, error) {
	_, locked, err := s.repo.GetActiveLoginLockout(ctx, scope, subjectHash, now)
	return locked, err
}

func (s *LoginService) startLockout(ctx context.Context, scope string, subjectHash []byte, now time.Time) error {
	_, _, err := s.repo.AdvanceLoginLockout(ctx, scope, subjectHash, now)
	if err != nil {
		// Rate limiting fails open when its backing store is unavailable.
		return nil
	}
	return lockedError()
}

func (s *LoginService) chargeLockout(ctx context.Context, emailHash, ipHash []byte) {
	// This is deliberately reached only after a failed authentication. Allow
	// increments a counter, so calling it from checkLockout would eventually
	// lock users who entered the correct password.
	accountKey := domain.LockoutAccountKey(emailHash)
	_, _ = s.limiter.Allow(ctx, accountKey, domain.LockoutMaxAttempts, domain.LockoutWindow)
	if len(ipHash) > 0 {
		ipKey := domain.LockoutIPKey(ipHash)
		_, _ = s.limiter.Allow(ctx, ipKey, domain.LockoutMaxAttempts, domain.LockoutWindow)
	}
}

func lockedError() error {
	return apperr.New(apperr.RateLimited, "ACCOUNT_LOCKED", "Too many failed login attempts. Please try again later.")
}

func (s *LoginService) recordFailed(
	ctx context.Context, userID *uuid.UUID, emailHash, ipHash []byte,
	reason string, now time.Time,
) {
	id, err := s.newID(ctx)
	if err != nil {
		id = uuid.New()
	}
	_ = s.repo.RecordLoginAttempt(ctx, id, userID, emailHash, ipHash, false, &reason, now)
}

func (s *LoginService) recordSuccess(ctx context.Context, userID uuid.UUID, emailHash, ipHash []byte, now time.Time) {
	id, err := s.newID(ctx)
	if err != nil {
		id = uuid.New()
	}
	_ = s.repo.RecordLoginAttempt(ctx, id, &userID, emailHash, ipHash, true, nil, now)
}
