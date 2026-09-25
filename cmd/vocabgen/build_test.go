package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluentra/fluentra/cmd/internal/vocabfixture"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// The headword and part of speech the build tests use.
const (
	testWord = "time"
	testPOS  = "noun"
)

// meaningsAI answers the meanings task: "harry" is skipped as a proper noun,
// every other lemma gets a valid A1 meaning.
type meaningsAI struct{ asked []string }

func (m *meaningsAI) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	lemmas := strings.Split(req.Vars["Lemmas"].(string), "\n")
	m.asked = append(m.asked, lemmas...)
	words := make([]modelMeaning, 0, len(lemmas))
	for _, lemma := range lemmas {
		if lemma == "harry" {
			words = append(words, modelMeaning{Lemma: lemma, Skip: "proper_noun"})
			continue
		}
		words = append(words, modelMeaning{
			Lemma: lemma, POS: testPOS, CEFRLevel: "A1",
			Definition: "a word", DefinitionVI: "một từ",
			Examples: []vocabfixture.Example{{Sentence: "I like the " + lemma + ".", SentenceVi: "Tôi thích."}},
		})
	}
	raw, err := json.Marshal(map[string]any{"words": words})
	return ai.Response{Text: string(raw)}, err
}

// TestGenerateUntil_ReplacesASkippedHeadword: the model skips a proper noun,
// the next headword down the list takes its place, and the skip is recorded so
// no later run asks about it again.
func TestGenerateUntil_ReplacesASkippedHeadword(t *testing.T) {
	dir := t.TempDir()
	model := &meaningsAI{}
	b, err := newBuilder(model, dir, filepath.Join(dir, "cache.json"), 2, false, io.Discard)
	if err != nil {
		t.Fatalf("new builder: %v", err)
	}
	candidates := []sourceLemma{{1, testWord}, {2, "harry"}, {3, "water"}, {4, "house"}}

	accepted, flagged, err := b.generateUntil(context.Background(), candidates, 3)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(flagged) != 0 {
		t.Fatalf("flagged = %+v, want none", flagged)
	}
	var got []string
	for _, lemma := range accepted {
		got = append(got, lemma.Lemma)
	}
	if strings.Join(got, ",") != "time,water,house" {
		t.Fatalf("accepted = %v, want time, water, house", got)
	}
	skipped, err := vocabfixture.ReadSkipped(dir)
	if err != nil {
		t.Fatalf("read skipped: %v", err)
	}
	if skipped["harry"] != "proper_noun" {
		t.Fatalf("skipped = %v, want harry recorded as a proper noun", skipped)
	}

	// A second run asks the model nothing: the meanings are cached and the
	// skip is on file.
	model.asked = nil
	again, err := newBuilder(model, dir, filepath.Join(dir, "cache.json"), 2, false, io.Discard)
	if err != nil {
		t.Fatalf("new builder: %v", err)
	}
	if _, _, err := again.generateUntil(context.Background(), candidates, 3); err != nil {
		t.Fatalf("generate again: %v", err)
	}
	if len(model.asked) != 0 {
		t.Fatalf("a resumed run asked about %v", model.asked)
	}
}

// TestWriteFixtures_KeepsPronunciationAndRemovesAnEmptiedLevel: a rewrite keeps
// the IPA and recording step 3 found, and a level with no words left loses its
// file rather than keeping stale words.
func TestWriteFixtures_KeepsPronunciationAndRemovesAnEmptiedLevel(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "words-c1.json")
	if err := os.WriteFile(stale, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	cache := map[string]modelMeaning{testWord: {
		Lemma: testWord, POS: testPOS, CEFRLevel: "A1", Definition: "minutes and hours",
		DefinitionVI: "thời gian", Examples: []vocabfixture.Example{{Sentence: "What time is it?"}},
	}}
	existing := map[string]vocabfixture.Word{testWord: {
		Lemma: testWord, IPA: "/taɪm/", AudioURL: "https://example.org/time.mp3",
		AudioAttribution: "https://commons.wikimedia.org/wiki/File:En-us-time.ogg",
	}}

	if err := writeFixtures(dir, []sourceLemma{{7, testWord}}, cache, existing, nil, io.Discard); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, err := vocabfixture.LoadWords(filepath.Join(dir, "words-a1.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	word := file.Words[0]
	if word.Rank != 7 || word.IPA != "/taɪm/" || word.AudioURL == "" {
		t.Fatalf("word = %+v, want rank 7 with its IPA and recording", word)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("the emptied C1 file must be removed, stat = %v", err)
	}
}

// stubPronunciation answers every lemma with a credited recording.
type stubPronunciation struct{}

func (stubPronunciation) Lookup(_ context.Context, lemma string) (repository.DictionaryEntry, bool, error) {
	return repository.DictionaryEntry{
		Lemma: lemma, IPA: "/x/", AudioURL: "https://example.org/" + lemma + ".mp3",
		AudioAttribution: "https://commons.wikimedia.org/wiki/File:En-us-" + lemma + ".ogg",
		AudioLicence:     "BY-SA 3.0",
	}, true, nil
}

// TestPronounce_GivesEveryWordItsRecording is step 3: a word with no recording
// is given the dictionary's IPA and a credited recording.
func TestPronounce_GivesEveryWordItsRecording(t *testing.T) {
	dir := t.TempDir()
	cache := map[string]modelMeaning{testWord: {
		Lemma: testWord, POS: testPOS, CEFRLevel: "A1", Definition: "minutes and hours",
		DefinitionVI: "thời gian", Examples: []vocabfixture.Example{{Sentence: "What time is it?"}},
	}}
	if err := writeFixtures(dir, []sourceLemma{{1, testWord}}, cache, nil, nil, io.Discard); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := pronounce(context.Background(), dir, stubPronunciation{}, io.Discard); err != nil {
		t.Fatalf("pronounce: %v", err)
	}
	file, err := vocabfixture.LoadWords(filepath.Join(dir, "words-a1.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if word := file.Words[0]; word.AudioURL == "" || word.AudioLicence != "BY-SA 3.0" || word.IPA != "/x/" {
		t.Fatalf("word = %+v, want the recording, its licence and the IPA", word)
	}
}
