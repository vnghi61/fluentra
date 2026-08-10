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
