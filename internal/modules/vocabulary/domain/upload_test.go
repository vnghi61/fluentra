package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
)

// What a learner actually pastes: a column out of a spreadsheet, a list they
// typed, a page from a handout. The parser has to find the words in all of it
// and stay dull — a parser that guesses wrong turns somebody's own vocabulary
// list into somebody else's, and they cannot tell until the exercises are odd.

func TestParseUpload_ReadsTheSeparatorsPeopleActuallyUse(t *testing.T) {
	for name, line := range map[string]string{
		"tab":       "leisure\tthời gian rảnh",
		"dash":      "leisure - thời gian rảnh",
		"en dash":   "leisure – thời gian rảnh",
		"em dash":   "leisure — thời gian rảnh",
		"colon":     "leisure: thời gian rảnh",
		"equals":    "leisure = thời gian rảnh",
		"semicolon": "leisure;thời gian rảnh",
		"pipe":      "leisure|thời gian rảnh",
	} {
		t.Run(name, func(t *testing.T) {
			entries := domain.ParseUpload(line)
			require.Len(t, entries, 1)
			assert.Equal(t, "leisure", entries[0].Term)
			assert.Equal(t, "thời gian rảnh", entries[0].Meaning)
		})
	}
}

func TestParseUpload_AcceptsABareWordList(t *testing.T) {
	// The commonest kind of paste there is. Demanding a meaning would reject it.
	entries := domain.ParseUpload("leisure\nhabit\njourney")

	require.Len(t, entries, 3)
	for _, entry := range entries {
		assert.Empty(t, entry.Meaning)
	}
	assert.Equal(t, "leisure", entries[0].Term)
}

func TestParseUpload_StripsBulletsAndNumbering(t *testing.T) {
	entries := domain.ParseUpload("- leisure\n* habit\n1. journey\n2) afford\n• breathe")

	require.Len(t, entries, 5)
	terms := make([]string, 0, len(entries))
	for _, entry := range entries {
		terms = append(terms, entry.Term)
	}
	assert.Equal(t, []string{"leisure", "habit", "journey", "afford", "breathe"}, terms)
}

func TestParseUpload_KeepsAMeaningThatContainsADash(t *testing.T) {
	// Tab is tried before the hand-typed marks precisely so this works: the
	// meaning owns the dash, and splitting on it would truncate the meaning.
	entries := domain.ParseUpload("leisure\tfree time - not working")

	require.Len(t, entries, 1)
	assert.Equal(t, "leisure", entries[0].Term)
	assert.Equal(t, "free time - not working", entries[0].Meaning)
}

func TestParseUpload_DropsDuplicatesCaseInsensitively(t *testing.T) {
	// The unique constraint would drop the second copy anyway; doing it here
	// means the count the learner is shown matches what was stored, and that
	// they are not paid XP twice for one word.
	entries := domain.ParseUpload("habit\nHabit\nHABIT - thói quen\njourney")

	require.Len(t, entries, 2)
	assert.Equal(t, "habit", entries[0].Term)
	assert.Equal(t, "journey", entries[1].Term)
}

func TestParseUpload_SkipsLinesWithNoWordInThem(t *testing.T) {
	// Page numbers, dividers and blank lines come with a pasted handout.
	// Submitting them would spend a dictionary call and a model call to be told
	// they are not words.
	entries := domain.ParseUpload("leisure\n\n   \n---\n42\n===\n7.\nhabit")

	require.Len(t, entries, 2)
	assert.Equal(t, "leisure", entries[0].Term)
	assert.Equal(t, "habit", entries[1].Term)
}

func TestParseUpload_KeepsTheLearnersOwnOrder(t *testing.T) {
	entries := domain.ParseUpload("zebra\napple\nmiddle")

	require.Len(t, entries, 3)
	assert.Equal(t, "zebra", entries[0].Term)
	assert.Equal(t, "apple", entries[1].Term)
	assert.Equal(t, "middle", entries[2].Term)
}

func TestParseUpload_StopsAtTheEntryLimit(t *testing.T) {
	// Every entry costs a dictionary call and a model call. One learner's paste
	// of ten thousand words would occupy the job for hours while everybody
	// else's upload waited behind it.
	var lines []string
	for i := 0; i < domain.MaxUploadEntries+50; i++ {
		lines = append(lines, "word"+strings.Repeat("x", i%5)+itoa(i))
	}

	entries := domain.ParseUpload(strings.Join(lines, "\n"))
	assert.Len(t, entries, domain.MaxUploadEntries)
}

func TestParseUpload_EmptyInputYieldsNothing(t *testing.T) {
	assert.Empty(t, domain.ParseUpload(""))
	assert.Empty(t, domain.ParseUpload("   \n\n\t\n"))
}

func TestParseUpload_HandlesWindowsLineEndings(t *testing.T) {
	// Pasting from a Windows editor brings \r\n, and a term with a trailing
	// carriage return matches no dictionary entry at all.
	entries := domain.ParseUpload("leisure\r\nhabit\r\n")

	require.Len(t, entries, 2)
	assert.Equal(t, "leisure", entries[0].Term)
	assert.Equal(t, "habit", entries[1].Term)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
