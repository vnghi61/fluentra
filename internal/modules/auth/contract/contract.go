package contract

import (
	"time"

	"github.com/google/uuid"
)

// Aggregate is the outbox aggregate name every event below is written under.
const Aggregate = "auth"

// Event names, declared fully qualified.
//
// The full form is what a consumer subscribes to and what `Event.Topic()`
// reassembles; the outbox writer strips the aggregate prefix before storing it.
// Writing them bare here would mean two spellings of one wire value.
const (
	// EventVerificationRequested asks for a verification code to be delivered.
	// It is the outbox row registration writes in the same transaction as the
	// account, so a rolled-back registration cannot send a code.
	EventVerificationRequested = "auth.verification_requested"

	// EventRegistrationAttempted asks for the "someone tried to register with
	// your address" warning. It is what an already-verified address gets
	// instead of a code, and it is why registration cannot be used to discover
	// whether an address is registered.
	EventRegistrationAttempted = "auth.registration_attempted"
)

// VerificationRequested carries what the mailer needs to send a code.
//
// It carries the code in plaintext, and that is worth being explicit about.
// There is nowhere else for it: the challenge row stores only an HMAC, so by
// the time a consumer runs, the code cannot be recovered from anything but this
// payload. The exposure is bounded by the code's own ten-minute life — after
// that the payload is worthless — but `ops.outbox_events` currently keeps
// published rows, so the row outlives its usefulness. Pruning them is filed in
// internal/modules/auth/TODO.md; it is a change to shared/outbox, not to this
// event.
//
// Field names follow the convention `audit` reads structurally, so that if this
// topic is ever added to its subscription list it produces a usable entry
// without `audit` importing anything from here.
type VerificationRequested struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Locale      string    `json:"locale"`
	Code        string    `json:"code"`
	ExpiresAt   time.Time `json:"expires_at"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// RegistrationAttempted is the warning sent to somebody whose address is
// already registered and verified.
//
// It carries no code and no challenge id: there is nothing for the recipient to
// verify, and the point of the message is to tell them that somebody else tried.
type RegistrationAttempted struct {
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	Locale     string    `json:"locale"`
	OccurredAt time.Time `json:"occurred_at"`
}
