package contract

import (
	"context"
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

	// EventSecurityEvent reports something worth investigating. `audit` already
	// subscribes to this topic and files it in the security stream rather than
	// the action log.
	EventSecurityEvent = "auth.security_event"

	// EventPasswordResetRequested asks for a reset code to be delivered. Like
	// verification, it carries the code because the challenge row stores only an
	// HMAC and there is nowhere else to recover it from.
	EventPasswordResetRequested = "auth.password_reset_requested"

	// EventPasswordChanged announces that a password was replaced, by either
	// route. It exists so the account's owner is told — a change they did not
	// make is the first sign of a takeover, and the notification is often the
	// only way they find out.
	EventPasswordChanged = "auth.password_changed"
)

// The security event kinds this module raises.
//
// The kind is a payload field rather than a topic of its own because `audit`
// keys its stream on the topic: a kind per topic would mean editing another
// module's subscription list every time this one learns to notice something new.
const (
	// SecurityKindRefreshReuse is a refresh token presented after it was already
	// spent (BR-AUTH-04). It is the strongest signal this module produces: a
	// single-use credential presented twice means two parties hold it.
	SecurityKindRefreshReuse = "refresh_reuse"

	// SecurityKindOAuthStateInvalid is a Google callback carrying a `state` this
	// server did not issue, or already spent, or issued more than ten minutes
	// ago (BR-AUTH-17).
	//
	// It is `medium` where refresh reuse is `high`, and the difference is what
	// each one proves. A replayed refresh token is evidence that a credential
	// leaked. An invalid state is evidence of nothing on its own: a learner who
	// left the consent screen open over lunch, or opened it twice and finished
	// the first tab, produces one without anybody attacking anything. What makes
	// it worth recording is the shape it takes in bulk — a run of them against
	// one address is somebody trying to graft their own Google account onto
	// another learner's session, and that is invisible unless each one is filed.
	SecurityKindOAuthStateInvalid = "oauth_state_invalid"
)

// Severity is how loudly an event asks to be looked at.
//
// The values are duplicated from `audit`'s own rather than imported, which is
// the same trade `audit` makes in the other direction: the wire format is the
// contract between the two modules, and a shared Go type would make one of them
// depend on the other's release cycle for a four-value string.
type Severity string

// The four levels, lowest first. `high` is where a dashboard starts drawing
// attention to something and `critical` is where it pages somebody, so the
// choice between them is the choice of whether to wake a human.
const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
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

// SessionRevoker ends every session an account has.
//
// It is the surface `user` calls when an account is deleted and `admin` calls
// when one is suspended: both need every device signed out, and neither may
// reach into this module to do it. The count is returned because a caller
// recording the action wants to say how many devices were affected.
//
// Access tokens already issued are not revoked by this. The caller is not the
// account's owner and holds none of them; they stop working within one
// access-token lifetime, which is the window ADR-0007 accepted in exchange for
// keeping a datastore read off every request. A suspension that must bite
// sooner needs the denylist, not this method.
type SessionRevoker interface {
	RevokeAll(ctx context.Context, userID uuid.UUID) (int, error)
}

// SecurityEvent is one occurrence worth investigating.
//
// The member names are the convention `audit` reads structurally without
// importing anything from here: `occurred_at`, `user_id` and `severity` are its
// envelope, and everything else — `kind`, `session_id` — falls through into the
// event's detail. Sending a field it does not know about is safe; renaming one
// it does is not.
//
// There is deliberately nothing here that came from the request. An attacker
// who can raise this event would otherwise choose what gets stored in a table
// nobody can UPDATE.
type SecurityEvent struct {
	Kind       string    `json:"kind"`
	Severity   Severity  `json:"severity"`
	UserID     uuid.UUID `json:"user_id"`
	SessionID  uuid.UUID `json:"session_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// PasswordResetRequested carries what the mailer needs to send a reset code.
//
// It carries the code in plaintext for the reason VerificationRequested does,
// and inherits the same open issue: `ops.outbox_events` keeps published rows, so
// the payload outlives the thirty minutes the code is worth anything for. The
// fix belongs in `shared/outbox` and is filed in this module's TODO.
type PasswordResetRequested struct {
	ChallengeID uuid.UUID `json:"challenge_id"`
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Locale      string    `json:"locale"`
	Code        string    `json:"code"`
	ExpiresAt   time.Time `json:"expires_at"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// PasswordChanged announces a completed reset or change.
//
// It carries no password, no hash and no indication of which route was taken.
// The recipient's question is "was this me?", and the answer does not depend on
// whether it came through a reset code or a signed-in change.
type PasswordChanged struct {
	UserID     uuid.UUID `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
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
