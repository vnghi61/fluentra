package main

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/vocabfixture"
)

// defaultVocabularyFixtureDir is where `cmd/vocabgen` writes the frozen word
// list this loader reads (WO 22 Stage C/D).
const defaultVocabularyFixtureDir = "db/fixtures/vocabulary"

// seedVocabularyFromFixture loads the frozen word list through the same writer
// the curated deck uses, so a fixture word and a hand-written one reach the
// database in exactly the same shape. It reports whether a fixture was found:
// a database without one still seeds the curated 200 rather than nothing.
func seedVocabularyFromFixture(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, dir string, out io.Writer,
) (int, bool, error) {
	files, err := vocabfixture.ReadAll(dir)
	if err != nil {
		return 0, false, fmt.Errorf("read vocabulary fixtures: %w", err)
	}
	if len(files) == 0 {
		_, _ = fmt.Fprintf(out,
			"No vocabulary fixtures in %s; seeding the curated 200 instead. "+
				"Run `cmd/vocabgen` and `-export` to freeze the 10,000-word list.\n", dir)
		return 0, false, nil
	}

	var senses []seedWordSense
	for _, file := range files {
		for _, word := range file.Words {
			examples := make([]seedExample, 0, len(word.Examples))
			for _, example := range word.Examples {
				examples = append(examples, seedExample{
					Sentence:   example.Sentence,
					SentenceVi: example.SentenceVi,
				})
			}
			senses = append(senses, seedWordSense{
				Lemma:            word.Lemma,
				POS:              word.POS,
				CEFRLevel:        word.CEFRLevel,
				IPA:              word.IPA,
				Definition:       word.Definition,
				DefinitionVI:     word.DefinitionVI,
				Examples:         examples,
				AudioURL:         word.AudioURL,
				AudioAttribution: word.AudioAttribution,
				AudioLicence:     word.AudioLicence,
			})
		}
	}

	count, err := seedVocabularyWords(ctx, pool, adminID, senses)
	if err != nil {
		return count, true, fmt.Errorf("seed fixture vocabulary: %w", err)
	}
	return count, true, nil
}
