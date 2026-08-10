package domain

import (
	"context"
	"strings"
)

// MinPasswordLength is the floor from SECURITY_GUIDELINE §2 and BR-AUTH-01, and
// the default when a caller supplies no PASSWORD_MIN_LENGTH.
const MinPasswordLength = 12

// MaxPasswordLength bounds what is handed to the key derivation. There is no
// security reason for an upper limit — Argon2id absorbs any length — but there
// is an availability one: hashing an unbounded body at 64 MiB per call is a
// cheap way for an anonymous caller to exhaust memory on the registration
// endpoint. 128 characters is far above any passphrase a learner will type.
const MaxPasswordLength = 128

// Policy decides whether a password may be used.
//
// It holds the three checks the card and BR-AUTH-01 name — length, similarity
// to the email address, and presence in a breach corpus — and nothing else. It
// is a value, so a zero Policy is usable and applies the defaults.
type Policy struct {
	// MinLength is PASSWORD_MIN_LENGTH. Zero means MinPasswordLength.
	MinLength int

	// Breaches is the corpus check. A nil checker skips it, which is what
	// BREACHED_PASSWORD_CHECK=false means: the composition root passes nothing
	// rather than passing a checker that has been told to say no.
	Breaches BreachChecker
}

// Validate reports whether password may be set for the account at email.
//
// The breach check is the only part that can fail for a reason that is not the
// password's fault, and when it does the password is **allowed**. That is the
// deliberate choice the card calls "fail open": Have I Been Pwned being slow or
// down is our outage, and refusing to let learners register during it trades a
// probabilistic improvement in password strength for a total loss of service.
// The adapter logs the failure at warn; nothing else here reacts to it.
//
// The three rejections share one error code on purpose. A distinct
// "this password is in a breach corpus" would tell whoever submitted it
// something true about a credential they may not own.
func (p Policy) Validate(ctx context.Context, password, email string) error {
	if err := p.checkShape(password, email); err != nil {
		return err
	}
	if p.Breaches == nil {
		return nil
	}
	breached, err := p.Breaches.Breached(ctx, password)
	if err != nil {
		return nil //nolint:nilerr // failing open is the documented behaviour, see above
	}
	if breached {
		return passwordViolation()
	}
	return nil
}

// checkShape is everything Validate can decide without leaving the process. It
// is separate so the pure rules can be exercised — and reasoned about — without
// a checker, and so Validate reads as "shape, then corpus".
func (p Policy) checkShape(password, email string) error {
	// Runes, not bytes. A 12-character Vietnamese passphrase is more than 12
	// bytes, and counting bytes would let it through a check it should pass
	// while rejecting nothing — the error is in the lenient direction, which is
	// the one worth catching.
	length := len([]rune(password))
	if length < p.minLength() || length > MaxPasswordLength {
		return passwordViolation()
	}

	// Equal to the local part of the address, case-insensitively. This is the
	// single most guessable password an attacker who knows the email can try,
	// and it is the one the card names.
	if local := localPart(email); local != "" && strings.EqualFold(password, local) {
		return passwordViolation()
	}
	return nil
}

func (p Policy) minLength() int {
	if p.MinLength <= 0 {
		return MinPasswordLength
	}
	return p.MinLength
}

// localPart returns everything before the last '@'. The last, not the first: an
// address may quote an '@' in its local part, and splitting on the first would
// hand back a fragment that matches nothing.
func localPart(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return ""
	}
	return strings.TrimSpace(email[:at])
}
