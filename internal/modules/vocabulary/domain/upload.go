package domain

import (
	"strings"
	"unicode"
)

// Parsing what a learner pasted.
//
// The input is whatever was in their clipboard: a column copied out of a
// spreadsheet, a list they typed, a page from a course handout. The parser's
// job is to find the words in it and not to be clever — an aggressive parser
// that guesses wrong turns a learner's own vocabulary list into somebody else's,
// and they have no way to tell it is wrong until the exercises are strange.
//
// So: one entry per line, split on the first separator that appears, and no
// attempt at anything else.

// UploadEntry is one word a learner submitted, and what they said it means.
type UploadEntry struct {
	Term string
	// Meaning may be empty. A learner pasting a bare word list is submitting
	// words to learn, not claims to check, and demanding a meaning would reject
	// the commonest kind of paste there is.
	Meaning string
}

// The separators a line may use, in the order they are tried.
//
// Tab first because a spreadsheet column pastes as tab-separated and a meaning
// may itself contain a dash or a colon. Then the marks people type by hand.
var uploadSeparators = []string{"\t", " - ", " – ", " — ", " = ", ": ", " : ", "=", ";", "|"}

// MaxUploadEntries bounds one paste.
//
// Not a technical limit — the column is capped at 100 kB — but a rate one
// learner's verification can proceed at. Every entry costs a dictionary call
// and a model call, and a paste of ten thousand words would occupy the job for
// hours while everybody else's uploads waited behind it.
const MaxUploadEntries = 300

// ParseUpload turns pasted text into entries.
//
// Blank lines and duplicates are dropped, and the order is the learner's own so
// the list they see back reads like the list they sent. Duplicates are compared
// case-insensitively: "Habit" and "habit" are one word, and the database's
// unique constraint would silently drop the second anyway — doing it here means
// the count the learner is shown matches what was stored.
func ParseUpload(raw string) []UploadEntry {
	seen := make(map[string]bool)
	entries := make([]UploadEntry, 0, 16)

	for _, line := range strings.Split(raw, "\n") {
		entry, ok := parseUploadLine(line)
		if !ok {
			continue
		}
		key := strings.ToLower(entry.Term)
		if seen[key] {
			continue
		}
		seen[key] = true

		entries = append(entries, entry)
		if len(entries) >= MaxUploadEntries {
			break
		}
	}
	return entries
}

// parseUploadLine reads one line, or reports that there is nothing in it.
func parseUploadLine(line string) (UploadEntry, bool) {
	// Numbered lists paste as "1. habit" or "1) habit", and a learner should
	// not have to strip that by hand.
	line = strings.TrimSpace(strings.TrimLeft(line, "-*•\t "))
	line = trimLeadingOrdinal(line)
	if line == "" {
		return UploadEntry{}, false
	}

	term, meaning := line, ""
	for _, separator := range uploadSeparators {
		before, after, found := strings.Cut(line, separator)
		if !found {
			continue
		}
		before, after = strings.TrimSpace(before), strings.TrimSpace(after)
		// A separator at the very start or end splits nothing useful: "- habit"
		// has already had its bullet trimmed, and "habit -" is a word with a
		// stray mark after it.
		if before == "" {
			continue
		}
		term, meaning = before, after
		break
	}

	term = strings.TrimSpace(strings.Trim(term, ".,;:"))
	if term == "" || !hasLetter(term) {
		// A line of digits or punctuation is a page number or a divider, not a
		// word. Submitting it would spend a model call to be told so.
		return UploadEntry{}, false
	}
	return UploadEntry{Term: term, Meaning: strings.TrimSpace(meaning)}, true
}

// trimLeadingOrdinal removes "12." or "3)" from the start of a line.
func trimLeadingOrdinal(line string) string {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits >= len(line) {
		return line
	}
	switch line[digits] {
	case '.', ')', ':':
		return strings.TrimSpace(line[digits+1:])
	default:
		return line
	}
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
