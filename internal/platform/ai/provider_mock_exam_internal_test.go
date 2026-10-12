package ai

import "testing"

// TestParseMockExamFormat_BoundsTheCounts: the counts come from prompt text and
// size slices, so an absurd one is clamped rather than allocated.
func TestParseMockExamFormat_BoundsTheCounts(t *testing.T) {
	f := parseMockExamFormat("Questions in this group: exactly 999999999\nOptions per question: exactly 400")
	if f.questions != maxMockQuestions || f.options != maxMockOptions {
		t.Fatalf("questions=%d options=%d, want %d and %d", f.questions, f.options, maxMockQuestions, maxMockOptions)
	}
	if got := len(mockExamQuestions(f)); got != maxMockQuestions {
		t.Errorf("built %d questions, want %d", got, maxMockQuestions)
	}
	if got := len(mockLetterOptions(-3)); got != 0 {
		t.Errorf("a negative option count built %d options, want 0", got)
	}

	ordinary := parseMockExamFormat("Questions in this group: exactly 3\nOptions per question: exactly 4")
	if ordinary.questions != 3 || ordinary.options != 4 {
		t.Errorf("ordinary format parsed as %d questions, %d options", ordinary.questions, ordinary.options)
	}
}
