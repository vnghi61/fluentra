package domain_test

import (
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// testParams are deliberately far cheaper than the production ones. Every test
// in this file is about the *shape* of the result — does it verify, are the
// parameters in the string, is a rehash asked for — and none of them is about
// how long the derivation takes. Hashing at 64 MiB three times per case would
// add minutes to the unit suite and prove nothing extra.
//
// The one thing that must still be tested at the real parameters is that the
// real parameters are what DefaultHashParams says, and that is asserted
// directly rather than by paying for a derivation.
var testParams = domain.HashParams{
	MemoryKiB:   8,
	Iterations:  1,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

const correctPassword = "correct horse battery staple"

func TestDefaultHashParams_MatchTheSecurityGuideline(t *testing.T) {
	params := domain.DefaultHashParams()

	// SECURITY_GUIDELINE §2 and the ARGON2_* keys in .env.example: m=64 MiB,
	// t=3, p=2. This assertion is the reason those three numbers cannot drift
	// downward unnoticed — a weakened default is invisible in every other test.
	if params.MemoryKiB != 64*1024 {
		t.Errorf("memory = %d KiB, want 65536", params.MemoryKiB)
	}
	if params.Iterations != 3 {
		t.Errorf("iterations = %d, want 3", params.Iterations)
	}
	if params.Parallelism != 2 {
		t.Errorf("parallelism = %d, want 2", params.Parallelism)
	}
}

func TestHash_EmbedsTheParametersInTheEncodedHash(t *testing.T) {
	hasher := domain.NewHasher(testParams)

	encoded, err := hasher.Hash(correctPassword)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// The parameters travelling inside the string is what makes raising them a
	// constant change plus rehash-on-login instead of a migration.
	wanted := "$argon2id$v=19$m=8,t=1,p=1$"
	if !strings.HasPrefix(encoded, wanted) {
		t.Errorf("hash = %q, want it to start with %q", encoded, wanted)
	}
	if strings.Contains(encoded, correctPassword) {
		t.Error("the encoded hash contains the plaintext password")
	}
}

func TestHash_IsSaltedSoTheSamePasswordHashesDifferently(t *testing.T) {
	hasher := domain.NewHasher(testParams)

	first, err := hasher.Hash(correctPassword)
	if err != nil {
		t.Fatalf("first Hash: %v", err)
	}
	second, err := hasher.Hash(correctPassword)
	if err != nil {
		t.Fatalf("second Hash: %v", err)
	}

	if first == second {
		t.Error("two hashes of one password are identical, so the salt is not random")
	}
}

func TestVerify_AcceptsTheRightPasswordAndAsksForNoRehash(t *testing.T) {
	hasher := domain.NewHasher(testParams)
	encoded := mustHash(t, hasher, correctPassword)

	result, err := hasher.Verify(correctPassword, encoded)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Matches {
		t.Error("the correct password did not verify")
	}
	if result.NeedsRehash {
		t.Error("a hash made with the current parameters was marked for rehashing")
	}
}

// TestVerify_RejectsWrongPasswordsRegardlessOfSharedPrefix is the guard against
// the class of bug the constant-time requirement exists to prevent. It cannot
// measure timing — a wall-clock assertion is flaky on shared CI hardware, and
// the actual constant-time property comes from crypto/subtle.ConstantTimeCompare
// inside argon2id.CheckHash. What it can do is fail loudly if anyone ever puts a
// cheaper comparison in front of the derivation: a prefix check, a length check
// or an equality shortcut would all let at least one of these cases through, or
// return a different error for it.
func TestVerify_RejectsWrongPasswordsRegardlessOfSharedPrefix(t *testing.T) {
	hasher := domain.NewHasher(testParams)
	encoded := mustHash(t, hasher, correctPassword)

	wrong := map[string]string{
		"nothing in common":     "totally different passphrase",
		"differs in the last":   correctPassword[:len(correctPassword)-1] + "X",
		"differs in the first":  "X" + correctPassword[1:],
		"is a strict prefix":    correctPassword[:len(correctPassword)-1],
		"has a trailing space":  correctPassword + " ",
		"differs only in case":  strings.ToUpper(correctPassword),
		"is the empty password": "",
	}
	for name, candidate := range wrong {
		t.Run(name, func(t *testing.T) {
			result, err := hasher.Verify(candidate, encoded)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if result.Matches {
				t.Errorf("%q verified against the hash of %q", candidate, correctPassword)
			}
		})
	}
}

// TestVerify_FlagsAHashMadeWithSupersededParameters is the acceptance criterion
// "a hash created with old parameters is transparently upgraded on successful
// login", at the point where the decision is made. The write that follows it is
// covered by the repository integration suite.
func TestVerify_FlagsAHashMadeWithSupersededParameters(t *testing.T) {
	old := testParams
	raised := testParams
	raised.Iterations = testParams.Iterations + 1

	encoded := mustHash(t, domain.NewHasher(old), correctPassword)

	result, err := domain.NewHasher(raised).Verify(correctPassword, encoded)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Matches {
		t.Fatal("raising the parameters stopped an existing password from verifying")
	}
	if !result.NeedsRehash {
		t.Error("a hash made at t=1 was not flagged for rehashing by a t=2 hasher")
	}
}

// TestVerify_DoesNotAskForARehashOnAWrongPassword matters because the caller
// uses NeedsRehash to decide whether to write. Acting on it without a match
// would store a hash of whatever the attacker typed.
func TestVerify_DoesNotAskForARehashOnAWrongPassword(t *testing.T) {
	old := testParams
	raised := testParams
	raised.Iterations = testParams.Iterations + 1

	encoded := mustHash(t, domain.NewHasher(old), correctPassword)

	result, err := domain.NewHasher(raised).Verify("not the password", encoded)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Matches || result.NeedsRehash {
		t.Errorf("result = %+v, want no match and no rehash", result)
	}
}

// TestVerify_EveryParameterChangeTriggersARehash covers the fields individually,
// because comparing the whole struct is what makes that true and a future
// refactor comparing only memory and iterations would silently strand every
// hash whose parallelism or key length changed.
func TestVerify_EveryParameterChangeTriggersARehash(t *testing.T) {
	encoded := mustHash(t, domain.NewHasher(testParams), correctPassword)

	changed := map[string]domain.HashParams{
		"memory":      {MemoryKiB: 16, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		"iterations":  {MemoryKiB: 8, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		"parallelism": {MemoryKiB: 8, Iterations: 1, Parallelism: 2, SaltLength: 16, KeyLength: 32},
		"salt length": {MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 24, KeyLength: 32},
		"key length":  {MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 64},
	}
	for name, params := range changed {
		t.Run(name, func(t *testing.T) {
			result, err := domain.NewHasher(params).Verify(correctPassword, encoded)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if !result.Matches {
				t.Fatal("the password stopped verifying")
			}
			if !result.NeedsRehash {
				t.Errorf("changing %s did not ask for a rehash", name)
			}
		})
	}
}

// TestVerify_MalformedStoredHashIsAnErrorNotAMismatch keeps a corrupt row from
// looking like a wrong password. The two need different handling: one locks a
// learner out of an account that is fine, the other is a normal login failure.
func TestVerify_MalformedStoredHashIsAnErrorNotAMismatch(t *testing.T) {
	hasher := domain.NewHasher(testParams)

	malformed := map[string]string{
		caseEmpty:            "",
		"not a hash at all":  "hunter2",
		"a bcrypt hash":      "$2y$10$abcdefghijklmnopqrstuv",
		"the wrong variant":  "$argon2i$v=19$m=8,t=1,p=1$c29tZXNhbHQ$c29tZWtleQ",
		"an unknown version": "$argon2id$v=16$m=8,t=1,p=1$c29tZXNhbHQ$c29tZWtleQ",
	}
	for name, encoded := range malformed {
		t.Run(name, func(t *testing.T) {
			result, err := hasher.Verify(correctPassword, encoded)
			if err == nil {
				t.Fatalf("Verify returned %+v and no error", result)
			}
			if !apperr.Is(err, apperr.Internal) {
				t.Errorf("error = %v, want an internal error", err)
			}
		})
	}
}

func mustHash(t *testing.T, hasher domain.Hasher, password string) string {
	t.Helper()
	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	return encoded
}
