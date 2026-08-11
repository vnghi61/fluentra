// Package domain holds this module's entities, invariants and pure functions:
// the password policy, the Argon2id hasher, and the parts of the breached-
// password check that involve no I/O. Nothing here opens a socket or a
// transaction, which is what lets the security-critical rules be tested without
// a database, a Redis, or a network.
package domain

import "github.com/fluentra/fluentra/internal/shared/apperr"

// The error codes this module owns, as declared in its AGENT.md §12.
var (
	// ErrPasswordTooWeak covers every way a password can fail policy: too
	// short, equal to the email local part, or present in the breach corpus.
	// One code for all three is deliberate — telling a caller which rule it
	// broke on a breached password would confirm the address is in the corpus.
	ErrPasswordTooWeak = apperr.New(
		apperr.Validation, "PASSWORD_TOO_WEAK", "That password does not meet the password policy.")

	// ErrCredentialNotFound means the account exists but has no password. That
	// is a real state, not a bug: an account created through Google has no
	// credential row until the learner sets one.
	ErrCredentialNotFound = apperr.New(
		apperr.NotFound, "CREDENTIAL_NOT_FOUND", "This account has no password set.")

	// ErrCredentialAlreadyExists is the uq_credentials_user constraint,
	// translated. Registration writing a second credential for one account is a
	// bug in the caller, not something a learner can provoke.
	ErrCredentialAlreadyExists = apperr.New(
		apperr.Conflict, "CREDENTIAL_ALREADY_EXISTS", "This account already has a password.")

	// ErrEmailAlreadyRegistered is the citext unique constraint on core.users,
	// translated. Registration normally never returns it — an address that is
	// already taken goes down an enumeration-safe path instead — so reaching it
	// means two requests raced and the loser's account vanished in between.
	ErrEmailAlreadyRegistered = apperr.New(
		apperr.Conflict, "EMAIL_ALREADY_REGISTERED", "That email address is already registered.")

	// ErrTokenExpired is an access token past its expiry. It is separate from
	// ErrTokenInvalid because the client's next move differs: refresh and
	// retry, rather than send the learner back to the sign-in form.
	ErrTokenExpired = apperr.New(
		apperr.Unauthenticated, "TOKEN_EXPIRED", "Your session has expired. Refresh and try again.")

	// ErrTokenInvalid covers everything else a token can be: malformed, signed
	// with a key we do not hold, for another audience, or missing a claim we
	// require. One code for all of them, because which validation a probe
	// tripped is information about our validation.
	ErrTokenInvalid = apperr.New(
		apperr.Unauthenticated, "TOKEN_INVALID", "That access token is not valid.")

	// ErrSessionRevoked is a token whose id has been denylisted by an explicit
	// logout. The signature is fine and it has not expired — it has been taken
	// away, and the learner has to sign in again.
	ErrSessionRevoked = apperr.New(
		apperr.Unauthenticated, "SESSION_REVOKED", "That session has been signed out.")

	// ErrEmailNotVerified is a correct password on an account that never proved
	// its address.
	//
	// It is 403, not 401, and the module's own AGENT.md §12 has always said so.
	// The credential presented was right; what is missing is permission to use
	// the account at all. Returning 401 — which P2.3 did — tells the client to
	// re-prompt for a password that was never wrong.
	ErrEmailNotVerified = apperr.New(
		apperr.Forbidden, "EMAIL_NOT_VERIFIED", "Verify your email address before signing in.")

	// ErrAccountSuspended is an account an administrator has disabled.
	//
	// It is a distinct code from ACCOUNT_LOCKED, which means "too many failed
	// attempts" and clears itself. These need to be distinguishable by the
	// client: one resolves by waiting, the other only by contacting support,
	// and telling a suspended learner to try again in fifteen minutes is
	// advice that will never come true.
	ErrAccountSuspended = apperr.New(
		apperr.Forbidden, "ACCOUNT_SUSPENDED", "This account has been suspended.")

	// ErrChallengeNotFound is an unknown challenge id. It is also what a caller
	// gets for an id that never existed, which is the same 404 — the id is the
	// secret gating the whole flow (BR-AUTH-11), so distinguishing "wrong id"
	// from "expired and swept" would be a probe.
	ErrChallengeNotFound = apperr.New(
		apperr.NotFound, "CHALLENGE_NOT_FOUND", "That verification request was not found.")

	// ErrChallengeInvalidCode is a wrong code. It carries the remaining attempt
	// count in Meta, because ADR-0021 promises the learner immediate,
	// understandable feedback and a count they can act on.
	ErrChallengeInvalidCode = apperr.New(
		apperr.Unauthenticated, "OTP_INVALID", "That code is not correct.")

	// ErrChallengeExpired means the ten-minute window closed. Separate from a
	// wrong code because the learner's next action is different: request a new
	// challenge, do not re-read the email.
	ErrChallengeExpired = apperr.New(
		apperr.Unauthenticated, "OTP_EXPIRED", "That code has expired. Request a new one.")

	// ErrChallengeAttemptsExceeded is the burned challenge (BR-AUTH-12). It is
	// a 429 rather than a 401: nothing is wrong with the credential presented,
	// the budget for presenting one is spent.
	ErrChallengeAttemptsExceeded = apperr.New(
		apperr.RateLimited, "OTP_ATTEMPTS_EXCEEDED", "Too many incorrect codes. Request a new one.")

	// ErrChallengeAlreadyUsed is a code presented twice. Single use is the
	// whole point of a one-time code.
	ErrChallengeAlreadyUsed = apperr.New(
		apperr.Conflict, "OTP_ALREADY_USED", "That code has already been used.")

	// ErrChallengeResendTooSoon is the 60-second cooldown (BR-AUTH-13).
	ErrChallengeResendTooSoon = apperr.New(
		apperr.RateLimited, "OTP_RESEND_TOO_SOON", "A code was sent recently. Wait before requesting another.")

	// ErrChallengeIssueLimitReached covers both issuance limiters — three per
	// subject per hour and the global per-IP cap. One code for both, because
	// telling a caller which limit they hit tells them how to spread the load
	// to avoid it.
	ErrChallengeIssueLimitReached = apperr.New(
		apperr.RateLimited, "OTP_ISSUE_LIMIT_REACHED", "Too many codes requested. Try again later.")

	// ErrPasswordHashMalformed means the stored string is not a PHC-encoded
	// Argon2id hash. The database CHECK makes this all but unreachable; it
	// exists so the verifier fails loudly instead of returning "no match" for
	// an account nobody can then log in to.
	ErrPasswordHashMalformed = apperr.New(
		apperr.Internal, "PASSWORD_HASH_MALFORMED", "The stored password could not be read.")
)

// passwordViolation builds the 422 with the field name attached. Every policy
// failure goes through it, so the transport layer never chooses a status and
// the client always learns which field to re-prompt.
func passwordViolation() error {
	return ErrPasswordTooWeak.WithFields(apperr.FieldViolation{
		Field:   "password",
		Code:    "PASSWORD_TOO_WEAK",
		Message: "That password does not meet the password policy.",
	})
}
