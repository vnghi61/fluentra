//go:build integration

package repository_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/modules/auth/repository"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// challengeNow is a fixed instant. Every timestamp these tests write comes from
// the caller rather than from the database's now(), which is what lets an
// expiry be crossed without sleeping.
var challengeNow = time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)

func newChallengeKeyring(t *testing.T) domain.Keyring {
	t.Helper()
	keys, err := domain.NewKeyring([]byte("integration-otp-hmac-key-32-bytes-min"))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	return keys
}

// seedChallenge writes one challenge and returns it with the code that opens it.
func seedChallenge(ctx context.Context, t *testing.T, repo *repository.Repository, keys domain.Keyring) (
	domain.Challenge, string,
) {
	t.Helper()

	challengeID := newID(ctx, t)
	const code = "123456"

	challenge, err := repo.CreateChallenge(ctx, domain.NewChallengeInput{
		ID:          challengeID,
		Purpose:     domain.PurposeVerifyEmail,
		SubjectHash: keys.SubjectHash("learner@fluentra.test"),
		CodeHash:    keys.CodeHash(challengeID, code),
		MaxAttempts: domain.MaxAttempts,
		ExpiresAt:   challengeNow.Add(domain.ChallengeTTL),
		Now:         challengeNow,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	t.Cleanup(func() { deleteChallenge(challengeID) }) //nolint:contextcheck // deleteChallenge explains it
	return challenge, code
}

// deleteChallenge removes a seeded row once the test body has returned. It takes
// no context on purpose: by cleanup time the test's context can already be
// cancelled, and the row would survive into the next test.
func deleteChallenge(challengeID uuid.UUID) {
	_, _ = pool.Exec(context.Background(), `DELETE FROM core.auth_challenges WHERE id = $1`, challengeID)
}

func TestChallenge_RoundTripsThroughTheDomainShape(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)

	written, code := seedChallenge(ctx, t, repo, keys)

	read, err := repo.GetChallenge(ctx, written.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if read.Purpose != domain.PurposeVerifyEmail {
		t.Errorf("purpose = %q, want verify_email", read.Purpose)
	}
	if !domain.EqualHash(read.CodeHash, keys.CodeHash(written.ID, code)) {
		t.Error("the code hash did not survive the round trip")
	}
	if read.Attempts != 0 || read.MaxAttempts != domain.MaxAttempts {
		t.Errorf("attempts = %d/%d, want 0/%d", read.Attempts, read.MaxAttempts, domain.MaxAttempts)
	}
	if read.Consumed() || read.Burned() {
		t.Error("a fresh challenge reports itself consumed or burned")
	}
}

func TestChallenge_UnknownIDIsANotFoundError(t *testing.T) {
	repo, ctx := newRepository(t)

	_, err := repo.GetChallenge(ctx, newID(ctx, t))
	if !apperr.Is(err, apperr.NotFound) {
		t.Fatalf("error = %v, want a not-found error", err)
	}
}

// TestChallenge_ConsumptionIsSingleUseUnderConcurrency is the acceptance
// criterion "the code is single-use", proved against the real planner rather
// than against the fake that imitates it. Ten goroutines present the same valid
// code at once; exactly one may win.
//
// This is the test the guarded UPDATE exists for. A read-then-write in the
// service would pass every sequential test and fail this one.
func TestChallenge_ConsumptionIsSingleUseUnderConcurrency(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	const racers = 10
	var (
		wait      sync.WaitGroup
		start     = make(chan struct{})
		mu        sync.Mutex
		successes int
	)
	for range racers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, ok, err := repo.ConsumeChallenge(ctx, challenge.ID, challenge.CodeHash, challengeNow)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("ConsumeChallenge: %v", err)
				return
			}
			if ok {
				successes++
			}
		}()
	}
	close(start)
	wait.Wait()

	if successes != 1 {
		t.Errorf("%d of %d concurrent consumptions succeeded, want exactly 1", successes, racers)
	}
}

func TestChallenge_ConsumptionRequiresTheCurrentCodeHash(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	_, ok, err := repo.ConsumeChallenge(ctx, challenge.ID, keys.CodeHash(challenge.ID, "999999"), challengeNow)
	if err != nil {
		t.Fatalf("ConsumeChallenge: %v", err)
	}
	if ok {
		t.Error("a challenge was consumed with the wrong code hash")
	}
}

func TestChallenge_ConsumptionIsRefusedAfterTheExpiry(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	_, ok, err := repo.ConsumeChallenge(ctx, challenge.ID, challenge.CodeHash,
		challengeNow.Add(domain.ChallengeTTL))
	if err != nil {
		t.Fatalf("ConsumeChallenge: %v", err)
	}
	if ok {
		t.Error("an expired challenge was consumed")
	}
}

// TestChallenge_AttemptsStopAtExactlyMaxAttempts is BR-AUTH-12 against the
// database. The count is checked at every step and the sixth charge must find
// no row — not raise a constraint violation, which would surface to a learner
// as a 500.
func TestChallenge_AttemptsStopAtExactlyMaxAttempts(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	for attempt := 1; attempt <= domain.MaxAttempts; attempt++ {
		charged, ok, err := repo.RecordFailedAttempt(ctx, challenge.ID, challengeNow)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if !ok {
			t.Fatalf("attempt %d found no row, want it charged", attempt)
		}
		if charged.Attempts != attempt {
			t.Errorf("attempt %d recorded %d attempts", attempt, charged.Attempts)
		}
	}

	_, ok, err := repo.RecordFailedAttempt(ctx, challenge.ID, challengeNow)
	if err != nil {
		t.Fatalf("the sixth attempt errored rather than matching no row: %v", err)
	}
	if ok {
		t.Error("a sixth attempt was charged against a five-attempt challenge")
	}

	// And a burned challenge cannot then be consumed, even with the right code.
	if _, ok, err := repo.ConsumeChallenge(ctx, challenge.ID, challenge.CodeHash, challengeNow); err != nil {
		t.Fatalf("ConsumeChallenge: %v", err)
	} else if ok {
		t.Error("a burned challenge was consumed with the correct code")
	}
}

// TestChallenge_ConcurrentAttemptsEachCostOne is why the increment is
// `attempts + 1` in SQL rather than a value the caller read. Five goroutines
// guessing at once must spend five attempts, not one.
func TestChallenge_ConcurrentAttemptsEachCostOne(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	var wait sync.WaitGroup
	for range domain.MaxAttempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, _, err := repo.RecordFailedAttempt(ctx, challenge.ID, challengeNow); err != nil {
				t.Errorf("RecordFailedAttempt: %v", err)
			}
		}()
	}
	wait.Wait()

	after, err := repo.GetChallenge(ctx, challenge.ID)
	if err != nil {
		t.Fatalf("GetChallenge: %v", err)
	}
	if after.Attempts != domain.MaxAttempts {
		t.Errorf("attempts = %d after %d concurrent guesses, want %d",
			after.Attempts, domain.MaxAttempts, domain.MaxAttempts)
	}
	if !after.Burned() {
		t.Error("the challenge is not burned after its whole budget was spent")
	}
}

// TestChallenge_ResendReplacesTheCodeWithoutMovingTheExpiry is BR-AUTH-13
// against the real UPDATE. The expiry assertion is the one that matters: a
// resend that also extended it would give an attacker an indefinitely valid
// challenge for the price of pressing a button, and the happy path looks
// identical either way.
func TestChallenge_ResendReplacesTheCodeWithoutMovingTheExpiry(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, code := seedChallenge(ctx, t, repo, keys)

	if _, _, err := repo.RecordFailedAttempt(ctx, challenge.ID, challengeNow); err != nil {
		t.Fatalf("RecordFailedAttempt: %v", err)
	}

	later := challengeNow.Add(domain.ResendCooldown)
	resent, ok, err := repo.ResendChallenge(ctx, challenge.ID,
		keys.CodeHash(challenge.ID, "654321"), later.Add(-domain.ResendCooldown), later)
	if err != nil {
		t.Fatalf("ResendChallenge: %v", err)
	}
	if !ok {
		t.Fatal("a resend past the cooldown found no row")
	}

	if !resent.ExpiresAt.Equal(challenge.ExpiresAt) {
		t.Errorf("expiry moved to %s from %s", resent.ExpiresAt, challenge.ExpiresAt)
	}
	if resent.Attempts != 0 {
		t.Errorf("attempts = %d after a resend, want 0", resent.Attempts)
	}
	if domain.EqualHash(resent.CodeHash, keys.CodeHash(challenge.ID, code)) {
		t.Error("the stored code hash did not change")
	}
}

// TestChallenge_ResendIsRefusedInsideTheCooldown is the database's own guard —
// the one that still holds when Redis is unreachable and `cache.Limiter`
// degrades to allowing everything.
func TestChallenge_ResendIsRefusedInsideTheCooldown(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	tooSoon := challengeNow.Add(domain.ResendCooldown - time.Second)
	_, ok, err := repo.ResendChallenge(ctx, challenge.ID,
		keys.CodeHash(challenge.ID, "654321"), tooSoon.Add(-domain.ResendCooldown), tooSoon)
	if err != nil {
		t.Fatalf("ResendChallenge: %v", err)
	}
	if ok {
		t.Error("a resend inside the cooldown was accepted")
	}
}

func TestChallenge_ResendDoesNotUnburnAChallenge(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	for range domain.MaxAttempts {
		if _, _, err := repo.RecordFailedAttempt(ctx, challenge.ID, challengeNow); err != nil {
			t.Fatalf("RecordFailedAttempt: %v", err)
		}
	}

	later := challengeNow.Add(domain.ResendCooldown)
	_, ok, err := repo.ResendChallenge(ctx, challenge.ID,
		keys.CodeHash(challenge.ID, "654321"), later.Add(-domain.ResendCooldown), later)
	if err != nil {
		t.Fatalf("ResendChallenge: %v", err)
	}
	if ok {
		t.Error("a burned challenge was resent, which is a way around the attempt cap")
	}
}

func TestChallenge_ResendIsRefusedForAConsumedChallenge(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	if _, ok, err := repo.ConsumeChallenge(ctx, challenge.ID, challenge.CodeHash, challengeNow); err != nil || !ok {
		t.Fatalf("ConsumeChallenge: ok=%v err=%v", ok, err)
	}

	later := challengeNow.Add(domain.ResendCooldown)
	_, ok, err := repo.ResendChallenge(ctx, challenge.ID,
		keys.CodeHash(challenge.ID, "654321"), later.Add(-domain.ResendCooldown), later)
	if err != nil {
		t.Fatalf("ResendChallenge: %v", err)
	}
	if ok {
		t.Error("a consumed challenge was resent")
	}
}

// TestChallengeSchema_RefusesAnAttemptCountPastTheCap pins the cap as a
// constraint rather than only a WHERE clause. A missing guard in application
// code should be a failed statement, not an unbounded guessing budget.
func TestChallengeSchema_RefusesAnAttemptCountPastTheCap(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challenge, _ := seedChallenge(ctx, t, repo, keys)

	_, err := pool.Exec(ctx, `UPDATE core.auth_challenges SET attempts = max_attempts + 1 WHERE id = $1`,
		challenge.ID)
	if err == nil {
		t.Fatal("the database accepted an attempt count past the cap")
	}
	if !strings.Contains(err.Error(), "ck_auth_challenges_attempts") {
		t.Errorf("error = %v, want the attempts CHECK to have rejected it", err)
	}
}

// TestChallengeSchema_RefusesATruncatedDigest catches a plaintext code, or a
// truncated hash, reaching a column that is supposed to hold a SHA-256 output.
func TestChallengeSchema_RefusesATruncatedDigest(t *testing.T) {
	_, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challengeID := newID(ctx, t)

	_, err := pool.Exec(ctx, `
		INSERT INTO core.auth_challenges (id, purpose, subject_hash, code_hash, expires_at)
		VALUES ($1, 'verify_email', $2, $3, now() + interval '10 minutes')`,
		challengeID, keys.SubjectHash("learner@fluentra.test"), []byte("123456"))
	if err == nil {
		deleteChallenge(challengeID)
		t.Fatal("the database accepted a six-byte code hash")
	}
	if !strings.Contains(err.Error(), "ck_auth_challenges_digest_lengths") {
		t.Errorf("error = %v, want the digest-length CHECK to have rejected it", err)
	}
}

func TestChallengeSchema_RefusesAnExpiryBeforeIssuance(t *testing.T) {
	_, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challengeID := newID(ctx, t)

	_, err := pool.Exec(ctx, `
		INSERT INTO core.auth_challenges (id, purpose, subject_hash, code_hash, expires_at, created_at)
		VALUES ($1, 'verify_email', $2, $3, $4, $5)`,
		challengeID, keys.SubjectHash("learner@fluentra.test"), keys.CodeHash(challengeID, "123456"),
		challengeNow, challengeNow.Add(time.Minute))
	if err == nil {
		deleteChallenge(challengeID)
		t.Fatal("the database accepted a challenge that expired before it was issued")
	}
	if !strings.Contains(err.Error(), "ck_auth_challenges_expiry_after_issue") {
		t.Errorf("error = %v, want the expiry CHECK to have rejected it", err)
	}
}

// TestChallengeSchema_RefusesAPurposeOutsideTheEnum is what makes `purpose` a
// closed set. Free text would let a caller invent one and silently bypass the
// limits that purpose deserves.
func TestChallengeSchema_RefusesAPurposeOutsideTheEnum(t *testing.T) {
	_, ctx := newRepository(t)
	keys := newChallengeKeyring(t)
	challengeID := newID(ctx, t)

	_, err := pool.Exec(ctx, `
		INSERT INTO core.auth_challenges (id, purpose, subject_hash, code_hash, expires_at)
		VALUES ($1, 'send_money', $2, $3, now() + interval '10 minutes')`,
		challengeID, keys.SubjectHash("learner@fluentra.test"), keys.CodeHash(challengeID, "123456"))
	if err == nil {
		deleteChallenge(challengeID)
		t.Fatal("the database accepted a purpose that is not in the enum")
	}
}

// TestChallenge_ARolledBackTransactionLeavesNoChallenge is what the outbox
// pattern rests on for P2.2: the account, the credential, the challenge and the
// row that sends the code are one transaction, so a registration that fails
// after the challenge is written must not leave a code anybody could use.
func TestChallenge_ARolledBackTransactionLeavesNoChallenge(t *testing.T) {
	repo, ctx := newRepository(t)
	keys := newChallengeKeyring(t)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	challengeID := newID(ctx, t)
	if _, err := repo.WithTx(tx).CreateChallenge(ctx, domain.NewChallengeInput{
		ID:          challengeID,
		Purpose:     domain.PurposeVerifyEmail,
		SubjectHash: keys.SubjectHash("rollback@fluentra.test"),
		CodeHash:    keys.CodeHash(challengeID, "123456"),
		MaxAttempts: domain.MaxAttempts,
		ExpiresAt:   challengeNow.Add(domain.ChallengeTTL),
		Now:         challengeNow,
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("CreateChallenge in tx: %v", err)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if _, err := repo.GetChallenge(ctx, challengeID); !apperr.Is(err, apperr.NotFound) {
		deleteChallenge(challengeID)
		t.Fatalf("error = %v, want the rolled-back challenge to be gone", err)
	}
}
