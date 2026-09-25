package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fluentra/fluentra/cmd/internal/vocabfixture"
	"github.com/fluentra/fluentra/internal/platform/ai"
)

// builder walks the frequency list, asking the model for the headwords it has
// no meaning for, until the fixture holds the headwords it should.
type builder struct {
	client    ai.Client
	dir       string
	cacheFile string
	batch     int
	dryRun    bool
	out       io.Writer
	// cache is every meaning written so far, keyed by lemma.
	cache map[string]modelMeaning
	// skipped is every headword the list will not teach, with the reason.
	skipped map[string]string
	// existing is the fixture as it stands, for the pronunciation it carries.
	existing map[string]vocabfixture.Word
}

func newBuilder(
	client ai.Client, dir, cacheFile string, batch int, dryRun bool, out io.Writer,
) (*builder, error) {
	if batch <= 0 {
		batch = 15
	}
	cache, err := readCache(cacheFile)
	if err != nil {
		return nil, err
	}
	// The fixtures already written are part of the cache: a run that stopped
	// after writing words 1-3,000 resumes at 3,001 without paying for them
	// twice, even if the scratch cache is gone.
	if err := mergeCacheFromFixtures(dir, cache); err != nil {
		return nil, err
	}
	skipped, err := vocabfixture.ReadSkipped(dir)
	if err != nil {
		return nil, err
	}
	existing, err := existingWords(dir)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(out, "Cache holds %d meaning(s); %d headword(s) are skipped\n", len(cache), len(skipped))
	return &builder{
		client: client, dir: dir, cacheFile: cacheFile, batch: batch, dryRun: dryRun, out: out,
		cache: cache, skipped: skipped, existing: existing,
	}, nil
}

// generateUntil walks the candidates in rank order, one batch at a time, until
// limit of them are accepted: not skipped, with a meaning that passes the
// fixture's checks. A headword the model skips, or whose meaning fails, is
// replaced by the next one down the list, so the fixture still reaches its
// size. The cache and the skipped list are saved after every batch, so a
// failure resumes where it stopped.
func (b *builder) generateUntil(
	ctx context.Context, candidates []sourceLemma, limit int,
) ([]sourceLemma, []flaggedWord, error) {
	var accepted []sourceLemma
	var flagged []flaggedWord
	for start := 0; start < len(candidates) && len(accepted) < limit; start += b.batch {
		window := candidates[start:min(start+b.batch, len(candidates))]
		failed, err := b.fill(ctx, window)
		if err != nil {
			return accepted, flagged, err
		}
		flagged = append(flagged, failed...)
		for _, candidate := range window {
			if len(accepted) >= limit {
				break
			}
			if _, skip := b.skipped[candidate.Lemma]; skip {
				continue
			}
			meaning, ok := b.cache[candidate.Lemma]
			if !ok {
				continue
			}
			word := toWord(candidate, meaning, nil)
			if err := vocabfixture.ValidateWord(word, word.CEFRLevel); err != nil {
				flagged = append(flagged, flaggedWord{Lemma: candidate.Lemma, Reason: err.Error()})
				continue
			}
			accepted = append(accepted, candidate)
		}
		_, _ = fmt.Fprintf(b.out, "  ✓ %d/%d headword(s) accepted\n", len(accepted), limit)
	}
	if len(accepted) < limit {
		_, _ = fmt.Fprintf(b.out,
			"  ! The list ran out at %d headword(s); rebuild it longer with scripts/vocab-source-list.py\n",
			len(accepted))
	}
	return accepted, flagged, nil
}

// fill asks the model about the window's headwords that are neither cached nor
// skipped. It returns the lemmas whose reply could not be parsed.
func (b *builder) fill(ctx context.Context, window []sourceLemma) ([]flaggedWord, error) {
	var ask []string
	for _, candidate := range window {
		if _, skip := b.skipped[candidate.Lemma]; skip {
			continue
		}
		if _, ok := b.cache[candidate.Lemma]; !ok {
			ask = append(ask, candidate.Lemma)
		}
	}
	if len(ask) == 0 {
		return nil, nil
	}
	if b.dryRun {
		_, _ = fmt.Fprintf(b.out, "Would ask the model for %d lemma(s): %s …\n", len(ask), strings.Join(ask, ", "))
		return nil, nil
	}

	meanings, failed, err := generateMeanings(ctx, b.client, ask)
	if err != nil {
		// The cache is saved up to the last complete batch, so the next run
		// resumes here.
		if saveErr := b.save(); saveErr != nil {
			return nil, saveErr
		}
		return nil, fmt.Errorf("generate meanings for %d lemma(s): %w", len(ask), err)
	}
	for _, meaning := range meanings {
		lemma := strings.ToLower(strings.TrimSpace(meaning.Lemma))
		if reason := strings.TrimSpace(meaning.Skip); reason != "" {
			b.skipped[lemma] = reason
			continue
		}
		b.cache[lemma] = meaning
	}
	flagged := make([]flaggedWord, 0, len(failed))
	for _, lemma := range failed {
		flagged = append(flagged, flaggedWord{Lemma: lemma, Reason: "the model's reply could not be parsed"})
	}
	return flagged, b.save()
}

// save writes the cache and the skipped list.
func (b *builder) save() error {
	if err := writeCache(b.cacheFile, b.cache); err != nil {
		return err
	}
	return vocabfixture.WriteSkipped(b.dir, b.skipped)
}

// existingWords is the fixture as it stands, keyed by lemma.
func existingWords(dir string) (map[string]vocabfixture.Word, error) {
	files, err := vocabfixture.ReadAll(dir)
	if err != nil {
		return nil, fmt.Errorf("read the word fixtures: %w", err)
	}
	words := map[string]vocabfixture.Word{}
	for _, file := range files {
		for _, word := range file.Words {
			words[strings.ToLower(word.Lemma)] = word
		}
	}
	return words, nil
}
