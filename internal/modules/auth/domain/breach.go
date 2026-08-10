package domain

import (
	"context"
	"crypto/sha1" //nolint:gosec // see PasswordRange: SHA-1 is the HIBP range API's wire format, not a security primitive here
	"encoding/hex"
	"strconv"
	"strings"
)

// BreachChecker reports whether a password appears in a public breach corpus.
//
// It is declared here, next to the policy that consumes it, rather than beside
// the implementation: the domain owns what it needs, and the adapter that
// speaks to Have I Been Pwned is one of possibly several things that could
// satisfy it — an offline Bloom filter is the fallback the module's AGENT.md
// §11 already anticipates.
//
// An implementation reports failure by returning an error. It must not turn a
// failure into `false` itself, because the policy has to be able to tell "not
// breached" from "could not tell" even though it treats them the same way.
type BreachChecker interface {
	Breached(ctx context.Context, password string) (bool, error)
}

// RangePrefixLength is the number of hex characters of the SHA-1 digest that
// are sent to the range API. Five is the k-anonymity parameter Have I Been
// Pwned defines, and it is the only part of the digest that may leave the
// process — the remaining 35 characters are matched locally against the bucket
// the server returns.
const RangePrefixLength = 5

// PasswordRange splits the SHA-1 of password into the part that is sent and the
// part that is kept.
//
// This function is the whole of the k-anonymity guarantee, which is why it is
// here and pure rather than inlined into the HTTP adapter: a test can assert
// that prefix is exactly five characters and that prefix+suffix reconstructs
// the digest, and the adapter has nothing else to build a URL out of.
//
// SHA-1 is not a security choice. The range API is defined over SHA-1 and the
// digest is never stored, compared for authenticity, or used to derive
// anything; a stronger hash here would simply not match the corpus.
func PasswordRange(password string) (prefix, suffix string) {
	digest := sha1.Sum([]byte(password)) //nolint:gosec // wire format of the HIBP range API, see above
	encoded := strings.ToUpper(hex.EncodeToString(digest[:]))
	return encoded[:RangePrefixLength], encoded[RangePrefixLength:]
}

// ParseRangeResponse finds suffix in a Have I Been Pwned range response and
// returns the number of times that password appears in the corpus, or zero if
// the bucket does not contain it.
//
// The response is a `SUFFIX:COUNT` line per entry. Everything about the parse is
// deliberately forgiving except the match itself: the server has changed line
// endings and letter case in the past, and a padded response mixes in synthetic
// entries whose counts are zero. A line that cannot be read is skipped rather
// than failing the check, because one malformed row in a bucket of several
// hundred is not a reason to reject a password the learner chose.
func ParseRangeResponse(body, suffix string) int {
	wanted := strings.ToUpper(strings.TrimSpace(suffix))
	for line := range strings.SplitSeq(body, "\n") {
		candidate, count, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(candidate), wanted) {
			continue
		}
		occurrences, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil {
			// The suffix is in the bucket; only its count is unreadable. That
			// is still a hit, and treating it as one is the safe direction.
			return 1
		}
		return occurrences
	}
	return 0
}
