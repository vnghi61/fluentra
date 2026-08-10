package domain

import (
	"errors"
	"fmt"

	"github.com/alexedwards/argon2id"
)

// HashParams are the Argon2id cost parameters. They mirror the ARGON2_* keys in
// `.env.example`; the defaults below are those values, so a caller that has no
// configuration yet still hashes at the documented strength rather than at the
// library's own defaults, which are weaker (t=1, p=NumCPU).
type HashParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultHashParams is SECURITY_GUIDELINE §2: m=64 MiB, t=3, p=2. Salt and key
// lengths are not configurable — 16 and 32 bytes are the PHC recommendations
// and there is no operational reason to tune them.
func DefaultHashParams() HashParams {
	return HashParams{
		MemoryKiB:   64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func (p HashParams) library() *argon2id.Params {
	return &argon2id.Params{
		Memory:      p.MemoryKiB,
		Iterations:  p.Iterations,
		Parallelism: p.Parallelism,
		SaltLength:  p.SaltLength,
		KeyLength:   p.KeyLength,
	}
}

func paramsFromLibrary(p *argon2id.Params) HashParams {
	return HashParams{
		MemoryKiB:   p.Memory,
		Iterations:  p.Iterations,
		Parallelism: p.Parallelism,
		SaltLength:  p.SaltLength,
		KeyLength:   p.KeyLength,
	}
}

// Hasher produces and verifies Argon2id password hashes at one set of
// parameters. It is immutable and safe to share: every call derives its own
// salt, and nothing is written back to the receiver.
type Hasher struct {
	params HashParams
}

// NewHasher returns a hasher that produces hashes at params.
func NewHasher(params HashParams) Hasher { return Hasher{params: params} }

// Params returns the parameters new hashes are produced at.
func (h Hasher) Params() HashParams { return h.params }

// Hash produces a PHC-encoded Argon2id hash of password.
//
// The password is never returned, logged, or stored anywhere but inside the
// digest, and the returned string is the only thing a caller should persist.
func (h Hasher) Hash(password string) (string, error) {
	encoded, err := argon2id.CreateHash(password, h.params.library())
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return encoded, nil
}

// Verification is the outcome of checking a password against a stored hash.
type Verification struct {
	// Matches is whether the password is the one the hash was made from.
	Matches bool
	// NeedsRehash is whether the hash was made with parameters other than the
	// ones this hasher produces. It is only ever true alongside Matches: the
	// plaintext is needed to produce the replacement, and a caller only has a
	// verified plaintext when the comparison succeeded (BR-AUTH-01).
	NeedsRehash bool
}

// Verify compares password against a stored PHC hash in constant time.
//
// Constant-time is not this function's doing: argon2id.CheckHash derives the
// key from the salt and parameters carried in the stored string and compares it
// with crypto/subtle.ConstantTimeCompare, so the work and the comparison are
// identical for a right and a wrong password of any shape. What this function
// must not do is add a cheaper path in front of it — a length check, a prefix
// check, an early return for the empty string — because any of those would
// reintroduce the side channel the library removed.
//
// A stored string that is not a readable Argon2id hash is an error rather than
// a non-match: it means the row is corrupt, and reporting "wrong password" for
// it would leave a learner locked out of an account nothing is wrong with.
func (h Hasher) Verify(password, encoded string) (Verification, error) {
	matches, stored, err := argon2id.CheckHash(password, encoded)
	if err != nil {
		if isMalformedHash(err) {
			return Verification{}, ErrPasswordHashMalformed
		}
		return Verification{}, fmt.Errorf("verify password: %w", err)
	}
	return Verification{
		Matches:     matches,
		NeedsRehash: matches && paramsFromLibrary(stored) != h.params,
	}, nil
}

// DummyHash is a valid PHC-encoded Argon2id hash with default parameters (m=64MB, t=3, p=2).
// It is used by DummyVerify to consume constant CPU time when an email does not exist,
// ensuring login response timing for an unknown email is indistinguishable from a wrong password (BR-AUTH-02).
const DummyHash = "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQxMjM0NTY3OA$q1W2e3R4t5Y6U7i8O9p0Q1W2e3R4t5Y6U7i8O9p0Q1W"

// DummyVerify performs a full Argon2id verification against a static dummy hash.
// This consumes constant work and returns Matches = false without throwing errors,
// satisfying timing equalisation (BR-AUTH-02).
func (h Hasher) DummyVerify(password string) (Verification, error) {
	_, _, _ = argon2id.CheckHash(password, DummyHash)
	return Verification{Matches: false, NeedsRehash: false}, nil
}

// isMalformedHash separates "this string is not a hash we can read" from any
// other failure, so only the former is translated into a domain error.
func isMalformedHash(err error) bool {
	return errors.Is(err, argon2id.ErrInvalidHash) ||
		errors.Is(err, argon2id.ErrIncompatibleVariant) ||
		errors.Is(err, argon2id.ErrIncompatibleVersion)
}
