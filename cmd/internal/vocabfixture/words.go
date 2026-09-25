// Package vocabfixture is the frozen vocabulary format shared by the build tool
// that produces it and the seed that loads it (WO 22 Stage C/D).
//
// The words are built once, checked, and frozen under db/fixtures/vocabulary,
// so `make seed` loads 10,000 words offline with no dictionary and no model
// call. Each file is headed with every source, licence and date.
package vocabfixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Format is the fixture schema version. A file with another format is refused
// rather than guessed at.
const Format = "fluentra.vocabulary.fixture.v1"

// File is one CEFR level's words.
type File struct {
	Format    string `json:"format"`
	Source    string `json:"source"`
	Licence   string `json:"licence"`
	CheckedAt string `json:"checked_at"`
	Level     string `json:"level"`
	Words     []Word `json:"words"`
}

// Word is one headword with its primary sense and, when one was found, a
// credited recording.
type Word struct {
	// Rank is the headword's place in the frequency list, 1 the commonest. It
	// is what the seed's frequency_rank and the "Top 1,000" deck read (D22-10);
	// zero in a fixture written before it was recorded.
	Rank         int       `json:"rank,omitempty"`
	Lemma        string    `json:"lemma"`
	POS          string    `json:"pos"`
	CEFRLevel    string    `json:"cefr_level"`
	IPA          string    `json:"ipa,omitempty"`
	Definition   string    `json:"definition"`
	DefinitionVI string    `json:"definition_vi"`
	Examples     []Example `json:"examples"`
	// AudioURL is the recording; when it is set the attribution and licence
	// must be too, so a clip a learner hears is always credited.
	AudioURL         string `json:"audio_url,omitempty"`
	AudioAttribution string `json:"audio_attribution,omitempty"`
	AudioLicence     string `json:"audio_licence,omitempty"`
}

// Example is one example sentence and its Vietnamese rendering.
type Example struct {
	Sentence   string `json:"sentence"`
	SentenceVi string `json:"sentence_vi,omitempty"`
}

// validLevels are the levels a word may carry. C1 is the ceiling: a 10,000-word
// frequency list does not reach C2 honestly (WO 22 D22-7).
var validLevels = map[string]bool{"A1": true, "A2": true, "B1": true, "B2": true, "C1": true}

// LoadWords reads and validates one file.
func LoadWords(path string) (*File, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the operator's own fixtures directory
	if err != nil {
		return nil, fmt.Errorf("read vocabulary fixture: %w", err)
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse vocabulary fixture: %w", err)
	}
	if file.Format != Format {
		return nil, fmt.Errorf("vocabulary fixture format is %q, want %q", file.Format, Format)
	}
	seen := make(map[string]bool, len(file.Words))
	for i, word := range file.Words {
		if err := word.validate(file.Level); err != nil {
			return nil, fmt.Errorf("word %d (%s): %w", i+1, word.Lemma, err)
		}
		lemma := strings.ToLower(word.Lemma)
		if seen[lemma] {
			return nil, fmt.Errorf("word %d: duplicate lemma %q", i+1, word.Lemma)
		}
		seen[lemma] = true
	}
	return &file, nil
}

// ReadAll reads every words-*.json in dir, in file-name order.
func ReadAll(dir string) ([]*File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read vocabulary fixtures directory: %w", err)
	}
	var files []*File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "words-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		file, err := LoadWords(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// ValidateWord checks one word against the format's rules, so the build tool
// can flag a bad word before it writes a whole file.
func ValidateWord(w Word, fileLevel string) error { return w.validate(fileLevel) }

func (w Word) validate(fileLevel string) error {
	if strings.TrimSpace(w.Lemma) == "" {
		return fmt.Errorf("needs a lemma")
	}
	if strings.TrimSpace(w.Definition) == "" {
		return fmt.Errorf("needs an English definition")
	}
	if strings.TrimSpace(w.DefinitionVI) == "" {
		return fmt.Errorf("needs a Vietnamese meaning")
	}
	if !validLevels[w.CEFRLevel] {
		return fmt.Errorf("CEFR level %q is not A1-C1", w.CEFRLevel)
	}
	if fileLevel != "" && w.CEFRLevel != fileLevel {
		return fmt.Errorf("level %q is not the file's %q", w.CEFRLevel, fileLevel)
	}
	if len(w.Examples) == 0 {
		return fmt.Errorf("needs at least one example")
	}
	if !lemmaAppearsInAnExample(w.Lemma, w.Examples) {
		return fmt.Errorf("lemma %q does not appear in any example", w.Lemma)
	}
	if w.AudioURL != "" {
		if strings.TrimSpace(w.AudioAttribution) == "" {
			return fmt.Errorf("a recording needs its attribution")
		}
		// The Commons fallback credits the file page, which states the licence;
		// the lookup does not guess one. Any licence that is named must allow
		// linking with a credit.
		if strings.TrimSpace(w.AudioLicence) == "" && !strings.Contains(w.AudioAttribution, "/wiki/File:") {
			return fmt.Errorf("a recording needs its licence, or a Commons file page that states it")
		}
		if w.AudioLicence != "" && !IsRecordingLicence(w.AudioLicence) {
			return fmt.Errorf("recording licence %q is not CC0, CC BY or CC BY-SA", w.AudioLicence)
		}
	}
	return nil
}

// IsRecordingLicence reports whether a recording may be linked and played with
// a credit: CC0, CC BY or CC BY-SA, any version. A recording is linked, never
// copied or built into other material, so share-alike places no condition on
// the lesson; non-commercial and no-derivatives licences are refused. Photos
// are stricter (examfixture.IsOpenLicence) because they illustrate our items.
func IsRecordingLicence(licence string) bool {
	compact := strings.NewReplacer(" ", "", "-", "", "_", "").
		Replace(strings.ToUpper(strings.TrimSpace(licence)))
	compact = strings.TrimPrefix(compact, "CC")
	if strings.HasPrefix(compact, "0") || strings.HasPrefix(compact, "PUBLICDOMAIN") {
		return true
	}
	if !strings.HasPrefix(compact, "BY") {
		return false
	}
	return !strings.Contains(compact, "NC") && !strings.Contains(compact, "ND")
}

// SkippedFile is where the build tool records the headwords it decided not to
// teach — proper nouns, abbreviations, inflected forms, non-words — with the
// reason, so a later run never asks the model about them again.
const SkippedFile = "skipped.json"

// Skipped is one headword the list will not teach.
type Skipped struct {
	Lemma  string `json:"lemma"`
	Reason string `json:"reason"`
}

// ReadSkipped reads the skipped headwords of a fixture directory, keyed by lemma.
func ReadSkipped(dir string) (map[string]string, error) {
	skipped := map[string]string{}
	raw, err := os.ReadFile(filepath.Join(dir, SkippedFile)) //nolint:gosec // the operator's own fixtures
	if err != nil {
		if os.IsNotExist(err) {
			return skipped, nil
		}
		return nil, fmt.Errorf("read skipped headwords: %w", err)
	}
	var rows []Skipped
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse skipped headwords: %w", err)
	}
	for _, row := range rows {
		skipped[strings.ToLower(row.Lemma)] = row.Reason
	}
	return skipped, nil
}

// WriteSkipped writes the skipped headwords, sorted, so the file diffs cleanly.
func WriteSkipped(dir string, skipped map[string]string) error {
	rows := make([]Skipped, 0, len(skipped))
	for lemma, reason := range skipped {
		rows = append(rows, Skipped{Lemma: lemma, Reason: reason})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Lemma < rows[j].Lemma })
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return fmt.Errorf("encode skipped headwords: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkippedFile), append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write skipped headwords: %w", err)
	}
	return nil
}

// lemmaAppearsInAnExample reports whether the headword appears in any example.
// An inflected form counts, since a word list holds lemmas ("study") and the
// example may use an inflection ("studies"): the check is on the lemma's stem,
// which is at least the first four characters.
func lemmaAppearsInAnExample(lemma string, examples []Example) bool {
	stem := strings.ToLower(strings.TrimSpace(lemma))
	if stem == "" {
		return false
	}
	if len(stem) > 4 {
		stem = stem[:len(stem)-2]
	}
	for _, example := range examples {
		if strings.Contains(strings.ToLower(example.Sentence), stem) {
			return true
		}
	}
	return false
}

// WriteFile writes one level's words to dir as words-<level>.json, headed with
// the sources and licences the build tool recorded.
func WriteFile(dir string, file *File) (string, error) {
	if file.Format == "" {
		file.Format = Format
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create vocabulary fixtures directory: %w", err)
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode vocabulary fixture: %w", err)
	}
	path := filepath.Join(dir, "words-"+strings.ToLower(file.Level)+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write vocabulary fixture: %w", err)
	}
	return path, nil
}

// SortedLemmas returns every lemma across the files, sorted, for the seed and
// for a person reading the 2 % sample.
func SortedLemmas(files []*File) []string {
	var lemmas []string
	for _, file := range files {
		for _, word := range file.Words {
			lemmas = append(lemmas, strings.ToLower(word.Lemma))
		}
	}
	sort.Strings(lemmas)
	return lemmas
}
