package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
)

const (
	wordSame   = "same"
	wordSchol  = "schol"
	wordSchool = "school"
	wordCat    = "cat"
	wordBanana = "banana"
)

func TestOSADistance(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{wordSame, wordSame, 0},
		{"Same", wordSame, 0},
		// Transposition of adjacent characters: 1
		{"form", "from", 1},
		{"recieve", "receive", 1},
		// Insertion: 1
		{wordSchol, wordSchool, 1},
		// Deletion: 1
		{wordSchool, wordSchol, 1},
		// Substitution: 1
		{wordCat, "bat", 1},
		// Multi-edit
		{wordCat, "dog", 3},
		{"", "abc", 3},
		{"abc", "", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.expected, domain.OSADistance(tt.a, tt.b))
		})
	}
}

func TestIsWithinSpellingBound(t *testing.T) {
	tests := []struct {
		term      string
		candidate string
		expected  bool
	}{
		// < 5 letters: bound is 1
		{"form", "from", true},    // len 4, dist 1 <= 1
		{wordCat, "bat", true},    // len 3, dist 1 <= 1
		{wordCat, "dog", false},   // len 3, dist 3 > 1
		{"look", "lookk", true},   // len 4, dist 1 <= 1
		{"look", "loookk", false}, // len 4, dist 2 > 1

		// >= 5 letters: bound is 2
		{wordSchol, wordSchool, true}, // len 5, dist 1 <= 2
		{"recieve", "receive", true},  // len 7, dist 1 <= 2
		{wordBanana, "banan", true},   // len 6, dist 1 <= 2
		{wordBanana, "bannaa", true},  // len 6, dist 2 <= 2
		{wordBanana, "apple", false},  // len 6, dist 5 > 2
		{wordSchool, "different", false},
	}

	for _, tt := range tests {
		t.Run(tt.term+"_"+tt.candidate, func(t *testing.T) {
			assert.Equal(t, tt.expected, domain.IsWithinSpellingBound(tt.term, tt.candidate))
		})
	}
}
