package domain_test

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// TestDecideLink_AnUnverifiedLocalMatchIsRefused is the account-takeover path
// this card exists to close, and it is written before the linking code that
// would otherwise get it wrong by being friendly.
//
// Registration does not prove an address. Anyone can type a stranger's address
// into the signup form, so an unverified local account is a *claim* on that
// address rather than ownership of it. If Google sign-in auto-linked to it, the
// sequence is: an attacker registers the victim's address and never verifies it,
// the victim later signs in with Google, and the account they land in is the
// attacker's — same rows, same password, now containing the victim's work.
//
// Refusing costs an honest learner one OTP. Auto-linking costs somebody their
// account. The test is first because "it seems friendlier" is a real pull.
func TestDecideLink_AnUnverifiedLocalMatchIsRefused(t *testing.T) {
	t.Parallel()

	outcome := domain.DecideLink(domain.LinkInput{
		ProviderEmailVerified: true,
		IdentityKnown:         false,
		LocalAccountExists:    true,
		LocalAccountVerified:  false,
	})

	if outcome != domain.LinkRefuseUnverified {
		t.Fatalf("outcome = %v, want a refusal: an unverified local account is a claim on an "+
			"address, not proof of it, and linking to one hands the account to whoever claimed it first",
			outcome)
	}
}

// TestDecideLink_TheFiveBranches is ADR-0023's policy as a table. Exhaustive on
// purpose: every combination that can reach this function has a row, so a branch
// added later without a decision shows up as a missing case rather than as
// whatever the `default` happens to do.
func TestDecideLink_TheFiveBranches(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input domain.LinkInput
		want  domain.LinkOutcome
	}{
		"identity already linked": {
			input: domain.LinkInput{ProviderEmailVerified: true, IdentityKnown: true},
			want:  domain.LinkSignIn,
		},
		"verified local match": {
			input: domain.LinkInput{
				ProviderEmailVerified: true, LocalAccountExists: true, LocalAccountVerified: true,
			},
			want: domain.LinkToVerified,
		},
		"unverified local match": {
			input: domain.LinkInput{ProviderEmailVerified: true, LocalAccountExists: true},
			want:  domain.LinkRefuseUnverified,
		},
		"no local account": {
			input: domain.LinkInput{ProviderEmailVerified: true},
			want:  domain.LinkCreateAccount,
		},
		"google will not vouch for the address": {
			input: domain.LinkInput{ProviderEmailVerified: false},
			want:  domain.LinkRefuseUnverifiedProvider,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := domain.DecideLink(testCase.input); got != testCase.want {
				t.Errorf("outcome = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestDecideLink_AnUnverifiedProviderEmailIsRefusedBeforeAnythingIsMatched pins
// the ordering, which carries as much of the policy as the branches do.
//
// An address Google will not vouch for is worth no more than one typed into a
// form. If it were matched against local accounts first, anyone able to make
// Google emit an unverified claim for an arbitrary address could walk into the
// link-to-verified branch and take the account.
func TestDecideLink_AnUnverifiedProviderEmailIsRefusedBeforeAnythingIsMatched(t *testing.T) {
	t.Parallel()

	// Every combination that would otherwise succeed, with the one claim that
	// matters set to false.
	for name, input := range map[string]domain.LinkInput{
		"even with a known identity": {
			ProviderEmailVerified: false, IdentityKnown: true,
		},
		"even with a verified local match": {
			ProviderEmailVerified: false, LocalAccountExists: true, LocalAccountVerified: true,
		},
		"even with no local account at all": {
			ProviderEmailVerified: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := domain.DecideLink(input); got != domain.LinkRefuseUnverifiedProvider {
				t.Errorf("outcome = %v, want the unverified-provider refusal", got)
			}
		})
	}
}

// TestPKCEFor_ProducesAnS256ChallengeOverAFullWidthVerifier checks the pair a
// caller actually has to get right, and checks it the way Google will.
func TestPKCEFor_ProducesAnS256ChallengeOverAFullWidthVerifier(t *testing.T) {
	t.Parallel()

	state, err := domain.NewOAuthState(nil)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}
	pkce := newKeyring(t).PKCEFor(state)

	// RFC 7636 §4.1: 43–128 characters.
	if len(pkce.Verifier) < 43 || len(pkce.Verifier) > 128 {
		t.Errorf("verifier is %d characters, outside the 43–128 RFC 7636 allows", len(pkce.Verifier))
	}
	if strings.ContainsAny(pkce.Verifier, "+/=") {
		t.Errorf("verifier %q is not base64url without padding", pkce.Verifier)
	}

	// The challenge is S256 over the verifier, which is what Google recomputes.
	// `plain` would put the verifier itself in the authorization request, where
	// whoever intercepts the code can also read it — the attack PKCE exists to
	// stop.
	sum := sha256.Sum256([]byte(pkce.Verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); pkce.Challenge != want {
		t.Errorf("challenge = %q, want the S256 digest %q", pkce.Challenge, want)
	}
	if pkce.Challenge == pkce.Verifier {
		t.Error("the challenge equals the verifier, which is the `plain` method and is worth nothing")
	}
}

// TestPKCEFor_RecomputesTheSameVerifierFromTheSameState is the property the
// whole flow rests on, and the reason the verifier is derived rather than drawn.
//
// The authorization request and the token exchange are minutes and one redirect
// apart, and may not even reach the same instance. Nothing carries the verifier
// across that gap: the row stores a digest and the client is never told it. The
// callback recomputes it from the `state` it just consumed, so this equality is
// what makes the exchange possible — if it ever stops holding, every Google
// sign-in fails at Google with an opaque `invalid_grant`.
func TestPKCEFor_RecomputesTheSameVerifierFromTheSameState(t *testing.T) {
	t.Parallel()

	keys := newKeyring(t)
	state, err := domain.NewOAuthState(nil)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}

	atStart := keys.PKCEFor(state)
	atCallback := keys.PKCEFor(state)

	if atStart.Verifier != atCallback.Verifier {
		t.Error("the same state derived two different verifiers, so no exchange could ever complete")
	}
	if atStart.Challenge != atCallback.Challenge {
		t.Error("the same state derived two different challenges")
	}
}

// TestPKCEFor_DerivesADifferentVerifierForEveryState pins the other half: one
// flow's verifier must be worthless in another, or intercepting a single code
// and verifier pair would unlock every subsequent flow.
func TestPKCEFor_DerivesADifferentVerifierForEveryState(t *testing.T) {
	t.Parallel()

	keys := newKeyring(t)
	seen := make(map[string]struct{}, 128)
	for range 128 {
		state, err := domain.NewOAuthState(nil)
		if err != nil {
			t.Fatalf("NewOAuthState: %v", err)
		}
		for _, value := range []string{keys.PKCEFor(state).Verifier, state} {
			if _, repeated := seen[value]; repeated {
				t.Fatal("a verifier or state repeated")
			}
			seen[value] = struct{}{}
		}
	}

	// A starved entropy source must fail rather than produce something shorter.
	// A short state is a guessable CSRF token — and now also a guessable
	// verifier, since one derives the other.
	if _, err := domain.NewOAuthState(strings.NewReader("too few")); err == nil {
		t.Error("a state was drawn from an entropy source that could not fill it")
	}
}

// TestPKCEFor_IsKeyedSoTheStateAloneDoesNotYieldTheVerifier is what keeps the
// stored row worthless to whoever reads it.
//
// The `state` column is stored in the clear, because it is the lookup key. If
// the verifier were a plain digest of it, anybody with read access to the table
// — or with a state observed in a redirect URL — could derive the verifier and
// complete an intercepted exchange, which is precisely what PKCE exists to
// prevent. The server key is what stands between those two facts.
func TestPKCEFor_IsKeyedSoTheStateAloneDoesNotYieldTheVerifier(t *testing.T) {
	t.Parallel()

	state, err := domain.NewOAuthState(nil)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}

	other, err := domain.NewKeyring([]byte("a-different-key-also-at-least-32-bytes"))
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}

	if newKeyring(t).PKCEFor(state).Verifier == other.PKCEFor(state).Verifier {
		t.Error("two different server keys derived one verifier, so the key is not in the derivation")
	}

	// And the state itself must not be recoverable from, or visible in, what is
	// derived from it.
	pkce := newKeyring(t).PKCEFor(state)
	if strings.Contains(pkce.Verifier, state) || strings.Contains(state, pkce.Verifier) {
		t.Error("the verifier and the state contain one another")
	}
}

// TestHashPKCEVerifier_IsTheDigestTheRowStores keeps the stored value and the
// value handed back to Google derived from one function, so they cannot drift.
//
// The column is no longer the only record of the verifier — the derivation is —
// but it is still what the callback checks its recomputation against, which
// turns "the key or the derivation changed" into a refused sign-in here rather
// than an opaque rejection at Google.
func TestHashPKCEVerifier_IsTheDigestTheRowStores(t *testing.T) {
	t.Parallel()

	state, err := domain.NewOAuthState(nil)
	if err != nil {
		t.Fatalf("NewOAuthState: %v", err)
	}
	pkce := newKeyring(t).PKCEFor(state)

	digest := domain.HashPKCEVerifier(pkce.Verifier)
	if len(digest) != 32 {
		t.Errorf("digest is %d bytes, want the 32 the CHECK constraint requires", len(digest))
	}
	if strings.Contains(string(digest), pkce.Verifier) {
		t.Error("the digest contains the verifier")
	}
}
