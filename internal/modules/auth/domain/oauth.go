package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/shared/apperr"
)

// ProviderGoogle is the only provider this version supports. Apple waits for an
// iOS app, where the App Store requires it.
const ProviderGoogle = "google"

// OAuthState is one in-flight authorization request.
//
// The PKCE verifier is a digest here and nowhere a plaintext: the row exists to
// prove this server started the flow, not to hold anything that completes one.
type OAuthState struct {
	ID               uuid.UUID
	State            string
	Provider         string
	Nonce            string
	PKCEVerifierHash []byte
	RedirectTo       *string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	ConsumedAt       *time.Time
}

// OAuthIdentity is a provider account attached to a local one.
type OAuthIdentity struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Provider  string
	Subject   string
	EmailHash []byte
	LinkedAt  time.Time
}

// The refusals this flow owns, as declared in this module AGENT.md §12.
var (
	// ErrOAuthStateInvalid covers missing, unknown, expired and already-used.
	// One code for all four: they are the shapes a CSRF attempt takes, and
	// which one a probe tripped is information about our validation.
	ErrOAuthStateInvalid = apperr.New(
		apperr.Validation, "OAUTH_STATE_INVALID", "That sign-in attempt is no longer valid. Start again.")

	// ErrOAuthEmailUnverified is a provider declining to vouch for the address.
	// An address a provider will not assert is worth no more than one typed
	// into a form (BR-AUTH-15).
	ErrOAuthEmailUnverified = apperr.New(
		apperr.Forbidden, "OAUTH_EMAIL_UNVERIFIED", "Google has not verified that email address.")

	// ErrOAuthAccountConflict is the account-takeover refusal (BR-AUTH-16): the
	// address matches a local account that has never proved it. Anyone can
	// claim an address by typing it into the signup form, so linking here would
	// hand the account to whoever claimed it first.
	ErrOAuthAccountConflict = apperr.New(
		apperr.Conflict, "OAUTH_ACCOUNT_CONFLICT",
		"An unverified account already uses that email. Verify it first, then link Google.")

	// ErrOAuthEmailMismatch is linking a Google account whose address is not
	// the one this account holds.
	ErrOAuthEmailMismatch = apperr.New(
		apperr.Conflict, "OAUTH_EMAIL_MISMATCH", "That Google account uses a different email address.")

	// ErrOAuthAlreadyLinked is one Google identity reaching for a second
	// Fluentra account. One to one, or signing in with Google identifies
	// nobody in particular.
	ErrOAuthAlreadyLinked = apperr.New(
		apperr.Conflict, "OAUTH_ALREADY_LINKED", "That Google account is already linked to an account.")

	// ErrOAuthTokenInvalid is an ID token failing signature, iss, aud, exp or
	// nonce. One code for all five, and nothing is written for any of them
	// (BR-AUTH-18).
	ErrOAuthTokenInvalid = apperr.New(
		apperr.Unauthenticated, "TOKEN_INVALID", "Google's response could not be verified.")

	// ErrOAuthProviderUnavailable is not being able to reach Google at all.
	//
	// It is deliberately not ErrOAuthTokenInvalid. Telling a learner their
	// sign-in was invalid when the truth is that we could not ask sends them to
	// re-check a Google account that is working perfectly, or to reset a
	// password that is not the problem. A 503 says "try again shortly", which is
	// both true and actionable.
	ErrOAuthProviderUnavailable = apperr.New(
		apperr.Unavailable, "DEPENDENCY_UNAVAILABLE", "Google sign-in is unavailable right now. Try again shortly.")

	// ErrLastSignInMethod refuses to leave an account with no way in
	// (BR-AUTH-20). A Google-only account has no password, so unlinking would
	// lock the learner out with no recovery path -- forgot-password cannot help
	// somebody who has no password to reset.
	ErrLastSignInMethod = apperr.New(
		apperr.Conflict, "LAST_SIGN_IN_METHOD",
		"That is the only way to sign in to this account. Set a password first.")
)

// StateBytes is the entropy in an OAuth `state`. It is the CSRF token for the
// whole flow, so it is drawn the same way and at the same width as everything
// else here that an attacker would otherwise guess.
const StateBytes = 32

// PKCE is a verifier and the challenge derived from it.
//
// The verifier goes to the token endpoint at exchange time and the challenge
// goes to the authorization endpoint at the start, which is the whole mechanism:
// whoever intercepts the authorization code cannot exchange it without the
// verifier, and the verifier never travelled over the channel the code did.
type PKCE struct {
	Verifier  string
	Challenge string
}

// PKCEFor derives the verifier for a state, and its S256 challenge.
//
// **The verifier is derived rather than drawn**, and that is what makes the
// exchange possible at all. The verifier has to be sent to Google at callback
// time, minutes after the authorization request that produced it, on whichever
// instance the browser happens to land on. A drawn value would have to be kept
// somewhere for that gap — and the row keeps only a digest, from which nothing
// can be recovered. Deriving it from the `state` means the callback recomputes
// the verifier from the row it already loaded, and nothing has to be stored.
//
// The security properties are the ones a drawn verifier had. The state carries
// 256 bits from crypto/rand, the HMAC is keyed with the server key, and neither
// the key nor the verifier is in the table — so a dump of `core.oauth_states`
// still contains nothing that completes a flow, which is the property the
// column was shaped around. What it costs is that the key now protects two
// things instead of one; what it buys is a flow with no plaintext at rest.
//
// S256 and not `plain`. RFC 7636 permits `plain`, and it is worth nothing: it
// sends the verifier itself to the authorization endpoint, so anyone who can see
// the authorization request can complete the exchange — which is the attack PKCE
// exists to stop.
func (k Keyring) PKCEFor(state string) PKCE {
	mac := hmac.New(sha256.New, k.key)
	mac.Write([]byte(pkceDerivationLabel))
	mac.Write([]byte{0})
	mac.Write([]byte(state))

	// Base64url of 32 bytes is 43 characters, the shortest RFC 7636 §4.1 allows
	// and 256 bits of it.
	verifier := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(sum[:])}
}

// pkceDerivationLabel domain-separates this MAC from the others the same key
// computes. Without it, a value that reached both SubjectHash and PKCEFor would
// produce related outputs, and one derivation would leak the other.
const pkceDerivationLabel = "pkce-verifier"

// HashPKCEVerifier digests a verifier for storage.
//
// Only the digest is stored. The verifier itself is reconstructed by nobody: it
// is handed back to Google at exchange time from the row's own record, and a
// dump of the table then contains nothing that completes a flow.
func HashPKCEVerifier(verifier string) []byte {
	sum := sha256.Sum256([]byte(verifier))
	return sum[:]
}

// NewOAuthState draws a `state` or a `nonce`. They are the same shape and the
// same width; what differs is where each one is checked.
func NewOAuthState(entropy io.Reader) (string, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	raw := make([]byte, StateBytes)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("draw oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// LinkOutcome is what the callback should do with a verified Google identity.
type LinkOutcome int

const (
	// LinkSignIn is an identity already attached to an account.
	LinkSignIn LinkOutcome = iota

	// LinkToVerified attaches the identity to a local account that has proved
	// the same address, then signs in.
	LinkToVerified

	// LinkRefuseUnverified is the branch this whole policy exists for. See
	// DecideLink.
	LinkRefuseUnverified

	// LinkCreateAccount is a Google account with no local counterpart.
	LinkCreateAccount

	// LinkRefuseUnverifiedProvider is Google declining to assert the address.
	LinkRefuseUnverifiedProvider
)

// LinkInput is everything the decision depends on. It is a struct rather than
// four parameters so a caller cannot silently transpose two booleans.
type LinkInput struct {
	// ProviderEmailVerified is Google's `email_verified` claim.
	ProviderEmailVerified bool
	// IdentityKnown is whether (provider, subject) already names an account.
	IdentityKnown bool
	// LocalAccountExists is whether the asserted address matches a local one.
	LocalAccountExists bool
	// LocalAccountVerified is whether that local account has proved the address.
	LocalAccountVerified bool
}

// DecideLink is the five-branch policy from ADR-0023, as a pure function.
//
// It is separate from the code that performs the linking for one reason: the
// third branch is an account-takeover path, and a policy expressed as nested
// conditionals inside a function that also writes rows is a policy nobody can
// read off the page. Here the branches are exhaustive, ordered, and testable
// without a database or a network.
//
// The order matters as much as the branches:
//
//   - **An unverified provider email is refused first**, before anything is
//     matched against. An address a provider will not vouch for is worth no more
//     than an address typed into a form, and matching on it would let anyone
//     with a Google account claiming any address walk into the branches below
//     (BR-AUTH-15).
//   - **A known identity signs in** without consulting the address at all. The
//     link already exists; the subject is the identity, and re-deriving it from
//     an email that Google may since have reassigned would be a way to lose an
//     account.
//   - **A verified local match links.** Both sides have proved the same address,
//     so they are the same person.
//   - **An unverified local match is refused (BR-AUTH-16).** This is the one
//     that looks unfriendly and is not negotiable. Registration does not prove
//     an address — anyone can type one in — so an unverified local account is a
//     *claim* on that address, not ownership of it. Auto-linking would mean:
//     somebody registers a stranger's address and never verifies it, the real
//     owner later signs in with Google, and the account they land in is the one
//     the attacker created and still knows the password to. Refusing costs the
//     honest learner one OTP; auto-linking costs somebody their account.
//   - **No local account creates one, already verified**, because Google has
//     performed exactly the verification the OTP would have (BR-AUTH-19).
func DecideLink(input LinkInput) LinkOutcome {
	switch {
	case !input.ProviderEmailVerified:
		return LinkRefuseUnverifiedProvider
	case input.IdentityKnown:
		return LinkSignIn
	case input.LocalAccountExists && input.LocalAccountVerified:
		return LinkToVerified
	case input.LocalAccountExists:
		return LinkRefuseUnverified
	default:
		return LinkCreateAccount
	}
}
