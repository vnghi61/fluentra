package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxDeviceLabelLength bounds what is stored, matching the schema's own limit
// on the field in `SessionSummary`.
const MaxDeviceLabelLength = 80

// Session is one signed-in device.
//
// There is no IP address member and there is no column for one. `IPHash` is a
// keyed digest, which answers "is this the same origin as last time" without
// the table becoming a movement log — and being keyed rather than plain is what
// stops a reader recovering the address by hashing all four billion of them.
type Session struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	DeviceLabel   *string
	IPHash        []byte
	UserAgentHash []byte
	CreatedAt     time.Time
	LastSeenAt    time.Time
	RevokedAt     *time.Time

	// AbsoluteExpiresAt is set once at sign-in and never moved. Rotation may
	// slide the idle window right up to it and no further (BR-AUTH-22).
	AbsoluteExpiresAt time.Time

	// TrustedDeviceID is set only for a session opened on a device the learner
	// chose to trust. Nil is the ordinary case.
	TrustedDeviceID *uuid.UUID
}

// AbsolutelyExpiredAt reports whether the session has reached the cap activity
// cannot move.
func (s Session) AbsolutelyExpiredAt(now time.Time) bool {
	return !s.AbsoluteExpiresAt.IsZero() && !now.Before(s.AbsoluteExpiresAt)
}

// TrustedDevice is a device the learner chose to stay signed in on.
type TrustedDevice struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DeviceIDHash   []byte
	Label          *string
	IdleWindow     time.Duration
	AbsoluteExpiry time.Time
	TrustedAt      time.Time
	LastSeenAt     time.Time
	RevokedAt      *time.Time
}

// IdleExpiresAt is when inactivity alone would end the trust. It is derived
// rather than stored, because it moves on every use and a stored copy would be
// a second record of the same fact that could disagree with last_seen_at.
func (d TrustedDevice) IdleExpiresAt() time.Time { return d.LastSeenAt.Add(d.IdleWindow) }

// Revoked reports whether the session has been signed out.
func (s Session) Revoked() bool { return s.RevokedAt != nil }

// The names the table emits. Constants because most appear twice: a browser's
// desktop token and its iOS token are different strings for the same product,
// which is the whole reason the table maps one to the other.
const (
	browserChrome  = "Chrome"
	browserFirefox = "Firefox"
	browserEdge    = "Edge"
	browserSafari  = "Safari"
	browserOpera   = "Opera"
)

// browsers and platforms are matched in order, most specific first.
//
// Order is the whole of the correctness here. Every Chrome user agent also
// contains "Safari", Edge contains both "Chrome" and "Safari", and Chrome on
// iOS contains "CriOS" and "Safari" but not "Chrome" — so a table walked in the
// wrong order labels most of the web "Safari". The tests pin real user agents
// against expected labels for exactly that reason.
var browsers = []struct{ token, name string }{
	{"Edg/", browserEdge},
	{"EdgiOS", browserEdge},
	{"OPR/", browserOpera},
	{"CriOS", browserChrome},
	{"FxiOS", browserFirefox},
	{browserFirefox, browserFirefox},
	{browserChrome, browserChrome},
	{browserSafari, browserSafari},
}

var platforms = []struct{ token, name string }{
	{"iPhone", "iPhone"},
	{"iPad", "iPad"},
	{"Android", "Android"},
	{"Windows", "Windows"},
	{"Macintosh", "macOS"},
	{"Mac OS X", "macOS"},
	{"CrOS", "ChromeOS"},
	{"Linux", "Linux"},
}

// DeviceLabel turns a user agent into something a learner recognises in a list
// of their own devices.
//
// It is deliberately coarse — "Chrome on macOS", never a version and never a
// fingerprint. Two things follow from that. The label is not identity: it does
// not distinguish two Chrome installs on one machine, and it is not what the
// session is keyed on. And it is not tracking data: a bounded set of a few dozen
// values reveals nothing that the learner did not just tell us by connecting.
//
// A user agent this cannot read returns nil rather than a guess. A wrong label
// in a security screen is worse than an absent one — a learner deciding whether
// to revoke a session needs to trust what it says, and "Unknown" they can reason
// about while "Safari on Linux" they cannot.
func DeviceLabel(userAgent string) *string {
	agent := strings.TrimSpace(userAgent)
	if agent == "" {
		return nil
	}

	browser := matchToken(agent, browsers)
	platform := matchToken(agent, platforms)

	var label string
	switch {
	case browser != "" && platform != "":
		label = browser + " on " + platform
	case browser != "":
		label = browser
	case platform != "":
		label = platform
	default:
		return nil
	}

	if len(label) > MaxDeviceLabelLength {
		label = label[:MaxDeviceLabelLength]
	}
	return &label
}

func matchToken(agent string, table []struct{ token, name string }) string {
	for _, entry := range table {
		if strings.Contains(agent, entry.token) {
			return entry.name
		}
	}
	return ""
}
