package service_test

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/service"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/secret"
)

// fakeRepository is an in-memory stand-in for the challenge table.
//
// Every guard in it mirrors a WHERE clause in `db/queries/auth/challenges.sql`,
// clause for clause, and the comments say which. That correspondence is the
// only thing that makes these tests meaningful — a fake that is more permissive
// than the SQL would pass tests the real system fails. The integration suite
// checks the same rules against Postgres, which is what catches the fake
// drifting from the query it imitates.
type fakeRepository struct {
	mu         sync.Mutex
	challenges map[uuid.UUID]domain.Challenge

	// createErr, if set, is returned by CreateChallenge.
	createErr error
	// consumeCalls counts guarded consumption attempts, so a test can prove a
	// wrong code never reaches the write.
	consumeCalls int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{challenges: make(map[uuid.UUID]domain.Challenge)}
}

func (f *fakeRepository) CreateChallenge(_ context.Context, input domain.NewChallengeInput) (
	domain.Challenge, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return domain.Challenge{}, f.createErr
	}
	challenge := domain.Challenge{
		ID:          input.ID,
		Purpose:     input.Purpose,
		SubjectHash: input.SubjectHash,
		CodeHash:    input.CodeHash,
		Attempts:    0,
		MaxAttempts: input.MaxAttempts,
		ExpiresAt:   input.ExpiresAt,
		UserID:      input.UserID,
		LastSentAt:  input.Now,
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}
	f.challenges[input.ID] = challenge
	return challenge, nil
}

func (f *fakeRepository) GetChallenge(_ context.Context, id uuid.UUID) (domain.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	challenge, ok := f.challenges[id]
	if !ok {
		return domain.Challenge{}, domain.ErrChallengeNotFound
	}
	return challenge, nil
}

// ConsumeChallenge mirrors:
//
//	WHERE id = @id AND code_hash = @code_hash AND consumed_at IS NULL
//	  AND attempts < max_attempts AND expires_at > @now
func (f *fakeRepository) ConsumeChallenge(_ context.Context, id uuid.UUID, codeHash []byte, now time.Time) (
	domain.Challenge, bool, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.consumeCalls++
	challenge, ok := f.challenges[id]
	if !ok || !domain.EqualHash(challenge.CodeHash, codeHash) ||
		challenge.Consumed() || challenge.Burned() || challenge.Expired(now) {
		return domain.Challenge{}, false, nil
	}

	consumedAt := now
	challenge.ConsumedAt = &consumedAt
	challenge.UpdatedAt = now
	f.challenges[id] = challenge
	return challenge, true, nil
}

// RecordFailedAttempt mirrors:
//
//	SET attempts = attempts + 1
//	WHERE id = @id AND consumed_at IS NULL AND attempts < max_attempts AND expires_at > @now
func (f *fakeRepository) RecordFailedAttempt(_ context.Context, id uuid.UUID, now time.Time) (
	domain.Challenge, bool, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	challenge, ok := f.challenges[id]
	if !ok || challenge.Consumed() || challenge.Burned() || challenge.Expired(now) {
		return domain.Challenge{}, false, nil
	}

	challenge.Attempts++
	challenge.UpdatedAt = now
	f.challenges[id] = challenge
	return challenge, true, nil
}

// ResendChallenge mirrors:
//
//	SET code_hash = @code_hash, attempts = 0, last_sent_at = @now
//	WHERE id = @id AND consumed_at IS NULL AND attempts < max_attempts
//	  AND expires_at > @now AND last_sent_at <= @resend_allowed_from
//
// expires_at is deliberately absent from the SET, which is BR-AUTH-13.
func (f *fakeRepository) ResendChallenge(
	_ context.Context, id uuid.UUID, codeHash []byte, resendAllowedFrom, now time.Time,
) (domain.Challenge, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	challenge, ok := f.challenges[id]
	if !ok || challenge.Consumed() || challenge.Burned() || challenge.Expired(now) ||
		challenge.LastSentAt.After(resendAllowedFrom) {
		return domain.Challenge{}, false, nil
	}

	challenge.CodeHash = codeHash
	challenge.Attempts = 0
	challenge.LastSentAt = now
	challenge.UpdatedAt = now
	f.challenges[id] = challenge
	return challenge, true, nil
}

// WithTx returns the same store. The fake has no transactions to model — what
// the integration suite proves is that a rollback removes the row, and that
// needs a real one.
func (f *fakeRepository) WithTx(pgx.Tx) service.Repository { return f }

// fakeLimiter records what it was asked and answers from a scripted verdict.
type fakeLimiter struct {
	mu sync.Mutex
	// denied is the set of keys that are over quota.
	denied map[string]bool
	// calls records every evaluation, in order, so a test can assert which
	// limiters ran and with what quota.
	calls []limiterCall
	// err, if set, is returned instead of a verdict.
	err error
	// degraded makes every verdict a degraded allow, which is what a
	// RedisLimiter returns when Redis is unreachable.
	degraded bool
}

type limiterCall struct {
	key    string
	limit  int
	window time.Duration
}

func newFakeLimiter() *fakeLimiter { return &fakeLimiter{denied: make(map[string]bool)} }

func (f *fakeLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (
	cache.LimitResult, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, limiterCall{key: key, limit: limit, window: window})
	if f.err != nil {
		return cache.LimitResult{}, f.err
	}
	if f.degraded {
		return cache.LimitResult{Allowed: true, Remaining: limit, Degraded: true}, nil
	}
	if f.denied[key] {
		return cache.LimitResult{Allowed: false, ResetIn: window}, nil
	}
	return cache.LimitResult{Allowed: true, Remaining: limit - 1, ResetIn: window}, nil
}

func (f *fakeLimiter) deny(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.denied[key] = true
}

func (f *fakeLimiter) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	names := make([]string, 0, len(f.calls))
	for _, call := range f.calls {
		names = append(names, call.key)
	}
	return names
}

// fakeSessions opens a session without a database.
//
// It mints a real access token — the two suites that use it say why a stub
// would be worse — and a real refresh token, and it stores neither. Rotation is
// the part that needs Postgres to mean anything, and it has its own integration
// suite; what login and verification need from a session here is that the
// caller is handed one they could actually present.
type fakeSessions struct {
	tokens *service.TokenService
	clock  clock.Clock
	newID  func(context.Context) (uuid.UUID, error)
}

func (f fakeSessions) Start(ctx context.Context, input service.StartInput) (service.SignedIn, error) {
	sessionID, err := f.newID(ctx)
	if err != nil {
		return service.SignedIn{}, err
	}
	session, err := f.tokens.Issue(ctx, input.UserID, sessionID)
	if err != nil {
		return service.SignedIn{}, err
	}
	raw, _, err := domain.NewRefreshToken(nil)
	if err != nil {
		return service.SignedIn{}, err
	}
	return service.SignedIn{
		Session:          session,
		RefreshToken:     secret.New(raw),
		RefreshExpiresAt: f.clock.Now().Add(service.DefaultRefreshTTL),
	}, nil
}
