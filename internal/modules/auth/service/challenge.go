// Package service holds the auth use cases. So far that is the challenge
// subsystem: issuing, verifying, resending and burning the short-lived one-time
// codes that registration, password reset and step-up verification all use.
//
// ADR-0021 chose one generic implementation over three purpose-specific ones.
// The reason is visible in this file: constant-time comparison, the attempt cap
// and the issuance limiters are written once, so there is one place to get them
// right and one place to review.
package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/platform/cache"
	"github.com/fluentra/fluentra/internal/shared/clock"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

// Repository is the persistence surface this service needs, declared by the
// consumer so the rules can be exercised without a database.
type Repository interface {
	CreateChallenge(ctx context.Context, challenge domain.NewChallengeInput) (domain.Challenge, error)
	GetChallenge(ctx context.Context, id uuid.UUID) (domain.Challenge, error)
	ConsumeChallenge(ctx context.Context, id uuid.UUID, codeHash []byte, now time.Time) (domain.Challenge, bool, error)
	RecordFailedAttempt(ctx context.Context, id uuid.UUID, now time.Time) (domain.Challenge, bool, error)
	ResendChallenge(ctx context.Context, id uuid.UUID, codeHash []byte, resendAllowedFrom, now time.Time) (
		domain.Challenge, bool, error)

	WithTx(tx pgx.Tx) Repository
}

// IDGenerator produces the identifier a new challenge is written with. It is a
// dependency rather than a direct call into shared/id so a test can make ids
// deterministic without also having to control the clock.
type IDGenerator func(ctx context.Context) (uuid.UUID, error)

// Config is the tunable half of the subsystem. Every field maps to an OTP_* key
// in `.env.example`; DefaultConfig carries the same values, so a service built
// before the composition root reads configuration still behaves as documented.
type Config struct {
	CodeLength              int
	TTL                     time.Duration
	MaxAttempts             int
	ResendCooldown          time.Duration
	IssuesPerSubjectPerHour int
	IssuesPerIPPerHour      int
}

// DefaultConfig mirrors the OTP_* block of `.env.example`.
func DefaultConfig() Config {
	return Config{
		CodeLength:              domain.CodeLength,
		TTL:                     domain.ChallengeTTL,
		MaxAttempts:             domain.MaxAttempts,
		ResendCooldown:          domain.ResendCooldown,
		IssuesPerSubjectPerHour: 3,
		IssuesPerIPPerHour:      20,
	}
}

// withDefaults fills anything a caller left zero, so a partially-populated
// Config cannot produce a challenge with no expiry or no attempt cap.
func (c Config) withDefaults() Config {
	defaults := DefaultConfig()
	if c.CodeLength <= 0 {
		c.CodeLength = defaults.CodeLength
	}
	if c.TTL <= 0 {
		c.TTL = defaults.TTL
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaults.MaxAttempts
	}
	if c.ResendCooldown <= 0 {
		c.ResendCooldown = defaults.ResendCooldown
	}
	if c.IssuesPerSubjectPerHour <= 0 {
		c.IssuesPerSubjectPerHour = defaults.IssuesPerSubjectPerHour
	}
	if c.IssuesPerIPPerHour <= 0 {
		c.IssuesPerIPPerHour = defaults.IssuesPerIPPerHour
	}
	return c
}

// ChallengeService issues, verifies and resends one-time codes.
type ChallengeService struct {
	repo    Repository
	limiter cache.Limiter
	keys    domain.Keyring
	clock   clock.Clock
	ids     IDGenerator
	config  Config
	env     string
}

// ChallengeDeps are the service's collaborators.
type ChallengeDeps struct {
	Repo    Repository
	Limiter cache.Limiter
	Keys    domain.Keyring
	Clock   clock.Clock
	NewID   IDGenerator
	Config  Config
	// Env namespaces the limiter keys, so a staging deploy pointed at a shared
	// Redis cannot spend a production learner's issuance budget.
	Env string
}

// NewChallengeService creates the challenge service.
func NewChallengeService(deps ChallengeDeps) *ChallengeService {
	return &ChallengeService{
		repo:    deps.Repo,
		limiter: deps.Limiter,
		keys:    deps.Keys,
		clock:   deps.Clock,
		ids:     deps.NewID,
		config:  deps.Config.withDefaults(),
		env:     deps.Env,
	}
}

// IssueRequest names what a challenge is for and who it is for.
type IssueRequest struct {
	Purpose Purpose
	// Subject is the email address, or whatever else identifies the recipient
	// for this purpose. It is hashed before it touches the database and it is
	// never logged.
	Subject string
	// UserID is the account the challenge belongs to, when one exists. Verify
	// needs it: the row stores only a keyed digest of the subject, which is
	// irreversible by design and so can identify nothing on its own.
	UserID *uuid.UUID
}

// Purpose is re-exported so a caller in another layer names the purpose without
// reaching past this package into the domain.
type Purpose = domain.Purpose

// Issued is a challenge and the code that belongs to it.
//
// The code is returned in process, to the caller, exactly once. It goes into the
// outbox payload that the mailer renders (P2.2) and nowhere else — not into a
// response body, not into a log, not into a span attribute (BR-AUTH-10). It is
// wrapped so that none of those can happen by accident.
type Issued struct {
	Challenge domain.Challenge
	Code      domain.Code
}

// Issue creates a challenge and returns it with its code.
func (s *ChallengeService) Issue(ctx context.Context, request IssueRequest) (Issued, error) {
	return s.issue(ctx, s.repo, request)
}

// IssueIn creates a challenge inside tx.
//
// Registration needs this: the account, the credential, the challenge and the
// outbox row that sends the code are one transaction, so that a rolled-back
// registration cannot send a code for an account that does not exist (rule L4,
// FLOW.md). The limiters are still consumed outside the transaction, because
// Redis has no notion of a rollback — a registration that fails after this
// point has spent one of the subject's three hourly issuances, which errs
// toward stricter and is the right direction for a limiter to err in.
func (s *ChallengeService) IssueIn(ctx context.Context, tx pgx.Tx, request IssueRequest) (Issued, error) {
	return s.issue(ctx, s.repo.WithTx(tx), request)
}

func (s *ChallengeService) issue(ctx context.Context, repo Repository, request IssueRequest) (Issued, error) {
	if !request.Purpose.Valid() {
		return Issued{}, fmt.Errorf("issue challenge: unknown purpose %q", request.Purpose)
	}
	if request.Subject == "" {
		return Issued{}, fmt.Errorf("issue challenge: empty subject")
	}

	subjectHash := s.keys.SubjectHash(request.Subject)
	if err := s.checkIssuanceLimits(ctx, subjectHash); err != nil {
		return Issued{}, err
	}

	challengeID, err := s.ids(ctx)
	if err != nil {
		return Issued{}, fmt.Errorf("issue challenge: %w", err)
	}
	code, err := domain.NewCode(s.config.CodeLength)
	if err != nil {
		return Issued{}, fmt.Errorf("issue challenge: %w", err)
	}

	now := s.clock.Now()
	challenge, err := repo.CreateChallenge(ctx, domain.NewChallengeInput{
		ID:          challengeID,
		Purpose:     request.Purpose,
		SubjectHash: subjectHash,
		// The id is bound into the digest, which is why the id had to be drawn
		// before the code could be hashed.
		CodeHash:    s.keys.CodeHash(challengeID, code.Reveal()),
		MaxAttempts: s.config.MaxAttempts,
		ExpiresAt:   now.Add(s.config.TTL),
		UserID:      request.UserID,
		Now:         now,
	})
	if err != nil {
		return Issued{}, err
	}

	// challenge_id, purpose and the expiry are safe to log; they are what the
	// client already holds. The subject and the code are not, and neither
	// appears here or anywhere below.
	slog.InfoContext(ctx, "otp challenge issued",
		"module", "auth", "op", "Issue",
		"challenge_id", challenge.ID.String(),
		"purpose", challenge.Purpose.String(),
		"expires_at", challenge.ExpiresAt)

	return Issued{Challenge: challenge, Code: code}, nil
}

// Verify checks code against the challenge and consumes it on success.
//
// The order of operations is the security of this function:
//
//  1. Load the challenge. An unknown id is a 404 and costs nothing.
//  2. Reject it if it is already consumed, burned or expired — before any
//     comparison, so a spent challenge cannot be used as a free oracle.
//  3. Compare in constant time (domain.EqualHash, crypto/subtle).
//  4. On a match, consume it with a guarded UPDATE. If that matches no row,
//     something else consumed it first and this caller loses the race.
//  5. On a mismatch, charge one attempt and report how many are left.
func (s *ChallengeService) Verify(ctx context.Context, challengeID uuid.UUID, code string) (domain.Challenge, error) {
	now := s.clock.Now()

	challenge, err := s.repo.GetChallenge(ctx, challengeID)
	if err != nil {
		return domain.Challenge{}, err
	}
	if err := challenge.Usable(now); err != nil {
		return domain.Challenge{}, err
	}

	// A wrong shape is still a wrong code: it charges an attempt. Skipping the
	// charge would make "submit one character" a free way to keep a challenge
	// alive for its full ten minutes without spending any of its budget.
	if !domain.ValidCodeShape(code, s.config.CodeLength) {
		return domain.Challenge{}, s.chargeFailedAttempt(ctx, challenge, now)
	}

	if !domain.EqualHash(challenge.CodeHash, s.keys.CodeHash(challengeID, code)) {
		return domain.Challenge{}, s.chargeFailedAttempt(ctx, challenge, now)
	}

	consumed, ok, err := s.repo.ConsumeChallenge(ctx, challengeID, challenge.CodeHash, now)
	if err != nil {
		return domain.Challenge{}, err
	}
	if !ok {
		// The code was right when it was read and the row would not take the
		// write. Something consumed it, burned it or resent it in between. Re-read
		// so the caller is told which, rather than being handed a generic failure.
		return domain.Challenge{}, s.explainLostRace(ctx, challengeID, now)
	}

	slog.InfoContext(ctx, "otp challenge verified",
		"module", "auth", "op", "Verify",
		"challenge_id", consumed.ID.String(), "purpose", consumed.Purpose.String())

	return consumed, nil
}

// chargeFailedAttempt records the wrong guess and returns what the caller should
// report: OTP_INVALID with the remaining count, or OTP_ATTEMPTS_EXCEEDED once
// the budget is spent.
func (s *ChallengeService) chargeFailedAttempt(
	ctx context.Context, challenge domain.Challenge, now time.Time,
) error {
	charged, ok, err := s.repo.RecordFailedAttempt(ctx, challenge.ID, now)
	if err != nil {
		return err
	}
	if !ok {
		// No attempt was available to charge, so the challenge became unusable
		// between the read and the write.
		return s.explainLostRace(ctx, challenge.ID, now)
	}

	if charged.Burned() {
		slog.WarnContext(ctx, "otp challenge burned after too many attempts",
			"module", "auth", "op", "Verify",
			"challenge_id", charged.ID.String(), "purpose", charged.Purpose.String(),
			"attempts", charged.Attempts)
		return domain.ErrChallengeAttemptsExceeded
	}
	return domain.ErrChallengeInvalidCode.WithMeta("attempts_remaining", charged.AttemptsRemaining())
}

// explainLostRace re-reads a challenge whose guarded write matched nothing and
// turns its current state into the error that describes it. A challenge that has
// vanished entirely is reported as not found.
func (s *ChallengeService) explainLostRace(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	current, err := s.repo.GetChallenge(ctx, challengeID)
	if err != nil {
		return err
	}
	if reason := current.Usable(now); reason != nil {
		return reason
	}
	// Usable again means a resend replaced the code under us. The code that was
	// submitted is genuinely no longer the right one.
	return domain.ErrChallengeInvalidCode.WithMeta("attempts_remaining", current.AttemptsRemaining())
}

// Resend replaces a challenge's code and clears its attempts.
//
// It deliberately does not move the expiry (BR-AUTH-13): a learner who keeps
// pressing resend gets fresh codes, not an indefinitely valid challenge. It also
// does not un-burn a challenge — BR-AUTH-12 says a spent one must be replaced,
// so resend is not a way around the attempt cap.
func (s *ChallengeService) Resend(ctx context.Context, challengeID uuid.UUID) (Issued, error) {
	return s.resend(ctx, s.repo, challengeID)
}

// ResendIn replaces the code inside tx, so the row that delivers the new code
// commits with the row that changed it. Without that, a resend whose outbox
// write failed would have invalidated the learner's old code and sent them no
// new one — the challenge alive but unusable by anybody.
func (s *ChallengeService) ResendIn(ctx context.Context, tx pgx.Tx, challengeID uuid.UUID) (Issued, error) {
	return s.resend(ctx, s.repo.WithTx(tx), challengeID)
}

func (s *ChallengeService) resend(ctx context.Context, repo Repository, challengeID uuid.UUID) (Issued, error) {
	now := s.clock.Now()

	challenge, err := repo.GetChallenge(ctx, challengeID)
	if err != nil {
		return Issued{}, err
	}
	if err := challenge.Usable(now); err != nil {
		return Issued{}, err
	}

	// The Redis check comes first because it is the cheap one and it stops a
	// spam loop before it reaches Postgres. It is not the authority: the
	// limiter allows the request when Redis is unreachable, so the database
	// guard below is what actually holds the cooldown.
	if err := s.checkResendCooldown(ctx, challenge); err != nil {
		return Issued{}, err
	}

	code, err := domain.NewCode(s.config.CodeLength)
	if err != nil {
		return Issued{}, fmt.Errorf("resend challenge: %w", err)
	}

	resent, ok, err := repo.ResendChallenge(
		ctx, challengeID, s.keys.CodeHash(challengeID, code.Reveal()), now.Add(-s.config.ResendCooldown), now)
	if err != nil {
		return Issued{}, err
	}
	if !ok {
		if reason := s.resendRefusal(ctx, challengeID, now); reason != nil {
			return Issued{}, reason
		}
		return Issued{}, domain.ErrChallengeResendTooSoon
	}

	slog.InfoContext(ctx, "otp challenge code resent",
		"module", "auth", "op", "Resend",
		"challenge_id", resent.ID.String(), "purpose", resent.Purpose.String(),
		"expires_at", resent.ExpiresAt)

	return Issued{Challenge: resent, Code: code}, nil
}

// resendRefusal explains a resend the database would not take. It returns nil
// when the challenge is still usable, which leaves the cooldown as the only
// remaining explanation.
func (s *ChallengeService) resendRefusal(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	current, err := s.repo.GetChallenge(ctx, challengeID)
	if err != nil {
		return err
	}
	return current.Usable(now)
}

// checkIssuanceLimits applies the two issuance caps: three per subject per hour,
// and the global per-IP cap that catches a script issuing challenges against
// many different addresses. The per-challenge attempt cap protects one
// challenge; only this one sees the campaign (AGENT.md §11).
//
// Both caps share one error code. Telling a caller which limit they hit tells
// them how to spread the load to avoid it.
func (s *ChallengeService) checkIssuanceLimits(ctx context.Context, subjectHash []byte) error {
	// The broader cap is evaluated first, so a distributed campaign is stopped
	// by the limiter that exists for it rather than by whichever address it
	// happened to reach three times.
	if address := httpx.ClientIP(ctx); address.IsValid() {
		key := s.limiterKey("otp:issue:ip", hex.EncodeToString(s.keys.SubjectHash(address.String())))
		if err := s.allow(ctx, key, s.config.IssuesPerIPPerHour, time.Hour); err != nil {
			return err
		}
	}

	key := s.limiterKey("otp:issue:subject", hex.EncodeToString(subjectHash))
	return s.allow(ctx, key, s.config.IssuesPerSubjectPerHour, time.Hour)
}

func (s *ChallengeService) checkResendCooldown(ctx context.Context, challenge domain.Challenge) error {
	key := s.limiterKey("otp:resend", challenge.ID.String())
	if err := s.allow(ctx, key, 1, s.config.ResendCooldown); err != nil {
		return domain.ErrChallengeResendTooSoon.WithRetryAfter(int(s.config.ResendCooldown.Seconds()))
	}
	return nil
}

// allow evaluates one limiter. A limiter that reports itself degraded has not
// evaluated anything — Redis is unreachable — and the request goes through,
// which is `cache.Limiter`'s documented behaviour and the reason the database
// carries its own guard for the cooldown.
func (s *ChallengeService) allow(ctx context.Context, key string, limit int, window time.Duration) error {
	if s.limiter == nil {
		return nil
	}
	result, err := s.limiter.Allow(ctx, key, limit, window)
	if err != nil {
		slog.WarnContext(ctx, "otp issuance limiter failed, allowing",
			"module", "auth", "op", "allow", "error", err)
		return nil
	}
	if result.Allowed {
		return nil
	}
	return domain.ErrChallengeIssueLimitReached.WithRetryAfter(int(result.ResetIn.Seconds()))
}

// limiterKey builds the namespaced Redis key. The identifier is always a hash
// or a uuid — never an address and never a subject — so the key space itself
// carries no personal data.
func (s *ChallengeService) limiterKey(entity, identifier string) string {
	return cache.Key(s.env, "auth", entity, identifier, 1)
}
