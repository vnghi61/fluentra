package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// RefreshTokenBytes is the entropy in a refresh token.
//
// Thirty-two bytes is the same 256 bits the token_hash column stores, and it is
// what makes a plain SHA-256 the right digest for this value: there is no
// dictionary to attack and no work factor that would add anything. It is also
// what makes an online guessing limiter unnecessary here — a value this size is
// not reached by guessing, only by theft, which is what reuse detection is for.
const RefreshTokenBytes = 32

// RefreshToken is one row of core.refresh_tokens, as the service sees it.
//
// The token itself is not a member. Only the digest is ever read back out of
// the database, and a struct that could carry the plaintext would eventually be
// filled in by something.
type RefreshToken struct {
	ID        uuid.UUID
	TokenHash []byte
	FamilyID  uuid.UUID
	SessionID uuid.UUID
	IssuedAt  time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

// SessionToken is a refresh token together with the account its session belongs
// to.
//
// The user id is not a column of core.refresh_tokens. It is carried through the
// join the lookup already performs, because every caller needs it to mint the
// access token or to name the account in a security event, and a second query
// for it would be a second chance for the two answers to disagree.
type SessionToken struct {
	RefreshToken

	UserID uuid.UUID
}

// Spent reports whether the token has already been exchanged. Presenting a
// spent token is the reuse signal BR-AUTH-04 turns into a family revocation.
func (t RefreshToken) Spent() bool { return t.UsedAt != nil }

// Revoked reports whether the token was taken away rather than exchanged.
func (t RefreshToken) Revoked() bool { return t.RevokedAt != nil }

// ExpiredAt reports whether the token's idle window had closed by now.
func (t RefreshToken) ExpiredAt(now time.Time) bool { return !now.Before(t.ExpiresAt) }

// NewRefreshToken draws a token and returns it with its digest.
//
// The two are produced together, by one function, so that no caller can hash
// something other than what it handed out — a mismatch there would be a token
// that never verifies, discovered by a learner rather than by a test.
//
// entropy is io.Reader rather than crypto/rand directly so a test can pin the
// bytes. A nil reader means crypto/rand, so the production path cannot be made
// deterministic by forgetting to set a field.
func NewRefreshToken(entropy io.Reader) (token string, digest []byte, err error) {
	if entropy == nil {
		entropy = rand.Reader
	}

	raw := make([]byte, RefreshTokenBytes)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		// A short read from the entropy source must never fall back to
		// something weaker. Failing the sign-in is the safe direction: the
		// learner retries, and nobody is issued a guessable credential.
		return "", nil, fmt.Errorf("draw refresh token: %w", err)
	}

	// Raw URL encoding, so the value survives a cookie, a header and a URL
	// without escaping — and with no padding, which some cookie parsers treat
	// as a delimiter.
	return base64.RawURLEncoding.EncodeToString(raw), HashRefreshToken(raw), nil
}

// HashRefreshToken digests the token bytes for storage and lookup.
func HashRefreshToken(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}

// RefreshTokenDigest decodes a presented token and digests it.
//
// A value that is not the encoding this module produces cannot match any row,
// so it is reported as not-a-token rather than hashed anyway: hashing arbitrary
// input would send a lookup to the database for every malformed string a
// scanner sends.
func RefreshTokenDigest(presented string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(presented)
	if err != nil || len(raw) != RefreshTokenBytes {
		return nil, false
	}
	return HashRefreshToken(raw), true
}

// SameRefreshDigest compares two digests in constant time.
//
// The digests are not secrets — they are what the database stores — so this is
// not guarding against a timing attack on the comparison itself. It exists so
// that a future caller comparing a digest derived from user input has one
// obvious function to reach for rather than `bytes.Equal`, which is the habit
// that puts a variable-time compare on a credential path.
func SameRefreshDigest(left, right []byte) bool {
	return subtle.ConstantTimeCompare(left, right) == 1
}
