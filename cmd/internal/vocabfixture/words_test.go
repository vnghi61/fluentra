package vocabfixture

import (
	"os"
	"path/filepath"
	"testing"
)

// testLemma is the headword the acceptance tests use.
const testLemma = "study"

func writeWords(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

const validWord = `{
	"lemma": "study",
	"pos": "verb",
	"cefr_level": "A2",
	"definition": "To learn about a subject by reading and thinking.",
	"definition_vi": "Học",
	"examples": [{"sentence": "She studies English every evening.", "sentence_vi": "Cô ấy học tiếng Anh mỗi tối."}],
	"audio_url": "https://example.org/study.mp3",
	"audio_attribution": "Wikimedia Commons",
	"audio_licence": "CC-BY-4.0"
}`

func wrap(words string) string {
	return `{"format":"` + Format + `","source":"wordfreq","licence":"CC-BY-4.0","level":"A2","words":[` + words + `]}`
}

func TestLoadWords_AcceptsACreditedWord(t *testing.T) {
	file, err := LoadWords(writeWords(t, "words-a2.json", wrap(validWord)))
	if err != nil {
		t.Fatalf("a valid word must load: %v", err)
	}
	if len(file.Words) != 1 || file.Words[0].Lemma != testLemma {
		t.Fatalf("words = %+v", file.Words)
	}
}

func TestLoadWords_RefusesWhatWouldBeWrong(t *testing.T) {
	tests := map[string]string{
		"duplicate lemma": wrap(validWord + "," + validWord),
		"cefr outside A1-C1": wrap(`{
			"lemma": "ubiquitous", "pos": "adjective", "cefr_level": "C2",
			"definition": "Present everywhere.", "definition_vi": "Khắp nơi",
			"examples": [{"sentence": "It is ubiquitous."}]
		}`),
		"level not the file's": wrap(`{
			"lemma": "book", "pos": "noun", "cefr_level": "B1",
			"definition": "A written work.", "definition_vi": "Sách",
			"examples": [{"sentence": "I read a book."}]
		}`),
		"no example": wrap(`{
			"lemma": "book", "pos": "noun", "cefr_level": "A2",
			"definition": "A written work.", "definition_vi": "Sách", "examples": []
		}`),
		"lemma missing from examples": wrap(`{
			"lemma": "library", "pos": "noun", "cefr_level": "A2",
			"definition": "A place with books.", "definition_vi": "Thư viện",
			"examples": [{"sentence": "I read every day."}]
		}`),
		"recording without attribution": wrap(`{
			"lemma": "book", "pos": "noun", "cefr_level": "A2",
			"definition": "A written work.", "definition_vi": "Sách",
			"examples": [{"sentence": "I read a book."}],
			"audio_url": "https://example.org/book.mp3", "audio_licence": "CC-BY-4.0"
		}`),
		"recording under a closed licence": wrap(`{
			"lemma": "book", "pos": "noun", "cefr_level": "A2",
			"definition": "A written work.", "definition_vi": "Sách",
			"examples": [{"sentence": "I read a book."}],
			"audio_url": "https://example.org/book.mp3", "audio_attribution": "x", "audio_licence": "CC-BY-NC-4.0"
		}`),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadWords(writeWords(t, "words-a2.json", body)); err == nil {
				t.Fatal("an unusable word must be refused")
			}
		})
	}
}

func TestWriteFile_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	file := &File{
		Source: "wordfreq", Licence: "CC-BY-4.0", CheckedAt: "2026-09-24", Level: "A2",
		Words: []Word{
			{
				Lemma: testLemma, POS: "verb", CEFRLevel: "A2",
				Definition: "To learn about a subject.", DefinitionVI: "Học",
				Examples: []Example{{Sentence: "She studies English every evening."}},
			},
		},
	}
	path, err := WriteFile(dir, file)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := LoadWords(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Words) != 1 || loaded.Words[0].Lemma != testLemma {
		t.Fatalf("words = %+v", loaded.Words)
	}
}

func TestReadAll_ReadsOnlyWordsFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "words-a2.json"), []byte(wrap(validWord)), 0o600); err != nil {
		t.Fatalf("write words: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o600); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	files, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(files) != 1 || len(files[0].Words) != 1 {
		t.Fatalf("files = %+v, want one words file", files)
	}
}

// TestIsRecordingLicence: a recording is linked with a credit, so CC BY-SA is
// fine; non-commercial and no-derivatives licences are not.
func TestIsRecordingLicence(t *testing.T) {
	for _, licence := range []string{"CC0", "CC BY 4.0", "BY-SA 3.0", "CC-BY-SA-4.0"} {
		if !IsRecordingLicence(licence) {
			t.Errorf("%q must be accepted", licence)
		}
	}
	for _, licence := range []string{"CC BY-NC 4.0", "CC-BY-ND-2.0", "All rights reserved"} {
		if IsRecordingLicence(licence) {
			t.Errorf("%q must be refused", licence)
		}
	}
}

// TestLoadWords_AcceptsACommonsRecordingWithoutANamedLicence: the Commons file
// page states the licence, so the word need not repeat it.
func TestLoadWords_AcceptsACommonsRecordingWithoutANamedLicence(t *testing.T) {
	body := wrap(`{
		"lemma": "book", "pos": "noun", "cefr_level": "A2",
		"definition": "A written work.", "definition_vi": "Sách",
		"examples": [{"sentence": "I read a book."}],
		"audio_url": "https://commons.wikimedia.org/wiki/Special:FilePath/En-us-book.ogg",
		"audio_attribution": "https://commons.wikimedia.org/wiki/File:En-us-book.ogg"
	}`)
	if _, err := LoadWords(writeWords(t, "words-a2.json", body)); err != nil {
		t.Fatalf("a Commons recording must load: %v", err)
	}
}

// TestSkipped_RoundTrips: the skipped headwords written are the ones read.
func TestSkipped_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSkipped(dir, map[string]string{"john": "proper_noun", "went": "inflection_of:go"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadSkipped(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got["john"] != "proper_noun" || got["went"] != "inflection_of:go" {
		t.Fatalf("skipped = %v", got)
	}
}
