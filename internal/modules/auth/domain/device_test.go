package domain_test

import (
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// caseEmpty names the table row several suites in this package share. It is a
// constant because goconst counts three uses as one waiting to be extracted, and
// because "" is what half of them are actually testing.
const caseEmpty = "empty"

// TestDeviceLabel_ReadsRealUserAgents is the whole reason this function has a
// table rather than a chain of Contains calls, and the reason the table's order
// is load-bearing.
//
// Every Chrome user agent also contains "Safari". Edge contains "Chrome" and
// "Safari" both. Chrome on iOS contains neither "Chrome" nor "Edg" but does
// contain "Safari". A matcher walked in the wrong order therefore labels most of
// the web "Safari", and the learner reading their device list to decide what to
// revoke is told the wrong thing about every one of them. These are real strings
// for that reason.
func TestDeviceLabel_ReadsRealUserAgents(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		agent string
		want  string
	}{
		"Chrome on macOS": {
			agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
				"(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
			want: "Chrome on macOS",
		},
		"Safari on macOS": {
			agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) Version/17.6 Safari/605.1.15",
			want: "Safari on macOS",
		},
		"Edge on Windows": {
			agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
				"(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0",
			want: "Edge on Windows",
		},
		"Firefox on Windows": {
			agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:129.0) Gecko/20100101 Firefox/129.0",
			want:  "Firefox on Windows",
		},
		"Chrome on Android": {
			agent: "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 " +
				"(KHTML, like Gecko) Chrome/128.0.0.0 Mobile Safari/537.36",
			want: "Chrome on Android",
		},
		"Safari on iPhone": {
			agent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1",
			want: "Safari on iPhone",
		},
		"Chrome on iPhone reports itself as CriOS": {
			agent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 " +
				"(KHTML, like Gecko) CriOS/128.0.0.0 Mobile/15E148 Safari/604.1",
			want: "Chrome on iPhone",
		},
		"Firefox on Linux": {
			agent: "Mozilla/5.0 (X11; Linux x86_64; rv:129.0) Gecko/20100101 Firefox/129.0",
			want:  "Firefox on Linux",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			label := domain.DeviceLabel(testCase.agent)
			if label == nil {
				t.Fatalf("no label for %q", testCase.agent)
			}
			if *label != testCase.want {
				t.Errorf("label = %q, want %q", *label, testCase.want)
			}
		})
	}
}

// TestDeviceLabel_PrefersNothingToAGuess pins the direction. A learner deciding
// whether to revoke a session has to be able to trust what the row says: an
// absent label they can reason about, a confidently wrong one they cannot.
func TestDeviceLabel_PrefersNothingToAGuess(t *testing.T) {
	t.Parallel()

	unreadable := map[string]string{
		caseEmpty:          "",
		"whitespace":       "   ",
		"a bare token":     "some-internal-client/1.0",
		"curl":             "curl/8.7.1",
		"our own test rig": "integration-suite",
	}
	for name, agent := range unreadable {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if label := domain.DeviceLabel(agent); label != nil {
				t.Errorf("%s produced the label %q, want none", name, *label)
			}
		})
	}
}

// TestDeviceLabel_IsBoundedAndCarriesNoVersion covers what must *not* be in it.
// A version string turns a coarse description into something closer to a
// fingerprint, and an unbounded label is a column an attacker-supplied header
// gets to size.
func TestDeviceLabel_IsBoundedAndCarriesNoVersion(t *testing.T) {
	t.Parallel()

	agent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

	label := domain.DeviceLabel(agent)
	if label == nil {
		t.Fatal("no label")
	}
	if strings.ContainsAny(*label, "0123456789") {
		t.Errorf("label %q carries a version", *label)
	}
	if len(*label) > domain.MaxDeviceLabelLength {
		t.Errorf("label is %d bytes, over the %d the column allows", len(*label), domain.MaxDeviceLabelLength)
	}

	// A header a caller chose cannot make the stored value any longer than the
	// vocabulary allows, because the label is built from the table's own words
	// and never from the input.
	hostile := strings.Repeat("Chrome on Windows ", 500)
	if long := domain.DeviceLabel(hostile); long != nil && len(*long) > domain.MaxDeviceLabelLength {
		t.Errorf("a %d-byte user agent produced a %d-byte label", len(hostile), len(*long))
	}
}
