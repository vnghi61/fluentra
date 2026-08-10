package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/secret"
)

// Purpose is what a challenge proves. The set is closed and matches the
// core.challenge_purpose enum; ADR-0021 chose one generic subsystem over three
// purpose-specific ones precisely so that constant-time comparison and attempt
// capping are written once.
type Purpose string

// The four purposes. Only PurposeVerifyEmail is used before P2.2; the other
// three exist because the enum is what the migration created and a caller
// inventing a fifth should not compile.
const (
	PurposeVerifyEmail   Purpose = "verify_email"
	PurposeLoginOTP      Purpose = "login_otp"
	PurposePasswordReset Purpose = "password_reset"
	PurposeLinkOAuth     Purpose = "link_oauth"
)

// Valid reports whether p is one of the four the database will accept.
func (p Purpose) Valid() bool {
	switch p {
	case PurposeVerifyEmail, PurposeLoginOTP, PurposePasswordReset, PurposeLinkOAuth:
		return true
	default:
		return false
	}
}

// String implements fmt.Stringer.
func (p Purpose) String() string { return string(p) }

// Challenge defaults, from ADR-0021 and the OTP_* keys in `.env.example`.
const (
	// CodeLength is the number of digits. Six is 10^6 of entropy, which is only
	// safe because MaxAttempts and the issuance limiters bound the guesses —
	// they are load-bearing, not decorative (BR-AUTH-10, AGENT.md §11).
	CodeLength = 6
	// MaxAttempts is the hard cap. The sixth wrong code does not fail, it finds
	// a burned challenge (BR-AUTH-12).
	MaxAttempts = 5
	// ChallengeTTL is absolute and is never extended by a resend (BR-AUTH-13).
	ChallengeTTL = 10 * time.Minute
	// ResendCooldown is the minimum gap between deliveries for one challenge.
	ResendCooldown = 60 * time.Second
)

// Challenge is one short-lived one-time code.
//
// The subject is stored only as a keyed HMAC, so the row carries no address.
// UserID is separate and is the account it belongs to: the digest is
// irreversible by design, so verification could not otherwise find the account
// it is supposed to mark verified.
type Challenge struct {
	ID      uuid.UUID
	Purpose Purpose

	// SubjectHash and CodeHash are HMAC-SHA256 outputs. Neither is a secret in
	// the sense the code is — they cannot be replayed — but neither is
	// something to print, so they are not rendered by any method here.
	SubjectHash []byte
	CodeHash    []byte

	// UserID is the account the challenge belongs to, when there is one. It is
	// a pointer because a challenge can legitimately precede an account:
	// link_oauth runs before a Google sign-up has one.
	UserID *uuid.UUID

	Attempts    int
	MaxAttempts int

	ExpiresAt  time.Time
	ConsumedAt *time.Time
	LastSentAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewChallengeInput is a challenge about to be written. It is a struct rather
// than seven positional parameters because five of them are timestamps and byte
// slices, and a call site that transposes two of those still compiles.
//
// It lives here rather than in the repository so the service can declare its
// persistence interface without importing the adapter that satisfies it.
type NewChallengeInput struct {
	ID          uuid.UUID
	Purpose     Purpose
	SubjectHash []byte
	CodeHash    []byte
	MaxAttempts int
	ExpiresAt   time.Time
	UserID      *uuid.UUID
	Now         time.Time
}

// Consumed reports whether the code has already been used.
func (c Challenge) Consumed() bool { return c.ConsumedAt != nil }

// Burned reports whether the attempt cap has been reached.
//
// It is derived rather than stored. A `burned_at` column would be a second
// source of truth for something the attempt count already says, and the two
// could disagree — which is the sort of disagreement an attacker looks for.
func (c Challenge) Burned() bool { return c.Attempts >= c.MaxAttempts }

// Expired reports whether the absolute window has passed.
func (c Challenge) Expired(now time.Time) bool { return !now.Before(c.ExpiresAt) }

// AttemptsRemaining is what the client is told after a wrong code, so a learner
// knows how many tries are left before they must request a new challenge
// (ADR-0021). It never goes below zero.
func (c Challenge) AttemptsRemaining() int { return max(c.MaxAttempts-c.Attempts, 0) }

// ResendAllowedAt is the earliest a new code may be delivered for this
// challenge.
func (c Challenge) ResendAllowedAt() time.Time { return c.LastSentAt.Add(ResendCooldown) }

// Usable returns the reason this challenge cannot be verified, or nil.
//
// The order matters. Consumed is checked before burned and burned before
// expired, so a learner is told the most actionable thing first: "you already
// used this" is more useful than "this expired", and both are more useful than
// a generic failure. None of the three reveals anything an attacker holding the
// challenge id does not already know — the id is the secret that gates all of
// this (BR-AUTH-11).
func (c Challenge) Usable(now time.Time) error {
	switch {
	case c.Consumed():
		return ErrChallengeAlreadyUsed
	case c.Burned():
		return ErrChallengeAttemptsExceeded
	case c.Expired(now):
		return ErrChallengeExpired
	default:
		return nil
	}
}

// Code is a generated one-time code, wrapped so it cannot be formatted by
// accident.
//
// The wrapper is not decoration. This value travels from the service to
// whatever puts it in an email, and on the way it passes through structs that
// get logged and test failures that get printed. Reading it takes an explicit
// Reveal, which is one call site.
type Code = secret.Redacted[string]

// NewCode draws a code of length digits from crypto/rand.
//
// Each digit is drawn with rand.Int over exactly ten values, so the result is
// uniform. The obvious alternative — one random integer reduced with `% 10^n` —
// is biased unless the source range is a multiple of the modulus, and a biased
// OTP is a smaller keyspace than the one the attempt cap was sized against.
//
// The length is a parameter rather than the constant because OTP_CODE_LENGTH is
// a real configuration key; a constant here would mean an operator could set it
// and watch nothing happen.
func NewCode(length int) (Code, error) {
	if length <= 0 {
		length = CodeLength
	}
	// Indexing a table rather than adding to '0'. The arithmetic would be
	// correct — rand.Int bounds the value to [0,10) — but it converts an int64
	// to a byte, and a conversion the reader has to prove safe is worse than a
	// lookup that cannot be unsafe.
	const digits = "0123456789"

	var builder strings.Builder
	builder.Grow(length)
	for range length {
		digit, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return Code{}, fmt.Errorf("draw otp digit: %w", err)
		}
		builder.WriteByte(digits[digit.Int64()])
	}
	return secret.New(builder.String()), nil
}

// ValidCodeShape reports whether a submitted string could be a code at all.
//
// It exists so a submission of the wrong shape is rejected before anything is
// hashed, and it is not a security check: it leaks nothing an attacker could
// not determine by counting the digits they typed. The attempt counter is still
// charged for a wrong shape, because otherwise "send garbage" would be a free
// way to keep a challenge alive past its attempt budget.
func ValidCodeShape(code string, length int) bool {
	if length <= 0 {
		length = CodeLength
	}
	if len(code) != length {
		return false
	}
	for index := range len(code) {
		if code[index] < '0' || code[index] > '9' {
			return false
		}
	}
	return true
}

// Keyring derives the HMACs this module stores. It holds the server key from
// OTP_HMAC_KEY and is the only thing that ever sees it.
type Keyring struct {
	key []byte
}

// NewKeyring builds a keyring over key.
//
// A short key is an error rather than something to pad or hash into shape: the
// alternative is a deployment that silently protects nothing because someone
// left the value empty.
func NewKeyring(key []byte) (Keyring, error) {
	const minKeyLength = 32
	if len(key) < minKeyLength {
		return Keyring{}, fmt.Errorf("otp hmac key must be at least %d bytes, got %d", minKeyLength, len(key))
	}
	return Keyring{key: append([]byte(nil), key...)}, nil
}

// SubjectHash keys an email address or other subject so the stored row cannot
// be reversed into an address book. Subjects are lowercased first, so the same
// address written two ways produces one hash and the per-subject issuance limit
// cannot be evaded by capitalising a letter.
func (k Keyring) SubjectHash(subject string) []byte {
	mac := hmac.New(sha256.New, k.key)
	mac.Write([]byte(strings.ToLower(strings.TrimSpace(subject))))
	return mac.Sum(nil)
}

// CodeHash binds the code to the challenge it belongs to.
//
// The challenge id goes into the MAC, not just the code. Without it, two live
// challenges that happened to draw the same six digits would accept each
// other's code — a one-in-a-million coincidence that becomes a certainty at
// volume. With it, "a code from challenge A does not verify challenge B" is a
// property of the construction rather than of the lookup.
//
// The separator matters for the same reason any length-extension guard does:
// the id is a fixed 36 characters here, but a bare concatenation is a habit
// worth not forming.
func (k Keyring) CodeHash(challengeID uuid.UUID, code string) []byte {
	mac := hmac.New(sha256.New, k.key)
	mac.Write([]byte(challengeID.String()))
	mac.Write([]byte{0})
	mac.Write([]byte(code))
	return mac.Sum(nil)
}

// EqualHash compares two digests in constant time.
//
// hmac.Equal is crypto/subtle.ConstantTimeCompare. Using `bytes.Equal` here
// would return as soon as two bytes differed, and the time it took to do so
// would tell an attacker how many leading bytes of the digest they had guessed
// — which, over enough requests, is the digest.
func EqualHash(left, right []byte) bool { return hmac.Equal(left, right) }
