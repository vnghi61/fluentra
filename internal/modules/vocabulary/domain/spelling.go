package domain

import (
	"strings"
	"unicode/utf8"
)

// OSADistance calculates the Optimal String Alignment distance (restricted Damerau-Levenshtein
// distance) between two strings, compared case-insensitively.
//
// Allowed operations:
// - Insertion (cost 1)
// - Deletion (cost 1)
// - Substitution (cost 1)
// - Transposition of two adjacent characters (cost 1)
// Substrings cannot be edited more than once.
func OSADistance(s1, s2 string) int {
	r1 := []rune(strings.ToLower(strings.TrimSpace(s1)))
	r2 := []rune(strings.ToLower(strings.TrimSpace(s2)))
	len1 := len(r1)
	len2 := len(r2)

	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	d := make([][]int, len1+1)
	for i := range d {
		d[i] = make([]int, len2+1)
		d[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		d[0][j] = j
	}

	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			// min of deletion, insertion, substitution
			delCost := d[i-1][j] + 1
			insCost := d[i][j-1] + 1
			subCost := d[i-1][j-1] + cost

			minCost := delCost
			if insCost < minCost {
				minCost = insCost
			}
			if subCost < minCost {
				minCost = subCost
			}
			d[i][j] = minCost

			// Transposition of adjacent characters
			if i > 1 && j > 1 && r1[i-1] == r2[j-2] && r1[i-2] == r2[j-1] {
				transCost := d[i-2][j-2] + 1
				if transCost < d[i][j] {
					d[i][j] = transCost
				}
			}
		}
	}

	return d[len1][len2]
}

// IsWithinSpellingBound checks whether candidate is a close spelling of term.
// Bound: distance <= 1 for terms under 5 letters; distance <= 2 for terms >= 5 letters.
func IsWithinSpellingBound(term, candidate string) bool {
	cleanTerm := strings.TrimSpace(term)
	cleanCand := strings.TrimSpace(candidate)
	if cleanTerm == "" || cleanCand == "" {
		return false
	}
	n := utf8.RuneCountInString(cleanTerm)
	bound := 2
	if n < 5 {
		bound = 1
	}
	return OSADistance(cleanTerm, cleanCand) <= bound
}
