package service

import "github.com/fluentra/fluentra/internal/modules/vocabulary/domain"

// OSADistance delegates to domain.OSADistance.
func OSADistance(s1, s2 string) int {
	return domain.OSADistance(s1, s2)
}

// IsWithinSpellingBound delegates to domain.IsWithinSpellingBound.
func IsWithinSpellingBound(term, candidate string) bool {
	return domain.IsWithinSpellingBound(term, candidate)
}
