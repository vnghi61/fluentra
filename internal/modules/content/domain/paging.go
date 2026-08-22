package domain

import "math"

// Page size bounds, matching the OpenAPI schema for GET /content and the same
// numbers audit/domain uses for its own list endpoints.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// NormaliseLimit clamps a requested page size into [1, MaxLimit]. Zero or
// negative means "not supplied", which is the documented default.
//
// The clamp returns int32 because that is what the generated query takes, and
// clamping before the conversion is what makes the conversion safe: `limit`
// arrives from strconv.Atoi over a query string, so on a 64-bit platform it can
// hold a value no int32 can. Converting first and range-checking afterwards
// reads the truncated number, and 4294967297 silently becomes a page of 1.
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

// NormaliseOffset clamps a requested offset into [0, math.MaxInt32]. There is
// no product ceiling on how deep a caller may page — a page past the end is
// simply empty — but the value still has to fit the column type.
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
