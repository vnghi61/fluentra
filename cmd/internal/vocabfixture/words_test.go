package vocabfixture

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if len(file.Words) != 1 || file.Words[0].Lemma != "study" {
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
				Lemma: "study", POS: "verb", CEFRLevel: "A2",
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
	if len(loaded.Words) != 1 || loaded.Words[0].Lemma != "study" {
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
