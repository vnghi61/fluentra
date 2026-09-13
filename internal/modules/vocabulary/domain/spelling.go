package domain

import (
	"strings"
	"unicode/utf8"
)

// maxSpellingRunes is the longest term OSADistance will align. The matrix is
// (len1+1)×(len2+1) ints and an uploaded term has no other length limit, so an
// unbounded pair is memory a paste can spend. No word worth correcting is this
// long; past it the distance is reported as the longer length, which no
// spelling bound accepts.
const maxSpellingRunes = 64

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
	if len1 > maxSpellingRunes || len2 > maxSpellingRunes {
		return max(len1, len2)
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
			d[i][j] = osaCell(d, r1, r2, i, j)
		}
	}

	return d[len1][len2]
}

// osaCell is one step of the alignment: the cheapest of a deletion, an insertion
// and a substitution, or a transposition of the two runes before it.
func osaCell(d [][]int, r1, r2 []rune, i, j int) int {
	cost := 1
	if r1[i-1] == r2[j-1] {
		cost = 0
	}
	best := min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
	if i > 1 && j > 1 && r1[i-1] == r2[j-2] && r1[i-2] == r2[j-1] {
		best = min(best, d[i-2][j-2]+1)
	}
	return best
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
