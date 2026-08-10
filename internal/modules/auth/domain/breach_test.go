package domain_test

import (
	"crypto/sha1" //nolint:gosec // the test asserts the wire format, see domain.PasswordRange
	"encoding/hex"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/internal/modules/auth/domain"
)

// sampleSuffix is a real-shaped 35-character suffix. The parse tests care about
// the format, not about which password it belongs to, so one value serves them
// all and never has to correspond to a digest.
const sampleSuffix = "00D4F6E8FA6EECAD2A3AA415EEC418D38EC"

// otherEntry is a second, unrelated bucket line, so the tests exercise a parse
// that has to skip past something before it finds what it wants.
const otherEntry = "0018A45C4D1DEF81644B54AB7F969B88D65:1"

// TestPasswordRange_SendsFiveCharactersAndKeepsTheRest is the acceptance
// criterion "only the first 5 characters of the SHA-1 hash ever leave the
// system", asserted at the only place the split is made. The HTTP adapter has
// nothing else to build a URL from, so if it holds here, it holds there.
func TestPasswordRange_SendsFiveCharactersAndKeepsTheRest(t *testing.T) {
	prefix, suffix := domain.PasswordRange(correctPassword)

	if len(prefix) != domain.RangePrefixLength {
		t.Errorf("prefix = %q, want %d characters", prefix, domain.RangePrefixLength)
	}
	if len(suffix) != 35 {
		t.Errorf("suffix is %d characters, want 35", len(suffix))
	}

	digest := sha1.Sum([]byte(correctPassword)) //nolint:gosec // the format under test
	wanted := strings.ToUpper(hex.EncodeToString(digest[:]))
	if prefix+suffix != wanted {
		t.Errorf("prefix+suffix = %q, want the full digest %q", prefix+suffix, wanted)
	}
}

// TestPasswordRange_IsUppercaseHex matters because the API is case-sensitive on
// the path and the corpus is published in upper case. A lower-case prefix
// returns a 404 rather than a wrong answer, which would surface as the check
// failing open for every password — a failure that looks exactly like success.
func TestPasswordRange_IsUppercaseHex(t *testing.T) {
	prefix, suffix := domain.PasswordRange(correctPassword)

	for _, part := range []string{prefix, suffix} {
		if strings.ToUpper(part) != part {
			t.Errorf("%q is not upper case", part)
		}
		if strings.Trim(part, "0123456789ABCDEF") != "" {
			t.Errorf("%q contains something that is not hex", part)
		}
	}
}

func TestPasswordRange_DiffersBetweenPasswords(t *testing.T) {
	firstPrefix, firstSuffix := domain.PasswordRange("one passphrase here")
	secondPrefix, secondSuffix := domain.PasswordRange("another passphrase!")

	if firstPrefix == secondPrefix && firstSuffix == secondSuffix {
		t.Error("two different passwords produced the same digest")
	}
}

func TestParseRangeResponse_FindsTheSuffixAndReturnsItsCount(t *testing.T) {
	body := strings.Join([]string{
		otherEntry,
		sampleSuffix + ":2",
		"011053FD0102E94D6AE2F8B83D76FAF94F6:3",
	}, "\r\n")

	if count := domain.ParseRangeResponse(body, sampleSuffix); count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

// TestParseRangeResponse_ReturnsZeroForASuffixTheBucketDoesNotHold is the
// ordinary case: a bucket always comes back full, and almost none of it is the
// password being checked.
func TestParseRangeResponse_ReturnsZeroForASuffixTheBucketDoesNotHold(t *testing.T) {
	body := otherEntry + "\r\n" + sampleSuffix + ":2"

	if count := domain.ParseRangeResponse(body, "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"); count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

// TestParseRangeResponse_ToleratesTheFormatVariationsTheAPIHasShown covers what
// the endpoint has actually returned over time: bare newlines, trailing blank
// lines, and lower-case suffixes.
func TestParseRangeResponse_ToleratesTheFormatVariationsTheAPIHasShown(t *testing.T) {
	bodies := map[string]string{
		"unix line endings":     otherEntry + "\n" + sampleSuffix + ":7\n",
		"windows line endings":  otherEntry + "\r\n" + sampleSuffix + ":7\r\n",
		"a trailing blank line": otherEntry + "\r\n" + sampleSuffix + ":7\r\n\r\n",
		"a lower-case suffix":   strings.ToLower(sampleSuffix) + ":7",
		"surrounding spaces":    "  " + sampleSuffix + " : 7  ",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			if count := domain.ParseRangeResponse(body, sampleSuffix); count != 7 {
				t.Errorf("count = %d, want 7", count)
			}
		})
	}
}

// TestParseRangeResponse_IgnoresTheZeroCountPaddingEntries is the Add-Padding
// header working as intended. The padding rows are real suffixes with a count
// of zero, so they must parse and must not read as a hit.
func TestParseRangeResponse_IgnoresTheZeroCountPaddingEntries(t *testing.T) {
	body := sampleSuffix + ":0\r\n" + otherEntry

	if count := domain.ParseRangeResponse(body, sampleSuffix); count != 0 {
		t.Errorf("count = %d, want 0 for a padding entry", count)
	}
}

func TestParseRangeResponse_SkipsLinesItCannotRead(t *testing.T) {
	body := "this line has no colon\r\n\r\n" + sampleSuffix + ":9"

	if count := domain.ParseRangeResponse(body, sampleSuffix); count != 9 {
		t.Errorf("count = %d, want 9", count)
	}
}

// TestParseRangeResponse_TreatsAnUnreadableCountAsAHit is the safe direction:
// the suffix is present, so the password is in the corpus regardless of whether
// the occurrence count can be read.
func TestParseRangeResponse_TreatsAnUnreadableCountAsAHit(t *testing.T) {
	if count := domain.ParseRangeResponse(sampleSuffix+":lots", sampleSuffix); count <= 0 {
		t.Errorf("count = %d, want a positive count", count)
	}
}

func TestParseRangeResponse_ReturnsZeroForAnEmptyBody(t *testing.T) {
	if count := domain.ParseRangeResponse("", sampleSuffix); count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
