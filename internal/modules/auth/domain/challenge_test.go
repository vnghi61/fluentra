package domain_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// challengeKey is a test key. It protects nothing; it exists so the HMAC has a
// key of legal length.
const challengeKey = "test-otp-hmac-key-at-least-32-bytes-long"

func newKeyring(t *testing.T) domain.Keyring {
	t.Helper()
	keys, err := domain.NewKeyring([]byte(challengeKey))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	return keys
}

func TestNewKeyring_RefusesAKeyShorterThanTheMinimum(t *testing.T) {
	// A deployment that left OTP_HMAC_KEY empty must fail loudly. Padding a
	// short key, or hashing it into shape, would produce a system that appears
	// to work while protecting nothing.
	if _, err := domain.NewKeyring([]byte("too short")); err == nil {
		t.Fatal("a nine-byte key was accepted")
	}
	if _, err := domain.NewKeyring(nil); err == nil {
		t.Fatal("an empty key was accepted")
	}
}

func TestPurpose_OnlyTheFourInTheEnumAreValid(t *testing.T) {
	valid := []domain.Purpose{
		domain.PurposeVerifyEmail, domain.PurposeLoginOTP,
		domain.PurposePasswordReset, domain.PurposeLinkOAuth,
	}
	for _, purpose := range valid {
		if !purpose.Valid() {
			t.Errorf("%q is not valid but is in the enum", purpose)
		}
	}
	for _, purpose := range []domain.Purpose{"", "verify", "VERIFY_EMAIL", "send_money"} {
		if purpose.Valid() {
			t.Errorf("%q is valid but is not in the enum", purpose)
		}
	}
}

func TestNewCode_IsTheRequestedNumberOfDigits(t *testing.T) {
	for _, length := range []int{6, 8} {
		code, err := domain.NewCode(length)
		if err != nil {
			t.Fatalf("NewCode(%d): %v", length, err)
		}
		if !domain.ValidCodeShape(code.Reveal(), length) {
			t.Errorf("NewCode(%d) = %q, which is not %d digits", length, code.Reveal(), length)
		}
	}
}

// TestNewCode_UsesTheWholeDigitRange is a coarse check that the draw is not
// biased. A `% 10^n` reduction over a source range that is not a multiple of the
// modulus skews toward the low digits, and a skewed OTP is a smaller keyspace
// than the attempt cap was sized against — a failure that is invisible in every
// functional test.
func TestNewCode_UsesTheWholeDigitRange(t *testing.T) {
	seen := make(map[byte]int)
	const draws = 400
	for range draws {
		code, err := domain.NewCode(domain.CodeLength)
		if err != nil {
			t.Fatalf("NewCode: %v", err)
		}
		for index := range len(code.Reveal()) {
			seen[code.Reveal()[index]]++
		}
	}

	if len(seen) != 10 {
		t.Fatalf("only %d of the ten digits appeared in %d draws", len(seen), draws)
	}
	// 2400 digits over ten values is 240 expected each. A digit appearing fewer
	// than 150 times is far outside sampling noise and means the draw is skewed.
	for digit, count := range seen {
		if count < 150 {
			t.Errorf("digit %q appeared %d times, want roughly 240", digit, count)
		}
	}
}

// TestCode_IsNotPrintedByAccident is the wrapper doing its job. The code travels
// from the service to whatever renders the email, through structs that get
// logged and test failures that get printed.
func TestCode_IsNotPrintedByAccident(t *testing.T) {
	code, err := domain.NewCode(domain.CodeLength)
	if err != nil {
		t.Fatalf("NewCode: %v", err)
	}

	for _, rendered := range []string{
		fmt.Sprintf("%v", code),
		fmt.Sprintf("%+v", code),
		// String() is what the %s verb calls, so covering it covers both.
		code.String(),
		fmt.Sprintf("%v", struct{ Code domain.Code }{Code: code}),
	} {
		if strings.Contains(rendered, code.Reveal()) {
			t.Errorf("formatting produced %q, which contains the code", rendered)
		}
	}
}

func TestValidCodeShape(t *testing.T) {
	cases := map[string]bool{
		"123456":  true,
		"000000":  true,
		"12345":   false,
		"1234567": false,
		"12345a":  false,
		"12345 ":  false,
		"":        false,
		"-12345":  false,
	}
	for code, wanted := range cases {
		if got := domain.ValidCodeShape(code, domain.CodeLength); got != wanted {
			t.Errorf("ValidCodeShape(%q) = %v, want %v", code, got, wanted)
		}
	}
}

// TestSubjectHash_IsKeyedAndCaseInsensitive covers both reasons the subject is
// HMACed rather than hashed: an unkeyed digest of an email is reversible with a
// wordlist, and a case-sensitive one would let the per-subject issuance cap be
// evaded by capitalising a letter.
func TestSubjectHash_IsKeyedAndCaseInsensitive(t *testing.T) {
	keys := newKeyring(t)

	digest := keys.SubjectHash("Learner@Fluentra.test")
	if len(digest) != 32 {
		t.Fatalf("digest is %d bytes, want 32", len(digest))
	}
	if !domain.EqualHash(digest, keys.SubjectHash("  learner@fluentra.test  ")) {
		t.Error("case and surrounding space changed the subject hash")
	}
	if domain.EqualHash(digest, keys.SubjectHash("other@fluentra.test")) {
		t.Error("two different subjects produced the same hash")
	}

	other, err := domain.NewKeyring([]byte("a-completely-different-key-32-bytes!!"))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	if domain.EqualHash(digest, other.SubjectHash("learner@fluentra.test")) {
		t.Error("a different server key produced the same digest, so the hash is not keyed")
	}
}

// TestCodeHash_BindsTheCodeToItsChallenge is the property that makes "a code
// from challenge A does not verify challenge B" structural rather than a
// one-in-a-million coincidence. The same code is hashed under two ids on
// purpose: a construction over the code alone would return equal digests.
func TestCodeHash_BindsTheCodeToItsChallenge(t *testing.T) {
	keys := newKeyring(t)
	first, second := uuid.New(), uuid.New()

	if domain.EqualHash(keys.CodeHash(first, "123456"), keys.CodeHash(second, "123456")) {
		t.Error("one code hashed under two challenge ids produced the same digest")
	}
	if !domain.EqualHash(keys.CodeHash(first, "123456"), keys.CodeHash(first, "123456")) {
		t.Error("the same code and id produced two different digests")
	}
	if domain.EqualHash(keys.CodeHash(first, "123456"), keys.CodeHash(first, "123457")) {
		t.Error("two different codes under one id produced the same digest")
	}
	if len(keys.CodeHash(first, "123456")) != 32 {
		t.Error("the code hash is not a SHA-256 output")
	}
}

// TestCodeHash_SeparatesTheIdFromTheCode guards the concatenation. Without the
// separator, a construction over `id||code` could in principle be confused by a
// value that shifts the boundary between the two.
func TestCodeHash_SeparatesTheIdFromTheCode(t *testing.T) {
	keys := newKeyring(t)
	challengeID := uuid.New()

	// The id is a fixed 36 characters, so this cannot actually collide today.
	// The assertion pins the behaviour so that a future change to the id format
	// does not quietly make it possible.
	if domain.EqualHash(keys.CodeHash(challengeID, "123456"),
		keys.SubjectHash(challengeID.String()+"123456")) {
		t.Error("the code hash is a bare concatenation")
	}
}

func newChallenge(now time.Time) domain.Challenge {
	return domain.Challenge{
		ID:          uuid.New(),
		Purpose:     domain.PurposeVerifyEmail,
		Attempts:    0,
		MaxAttempts: domain.MaxAttempts,
		ExpiresAt:   now.Add(domain.ChallengeTTL),
		LastSentAt:  now,
		CreatedAt:   now,
	}
}

func TestChallenge_UsableUntilItIsConsumedBurnedOrExpired(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)

	fresh := newChallenge(now)
	if err := fresh.Usable(now); err != nil {
		t.Fatalf("a fresh challenge is unusable: %v", err)
	}

	consumed := newChallenge(now)
	at := now
	consumed.ConsumedAt = &at
	if err := consumed.Usable(now); err != domain.ErrChallengeAlreadyUsed {
		t.Errorf("consumed: error = %v, want ErrChallengeAlreadyUsed", err)
	}

	burned := newChallenge(now)
	burned.Attempts = domain.MaxAttempts
	if err := burned.Usable(now); err != domain.ErrChallengeAttemptsExceeded {
		t.Errorf("burned: error = %v, want ErrChallengeAttemptsExceeded", err)
	}

	expired := newChallenge(now)
	if err := expired.Usable(now.Add(domain.ChallengeTTL)); err != domain.ErrChallengeExpired {
		t.Errorf("expired: error = %v, want ErrChallengeExpired", err)
	}
}

// TestChallenge_ExpiryBoundaryIsInclusiveOfTheLastInstant pins which side of the
// boundary counts. One nanosecond inside the window must still work; the instant
// the window closes must not.
func TestChallenge_ExpiryBoundaryIsInclusiveOfTheLastInstant(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)
	challenge := newChallenge(now)

	if challenge.Expired(challenge.ExpiresAt.Add(-time.Nanosecond)) {
		t.Error("a challenge one nanosecond before its expiry is already expired")
	}
	if !challenge.Expired(challenge.ExpiresAt) {
		t.Error("a challenge at its exact expiry is not expired")
	}
}

// TestChallenge_BurnedIsDerivedFromTheAttemptCount pins that there is no second
// source of truth. A stored `burned_at` could disagree with the count, and a
// disagreement between two records of the same fact is what an attacker looks
// for.
func TestChallenge_BurnedIsDerivedFromTheAttemptCount(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)

	for attempts := range domain.MaxAttempts + 1 {
		challenge := newChallenge(now)
		challenge.Attempts = attempts

		wantBurned := attempts >= domain.MaxAttempts
		if challenge.Burned() != wantBurned {
			t.Errorf("attempts = %d: Burned() = %v, want %v", attempts, challenge.Burned(), wantBurned)
		}
		if want := domain.MaxAttempts - attempts; challenge.AttemptsRemaining() != max(want, 0) {
			t.Errorf("attempts = %d: remaining = %d, want %d", attempts, challenge.AttemptsRemaining(), want)
		}
	}
}

func TestChallenge_ResendAllowedAtIsOneCooldownAfterTheLastDelivery(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)
	challenge := newChallenge(now)

	if want := now.Add(domain.ResendCooldown); !challenge.ResendAllowedAt().Equal(want) {
		t.Errorf("resend allowed at = %s, want %s", challenge.ResendAllowedAt(), want)
	}
}

// TestEqualHash_MatchesOnlyIdenticalDigests. The constant-time property itself
// comes from crypto/subtle via hmac.Equal and is verified by reading it, not by
// a wall-clock measurement — a timing assertion is flaky on shared CI hardware.
// What this pins is that the comparison is total: no prefix, no length, no
// early match.
func TestEqualHash_MatchesOnlyIdenticalDigests(t *testing.T) {
	keys := newKeyring(t)
	challengeID := uuid.New()
	digest := keys.CodeHash(challengeID, "123456")

	if !domain.EqualHash(digest, keys.CodeHash(challengeID, "123456")) {
		t.Error("two digests of the same input did not match")
	}
	if domain.EqualHash(digest, digest[:len(digest)-1]) {
		t.Error("a truncated digest matched, so length is not compared")
	}

	differsAtTheEnd := append([]byte(nil), digest...)
	differsAtTheEnd[len(differsAtTheEnd)-1] ^= 0x01
	if domain.EqualHash(digest, differsAtTheEnd) {
		t.Error("a digest differing only in its last byte matched")
	}

	differsAtTheStart := append([]byte(nil), digest...)
	differsAtTheStart[0] ^= 0x01
	if domain.EqualHash(digest, differsAtTheStart) {
		t.Error("a digest differing only in its first byte matched")
	}
}
