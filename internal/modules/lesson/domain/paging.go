package domain

import (
	"math"
	"regexp"
)

// Page size bounds, matching the OpenAPI schema for GET /courses.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	cefrRegex = regexp.MustCompile(`^(A1|A2|B1|B2|C1|C2)$`)
)

// NormaliseLimit clamps a requested page size into [1, MaxLimit]. Zero or
// negative means "not supplied", which defaults to DefaultLimit.
//
// Clamping in int space before narrowing to int32 prevents integer overflow
// vulnerabilities on 64-bit platforms.
func NormaliseLimit(limit int) int32 {
	switch {
	case limit <= 0:
		return DefaultLimit
	case limit > MaxLimit:
		return MaxLimit
	default:
		return int32(limit)
	}
}

// NormaliseOffset clamps a requested offset into [0, math.MaxInt32].
func NormaliseOffset(offset int) int32 {
	switch {
	case offset <= 0:
		return 0
	case offset > math.MaxInt32:
		return math.MaxInt32
	default:
		return int32(offset)
	}
}

// IsValidSlug validates that the given slug is kebab-case.
func IsValidSlug(slug string) bool {
	if len(slug) == 0 || len(slug) > 100 {
		return false
	}
	return slugRegex.MatchString(slug)
}

// IsValidCEFRLevel validates that the given CEFR level is one of A1, A2, B1, B2, C1, C2.
func IsValidCEFRLevel(level string) bool {
	return cefrRegex.MatchString(level)
}
