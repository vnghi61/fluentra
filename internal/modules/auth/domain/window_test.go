package domain_test

import (
	"testing"
	"time"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// TestWindowsFor_AnAdminNeverReceivesTheExtendedWindow is one of the two things
// P2.9's card singles out as easy to get wrong, and it names the way it gets
// wrong: sharing one code path between the role check and the trusted check.
//
// The case that matters is the last one. An administrator who ticks "remember
// this device" must still get twelve hours and seven days — the flag is a
// learner's convenience, and an admin session is worth more and is used from
// fewer places. An implementation that applies the trusted window first and the
// role afterwards, or that reads `trusted` before returning for an admin, passes
// every other row in this table.
func TestWindowsFor_AnAdminNeverReceivesTheExtendedWindow(t *testing.T) {
	t.Parallel()

	config := domain.DefaultWindowConfig()

	cases := map[string]struct {
		role         string
		trusted      bool
		wantIdle     time.Duration
		wantAbsolute time.Duration
	}{
		"learner, untrusted": {
			role: domain.RoleUser, trusted: false,
			wantIdle: domain.DefaultIdleWindow, wantAbsolute: domain.DefaultAbsoluteTTL,
		},
		"learner, trusted": {
			role: domain.RoleUser, trusted: true,
			wantIdle: domain.DefaultIdleWindowTrusted, wantAbsolute: domain.DefaultAbsoluteTTL,
		},
		"admin, untrusted": {
			role: domain.RoleAdmin, trusted: false,
			wantIdle: domain.DefaultIdleWindowAdmin, wantAbsolute: domain.DefaultAbsoluteTTLAdmin,
		},
		"admin, asking to be remembered": {
			role: domain.RoleAdmin, trusted: true,
			wantIdle: domain.DefaultIdleWindowAdmin, wantAbsolute: domain.DefaultAbsoluteTTLAdmin,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			windows := domain.WindowsFor(testCase.role, testCase.trusted, config)
			if windows.Idle != testCase.wantIdle {
				t.Errorf("idle = %s, want %s", windows.Idle, testCase.wantIdle)
			}
			if windows.Absolute != testCase.wantAbsolute {
				t.Errorf("absolute = %s, want %s", windows.Absolute, testCase.wantAbsolute)
			}
		})
	}
}

// TestWindowsFor_TrustingNeverTouchesTheAbsoluteCap is the other half of the
// same rule. A learner can consent to a longer idle window; they cannot consent
// their way out of the cap, because the cap is what bounds a theft and the
// device asking may be the one an attacker is holding.
func TestWindowsFor_TrustingNeverTouchesTheAbsoluteCap(t *testing.T) {
	t.Parallel()

	config := domain.DefaultWindowConfig()
	untrusted := domain.WindowsFor(domain.RoleUser, false, config)
	trusted := domain.WindowsFor(domain.RoleUser, true, config)

	if trusted.Absolute != untrusted.Absolute {
		t.Errorf("trusting changed the absolute cap: %s -> %s", untrusted.Absolute, trusted.Absolute)
	}
	if trusted.Idle <= untrusted.Idle {
		t.Errorf("trusting did not lengthen the idle window: %s -> %s", untrusted.Idle, trusted.Idle)
	}
	if trusted.Idle > trusted.Absolute {
		t.Errorf("the trusted idle window (%s) outlives the cap (%s), which makes the cap unreachable",
			trusted.Idle, trusted.Absolute)
	}
}

// TestWindowConfig_ZeroMeansTheDefaultAndNeverImmediateExpiry pins the direction
// a missing environment variable fails in. A window of zero is a session that
// has already ended when it is created, and it would surface as every learner
// being signed out on their next request with nothing in the logs to say why.
func TestWindowConfig_ZeroMeansTheDefaultAndNeverImmediateExpiry(t *testing.T) {
	t.Parallel()

	for name, role := range map[string]string{"learner": domain.RoleUser, "admin": domain.RoleAdmin} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			windows := domain.WindowsFor(role, true, domain.WindowConfig{})
			if windows.Idle <= 0 || windows.Absolute <= 0 {
				t.Fatalf("an empty config produced %+v, which expires on creation", windows)
			}
		})
	}
}

// TestClampToAbsolute is the line that turns sliding-with-no-cap into the design
// ADR-0022 actually chose. It is invisible on the happy path — a rotation well
// inside the cap is unaffected — which is exactly why it is tested directly
// rather than only through the sessions that use it.
func TestClampToAbsolute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 12, 9, 0, 0, 0, time.UTC)
	absolute := now.Add(48 * time.Hour)

	// Inside the cap: untouched, which is the ordinary case and must stay cheap
	// and exact.
	inside := now.Add(time.Hour)
	if got := domain.ClampToAbsolute(inside, absolute); !got.Equal(inside) {
		t.Errorf("a renewal inside the cap was moved: %s -> %s", inside, got)
	}

	// Past it: cut back to the cap, never beyond.
	beyond := now.Add(72 * time.Hour)
	if got := domain.ClampToAbsolute(beyond, absolute); !got.Equal(absolute) {
		t.Errorf("a renewal past the cap was allowed to outlive it: %s, want %s", got, absolute)
	}

	// Exactly on it: the boundary is inclusive, so a renewal landing precisely
	// on the cap is not silently extended by a nanosecond.
	if got := domain.ClampToAbsolute(absolute, absolute); !got.Equal(absolute) {
		t.Errorf("a renewal exactly at the cap = %s, want %s", got, absolute)
	}
}
