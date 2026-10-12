package main

import (
	"context"
	"fmt"
	"io"

	"github.com/fluentra/fluentra/cmd/internal/vocabfixture"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// pronunciationLookup answers a lemma's IPA and credited recording.
type pronunciationLookup interface {
	Lookup(ctx context.Context, lemma string) (repository.DictionaryEntry, bool, error)
}

// pronounceSaveEvery is how many lookups pass between two saves, so a run that
// stops at word 7,000 keeps what it found.
const pronounceSaveEvery = 200

// pronounce is step 3 (WO 22 D22-8): every word without a recording is looked
// up once — the free dictionary, then Wikimedia Commons — and the fixture is
// given the dictionary's IPA and a credited recording, US first. A word with no
// recording keeps none; the browser falls back as before (D21-8). The files are
// saved as it goes, and a re-run asks only about words that still have none.
func pronounce(ctx context.Context, dir string, lookup pronunciationLookup, out io.Writer) error {
	files, err := vocabfixture.ReadAll(dir)
	if err != nil {
		return fmt.Errorf("read the word fixtures: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no word fixtures in %s", dir)
	}

	var checked, recorded, failed int
	for _, file := range files {
		for i := range file.Words {
			word := &file.Words[i]
			if word.AudioURL != "" {
				continue
			}
			entry, found, err := lookup.Lookup(ctx, word.Lemma)
			checked++
			switch {
			case err != nil:
				failed++
			case found:
				if entry.IPA != "" {
					word.IPA = entry.IPA
				}
				if entry.AudioURL != "" && entry.AudioAttribution != "" {
					word.AudioURL = entry.AudioURL
					word.AudioAttribution = entry.AudioAttribution
					word.AudioLicence = entry.AudioLicence
					recorded++
				}
			}
			if checked%pronounceSaveEvery == 0 {
				if err := writeAll(dir, files); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "  … %d looked up, %d recorded\n", checked, recorded)
			}
		}
	}
	if err := writeAll(dir, files); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "  ✓ Pronunciation: %d looked up, %d given a recording, %d lookups failed\n",
		checked, recorded, failed)
	return nil
}

// canonicalise rewrites every level file through this tool's writer, after a
// file was edited by another tool (scripts/vocab-source-list.py).
func canonicalise(dir string, out io.Writer) error {
	files, err := vocabfixture.ReadAll(dir)
	if err != nil {
		return fmt.Errorf("read the word fixtures: %w", err)
	}
	if err := writeAll(dir, files); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "  ✓ %d level file(s) rewritten\n", len(files))
	return nil
}

// writeAll writes every level file back, checking each loads.
func writeAll(dir string, files []*vocabfixture.File) error {
	for _, file := range files {
		path, err := vocabfixture.WriteFile(dir, file)
		if err != nil {
			return err
		}
		if _, err := vocabfixture.LoadWords(path); err != nil {
			return fmt.Errorf("the written fixture %s is not loadable: %w", path, err)
		}
	}
	return nil
}
