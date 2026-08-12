package domain

import "time"

// The default windows, mirroring the SESSION_* keys in `.env.example` and the
// numbers ADR-0022 settled on.
const (
	DefaultIdleWindow        = 30 * 24 * time.Hour
	DefaultIdleWindowTrusted = 90 * 24 * time.Hour
	DefaultAbsoluteTTL       = 180 * 24 * time.Hour

	// An administrator's account is worth more than a learner's and is used
	// from fewer places, so it gets neither extension. Twelve hours of
	// inactivity ends the session and seven days ends it regardless.
	DefaultIdleWindowAdmin  = 12 * time.Hour
	DefaultAbsoluteTTLAdmin = 7 * 24 * time.Hour
)

// Windows are the two lifetimes a session has.
//
// They are separate because they answer different questions. Idle asks "have we
// seen you lately"; absolute asks "how long has it been since you proved you are
// you". Activity moves the first and never the second, and that asymmetry is the
// entire security argument for the idle window being as long as it is
// (BR-AUTH-22, ADR-0022 alternative C).
type Windows struct {
	Idle     time.Duration
	Absolute time.Duration
}

// WindowConfig is the configured set, one entry per case.
type WindowConfig struct {
	Idle          time.Duration
	IdleTrusted   time.Duration
	Absolute      time.Duration
	IdleAdmin     time.Duration
	AbsoluteAdmin time.Duration
}

// DefaultWindowConfig mirrors the SESSION_* block of `.env.example`.
func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		Idle:          DefaultIdleWindow,
		IdleTrusted:   DefaultIdleWindowTrusted,
		Absolute:      DefaultAbsoluteTTL,
		IdleAdmin:     DefaultIdleWindowAdmin,
		AbsoluteAdmin: DefaultAbsoluteTTLAdmin,
	}
}

// WithDefaults fills anything a caller left zero.
//
// Zero must not mean "expires immediately", which is what a partially populated
// config would otherwise produce: a session that has already ended when it is
// created signs every learner out on their next request, and the cause looks
// like a token bug rather than a missing environment variable.
func (c WindowConfig) WithDefaults() WindowConfig {
	defaults := DefaultWindowConfig()
	if c.Idle <= 0 {
		c.Idle = defaults.Idle
	}
	if c.IdleTrusted <= 0 {
		c.IdleTrusted = defaults.IdleTrusted
	}
	if c.Absolute <= 0 {
		c.Absolute = defaults.Absolute
	}
	if c.IdleAdmin <= 0 {
		c.IdleAdmin = defaults.IdleAdmin
	}
	if c.AbsoluteAdmin <= 0 {
		c.AbsoluteAdmin = defaults.AbsoluteAdmin
	}
	return c
}

// WindowsFor picks the pair a new session gets.
//
// **An administrator returns before `trusted` is ever read.** That early return
// is the whole point of this function existing separately from the code that
// creates sessions: the card and ADR-0022 both single out "an admin session must
// not receive the extended window" as the thing easiest to get wrong, and the
// way it gets got wrong is one code path with `if trusted { ... }` layered on
// top of a role check somebody later reorders. Here the two cases cannot
// interleave, and the test asserts an admin asking to be remembered still gets
// twelve hours.
//
// Trusting is opt-in and buys a longer idle window only. It does not touch the
// absolute cap, because the cap is what bounds a theft and a learner cannot
// consent their way out of it on a device an attacker may be holding.
func WindowsFor(role string, trusted bool, config WindowConfig) Windows {
	config = config.WithDefaults()

	if role == RoleAdmin {
		return Windows{Idle: config.IdleAdmin, Absolute: config.AbsoluteAdmin}
	}

	idle := config.Idle
	if trusted {
		idle = config.IdleTrusted
	}
	return Windows{Idle: idle, Absolute: config.Absolute}
}

// ClampToAbsolute is what makes a sliding window stop sliding.
//
// A rotation issues a replacement whose idle window starts again from now. Left
// alone that is ADR-0022's rejected alternative C: a stolen token used regularly
// renews itself forever, the theft becomes permanent, and nothing ever notices.
// The clamp is the one line that turns sliding-with-no-cap into the design that
// was actually chosen, and it is invisible on the happy path — which is why its
// test is written first.
func ClampToAbsolute(idleExpiry, absoluteExpiry time.Time) time.Time {
	if idleExpiry.After(absoluteExpiry) {
		return absoluteExpiry
	}
	return idleExpiry
}
