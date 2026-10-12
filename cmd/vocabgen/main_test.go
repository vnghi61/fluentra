package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSource_FiltersAndDeduplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	body := "# a comment line\n1\tthe\n2\tThe\n3\tstudy\n4\tb\n5\tUnited States\n6\tgo\n7\tgo\n8\tA\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	lemmas, err := readSource(path, 10)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	// "The" folds into "the"; "ab" is too short; the multi-word line is dropped;
	// the duplicate "go" is dropped; "A" is too short.
	want := []sourceLemma{{1, "the"}, {3, "study"}, {6, "go"}}
	if len(lemmas) != len(want) {
		t.Fatalf("lemmas = %v, want %v", lemmas, want)
	}
	for i := range want {
		if lemmas[i] != want[i] {
			t.Errorf("lemma %d = %+v, want %+v", i, lemmas[i], want[i])
		}
	}
}

func TestReadSource_HonoursTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	lemmas, err := readSource(path, 2)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if len(lemmas) != 2 {
		t.Fatalf("lemmas = %v, want two", lemmas)
	}
}

func TestClampLevel(t *testing.T) {
	cases := map[string]string{
		"A1": "A1", "a2": "A2", "B1": "B1", "b2": "B2", "C1": "C1",
		"C2": "C1", "unknown": "B1", "": "B1",
	}
	for input, want := range cases {
		if got := clampLevel(input); got != want {
			t.Errorf("clampLevel(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestIsWord_DropsInflectedProfanity. The exact list let "fucked", "shitty" and
// "bitches" into the frozen fixture; the stems catch the forms it misses, and an
// ordinary word survives.
func TestIsWord_DropsInflectedProfanity(t *testing.T) {
	for _, word := range []string{"fucked", "fuckin", "shitty", "bullshit", "bitches", "porn", "tits"} {
		if isWord(word) {
			t.Errorf("isWord(%q) = true, want the profanity dropped", word)
		}
	}
	for _, word := range []string{"spicy", "cocktail", "therapeutic", "suspicious", "shift"} {
		if !isWord(word) {
			t.Errorf("isWord(%q) = false, want an ordinary word kept", word)
		}
	}
}
