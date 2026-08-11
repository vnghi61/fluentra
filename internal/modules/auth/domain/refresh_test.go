package domain_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

func TestNewRefreshToken_DrawsAFullWidthValueAndItsMatchingDigest(t *testing.T) {
	t.Parallel()

	token, digest, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("the token is not the encoding this module claims to produce: %v", err)
	}
	if len(raw) != domain.RefreshTokenBytes {
		t.Errorf("token carries %d bytes of entropy, want %d", len(raw), domain.RefreshTokenBytes)
	}
	if len(digest) != 32 {
		t.Errorf("digest is %d bytes, want the 32 the column's CHECK constraint requires", len(digest))
	}

	// The pairing is the point: a function that returned a digest of anything
	// other than the token it handed out would issue a credential that can
	// never be exchanged, and the first person to find out would be a learner.
	recomputed, ok := domain.RefreshTokenDigest(token)
	if !ok {
		t.Fatal("the token this function produced was rejected by its own parser")
	}
	if !bytes.Equal(recomputed, digest) {
		t.Error("the returned digest is not the digest of the returned token")
	}
}

// TestNewRefreshToken_DoesNotRepeatItself is a smoke test against the mistake
// that would be catastrophic and silent: a token derived from something
// predictable. It cannot prove randomness, but it does catch a constant.
func TestNewRefreshToken_DoesNotRepeatItself(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 256)
	for range 256 {
		token, _, err := domain.NewRefreshToken(nil)
		if err != nil {
			t.Fatalf("NewRefreshToken: %v", err)
		}
		if _, repeated := seen[token]; repeated {
			t.Fatal("the same refresh token was drawn twice")
		}
		seen[token] = struct{}{}
	}
}

// TestNewRefreshToken_FailsRatherThanWeakenTheToken pins the direction of the
// failure. A short read from the entropy source must not be padded, retried
// with a fallback, or truncated into a shorter token — the sign-in fails, the
// learner tries again, and nobody is issued a guessable credential.
func TestNewRefreshToken_FailsRatherThanWeakenTheToken(t *testing.T) {
	t.Parallel()

	starved := strings.NewReader("only-a-few-bytes")

	token, digest, err := domain.NewRefreshToken(starved)
	if err == nil {
		t.Fatal("a token was issued from an entropy source that could not fill it")
	}
	if token != "" || digest != nil {
		t.Errorf("a failed draw still returned material: token=%q digest=%v", token, digest)
	}
}

func TestRefreshTokenDigest_RejectsAnythingThatIsNotOneOfOurs(t *testing.T) {
	t.Parallel()

	valid, _, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}

	rejected := map[string]string{
		"empty":               "",
		"not base64":          "!!!!not-base64!!!!",
		"standard padding":    base64.StdEncoding.EncodeToString(make([]byte, domain.RefreshTokenBytes)),
		"too few bytes":       base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
		"too many bytes":      base64.RawURLEncoding.EncodeToString(make([]byte, 64)),
		"a jwt, not a cookie": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.sig", // gitleaks:allow
	}
	for name, value := range rejected {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, ok := domain.RefreshTokenDigest(value); ok {
				t.Errorf("%s was accepted as a refresh token", name)
			}
		})
	}

	if _, ok := domain.RefreshTokenDigest(valid); !ok {
		t.Error("a token this package produced was rejected")
	}
}

func TestRefreshToken_StateReadsTheWayTheRotationBranchesOnIt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)
	earlier := now.Add(-time.Hour)

	live := domain.RefreshToken{ExpiresAt: now.Add(time.Hour)}
	if live.Spent() || live.Revoked() || live.ExpiredAt(now) {
		t.Error("a fresh token reported as spent, revoked or expired")
	}

	spent := live
	spent.UsedAt = &earlier
	if !spent.Spent() {
		t.Error("a token with used_at set did not report as spent")
	}

	revoked := live
	revoked.RevokedAt = &earlier
	if !revoked.Revoked() {
		t.Error("a token with revoked_at set did not report as revoked")
	}

	// The boundary is the acceptance criterion: one millisecond past expiry
	// fails, and the last instant inside it does not.
	expiring := domain.RefreshToken{ExpiresAt: now}
	if !expiring.ExpiredAt(now) {
		t.Error("a token expiring exactly now was still accepted")
	}
	if expiring.ExpiredAt(now.Add(-time.Millisecond)) {
		t.Error("a token one millisecond inside its window was reported expired")
	}
}

func TestSameRefreshDigest(t *testing.T) {
	t.Parallel()

	_, left, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	_, right, err := domain.NewRefreshToken(nil)
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}

	if !domain.SameRefreshDigest(left, left) {
		t.Error("a digest did not equal itself")
	}
	if domain.SameRefreshDigest(left, right) {
		t.Error("two different digests compared equal")
	}
	if domain.SameRefreshDigest(left, left[:16]) {
		t.Error("a truncated digest compared equal to the full one")
	}
}
