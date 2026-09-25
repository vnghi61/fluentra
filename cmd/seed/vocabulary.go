package main

import (
	"context"
	"fmt"
	"io"
	"strings"

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
				Rank:             word.Rank,
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

	count, err := seedVocabularyWords(ctx, pool, adminID, senses, fixtureDecksFor)
	if err != nil {
		return count, true, fmt.Errorf("seed fixture vocabulary: %w", err)
	}
	if !hasRanks(senses) {
		_, _ = fmt.Fprintln(out,
			"  ! The word fixture carries no frequency ranks, so no \"Top 1,000\" deck was built; "+
				"re-run `cmd/vocabgen` to record them.")
	}
	return count, true, nil
}

// topWordsRank is the rank the "Top 1,000" deck stops at (D22-10).
const topWordsRank = 1000

// topWordsDeck holds the commonest thousand headwords: the deck a new learner
// starts from, in place of the curated 200.
var topWordsDeck = seedDeck{
	Slug:        "top-1000",
	Name:        "Top 1,000 words",
	Description: "The thousand commonest English headwords, by frequency.",
}

// fixtureDecksFor puts a fixture word in its level's deck and, when it ranks in
// the first thousand, in "Top 1,000" (D22-10). It never puts it in one deck of
// all 10,000: nothing enrols a learner in 10,000 cards.
func fixtureDecksFor(sense seedWordSense) []seedDeck {
	level := strings.ToUpper(sense.CEFRLevel)
	decks := []seedDeck{{
		Slug:        "words-" + strings.ToLower(level),
		Name:        level + " words",
		Description: "Every " + level + " headword of the 10,000-word list, by frequency.",
	}}
	if sense.Rank > 0 && sense.Rank <= topWordsRank {
		decks = append(decks, topWordsDeck)
	}
	return decks
}

// hasRanks reports whether any fixture word carries a frequency rank.
func hasRanks(senses []seedWordSense) bool {
	for _, sense := range senses {
		if sense.Rank > 0 {
			return true
		}
	}
	return false
}
