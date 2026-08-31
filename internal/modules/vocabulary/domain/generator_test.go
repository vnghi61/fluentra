package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
)

func sense(lemma, pos, cefr, definition string, examples ...string) domain.GenSense {
	pairs := make([]domain.GenExample, 0, len(examples))
	for _, example := range examples {
		pairs = append(pairs, domain.GenExample{Sentence: example, SentenceVi: "bản dịch"})
	}
	return domain.GenSense{
		Lemma: lemma, POS: pos, CEFRLevel: cefr, Definition: definition,
		IPA: "/x/", Examples: pairs,
	}
}

// A pool wide enough that distractor selection has something to choose from.
func pool() []domain.GenSense {
	return []domain.GenSense{
		sense("leisure", "noun", "B1", "Time when one is not working.",
			"He spends his leisure time restoring old bicycles.",
			"The hotel offers a range of leisure activities for families.",
			"She reads at leisure."),
		sense("journey", "noun", "B1", "An act of travelling from one place to another."),
		sense("habit", "noun", "A2", "A settled or regular tendency."),
		sense("barrier", "noun", "B1", "An obstacle that prevents movement."),
		sense("comfort", "noun", "B1", "A state of physical ease."),
		sense("achieve", "verb", "B1", "Bring about by effort."),
		sense("borrow", "verb", "A2", "Take and use something temporarily."),
	}
}

func find(t *testing.T, exercises []domain.GenExercise, kind string) domain.GenExercise {
	t.Helper()
	for _, exercise := range exercises {
		if exercise.Kind == kind {
			return exercise
		}
	}
	t.Fatalf("no %s was generated", kind)
	return domain.GenExercise{}
}

func target() domain.GenSense { return pool()[0] }

// ------------------------------------------------------------- determinism

func TestGenerateForSense_IsDeterministic(t *testing.T) {
	// The generator runs on a schedule. One that shuffled differently each time
	// would rewrite the whole catalogue every hour, drop every cached lesson,
	// and change the exercise under a learner who had it open.
	first := domain.GenerateForSense(target(), pool())
	second := domain.GenerateForSense(target(), pool())

	require.Equal(t, len(first), len(second))
	for i := range first {
		assert.Equal(t, first[i].Slug, second[i].Slug)
		assert.Equal(t, first[i].Config, second[i].Config)
		assert.Equal(t, first[i].Body, second[i].Body)
	}
}

func TestGenerateForSense_SlugsAreStableAndDistinct(t *testing.T) {
	exercises := domain.GenerateForSense(target(), pool())

	seen := map[string]bool{}
	for _, exercise := range exercises {
		assert.False(t, seen[exercise.Slug], "two exercises share the slug %q", exercise.Slug)
		seen[exercise.Slug] = true
		assert.Contains(t, exercise.Slug, "leisure")
	}
}

// ------------------------------------------------------- what gets produced

func TestGenerateForSense_CoversEveryKindItCan(t *testing.T) {
	exercises := domain.GenerateForSense(target(), pool())

	kinds := map[string]bool{}
	for _, exercise := range exercises {
		kinds[exercise.Kind] = true
	}
	// Matching needs several words and is generated separately.
	assert.True(t, kinds[domain.KindFlashcard])
	assert.True(t, kinds[domain.KindMultipleChoice])
	assert.True(t, kinds[domain.KindGapFill])
	assert.True(t, kinds[domain.KindListenType])
	assert.True(t, kinds[domain.KindContextChoice])
	assert.True(t, kinds[domain.KindReorder])
	assert.False(t, kinds[domain.KindMatch])
}

func TestGenerateForSense_SkipsAKindItCannotBuildRatherThanFakingIt(t *testing.T) {
	// No examples: no gap fill, no reorder, no context choice. An exercise
	// built from an absent sentence is an exercise about nothing.
	bare := sense("solitary", "adjective", "B2", "Done or existing alone.")
	exercises := domain.GenerateForSense(bare, pool())

	for _, exercise := range exercises {
		assert.NotEqual(t, domain.KindGapFill, exercise.Kind)
		assert.NotEqual(t, domain.KindReorder, exercise.Kind)
		assert.NotEqual(t, domain.KindContextChoice, exercise.Kind)
	}
	assert.NotEmpty(t, exercises, "a word with a definition can still be a flashcard")
}

func TestGenerateForSense_RefusesAWordWithNoDefinition(t *testing.T) {
	assert.Empty(t, domain.GenerateForSense(sense("x", "noun", "B1", "  "), pool()))
	assert.Empty(t, domain.GenerateForSense(sense("  ", "noun", "B1", "A meaning."), pool()))
}

// ------------------------------------------------- the answer never leaks

func TestGeneratedConfigsNeverCarryTheAnswer(t *testing.T) {
	// The config is what the browser receives. Every answer key belongs in the
	// body, which the grader reads server-side — the same split the redaction
	// list exists to enforce.
	answerKeys := []string{
		"correct_answer", "correct_option_id", "correct_pairs", "acceptable",
	}

	exercises := domain.GenerateForSense(target(), pool())
	match, ok := domain.GenerateMatch(pool()[:4])
	require.True(t, ok)
	exercises = append(exercises, match)

	for _, exercise := range exercises {
		for _, key := range answerKeys {
			_, present := exercise.Config[key]
			assert.False(t, present,
				"%s config carries %q, which the learner must not hold", exercise.Kind, key)
		}
	}
}

func TestEveryGeneratedExerciseIsGradable(t *testing.T) {
	// The grader refuses a body with neither a correct answer nor a pair map,
	// and schedules review cards from word_lemmas.
	exercises := domain.GenerateForSense(target(), pool())
	match, ok := domain.GenerateMatch(pool()[:4])
	require.True(t, ok)
	exercises = append(exercises, match)

	for _, exercise := range exercises {
		answer, hasAnswer := exercise.Body["correct_answer"].(string)
		pairs, hasPairs := exercise.Body["correct_pairs"].(map[string]string)
		assert.True(t, (hasAnswer && answer != "") || (hasPairs && len(pairs) > 0),
			"%s has nothing the grader can score", exercise.Kind)

		lemmas, hasLemmas := exercise.Body["word_lemmas"].([]string)
		assert.True(t, hasLemmas && len(lemmas) > 0,
			"%s names no word, so answering it earns a review card for nothing", exercise.Kind)
	}
}

// ------------------------------------------------------------ per kind

func TestGapFill_BlanksTheWordOutOfItsOwnSentence(t *testing.T) {
	exercise := find(t, domain.GenerateForSense(target(), pool()), domain.KindGapFill)

	before := exercise.Config["sentence_before"].(string)
	after := exercise.Config["sentence_after"].(string)
	assert.Equal(t, "He spends his", before)
	assert.Equal(t, "time restoring old bicycles.", after)
	assert.NotContains(t, strings.ToLower(before+" "+after), "leisure",
		"the blank is the exercise; leaving the word in either half gives it away")
	assert.Equal(t, "leisure", exercise.Config["expected_answer"])
}

func TestMultipleChoice_AsksForTheWordGivenTheMeaning(t *testing.T) {
	exercise := find(t, domain.GenerateForSense(target(), pool()), domain.KindMultipleChoice)

	// The definition is the question. The other way round, a learner can often
	// eliminate three definitions by register alone.
	assert.Contains(t, exercise.Config["prompt"], "Time when one is not working")

	options := exercise.Config["options"].([]map[string]string)
	require.Len(t, options, 4)

	texts := make([]string, 0, len(options))
	for _, option := range options {
		texts = append(texts, option["text"])
	}
	assert.Contains(t, texts, "leisure")
	assert.Len(t, unique(texts), 4, "a repeated option is a free elimination")
}

func TestMultipleChoice_PrefersDistractorsOfTheSamePartOfSpeech(t *testing.T) {
	// "leisure" against a verb is a grammar question the learner can win
	// without knowing the word.
	exercise := find(t, domain.GenerateForSense(target(), pool()), domain.KindMultipleChoice)
	options := exercise.Config["options"].([]map[string]string)

	nouns := map[string]bool{
		"leisure": true, "journey": true, "habit": true,
		"barrier": true, "comfort": true,
	}
	for _, option := range options {
		assert.True(t, nouns[option["text"]],
			"%q is not a noun, and the pool had enough nouns to avoid it", option["text"])
	}
}

func TestListenType_HidesNothingItShouldNotAndCarriesTheSpokenWord(t *testing.T) {
	exercise := find(t, domain.GenerateForSense(target(), pool()), domain.KindListenType)

	// The spoken text is the answer and reaches the client deliberately —
	// synthesis happens there. It is named `audio_text` so that is legible.
	assert.Equal(t, "leisure", exercise.Config["audio_text"])
	assert.Equal(t, "Time when one is not working.", exercise.Config["hint"])
}

func TestContextChoice_UsesADifferentSentenceFromTheGapFill(t *testing.T) {
	exercises := domain.GenerateForSense(target(), pool())
	gapFill := find(t, exercises, domain.KindGapFill)
	context := find(t, exercises, domain.KindContextChoice)

	gapSentence := gapFill.Config["sentence_before"].(string)
	assert.NotContains(t, context.Config["sentence"], gapSentence,
		"meeting the same sentence twice in a lesson teaches the sentence, not the word")
}

func TestReorder_ShufflesAndNeverReturnsTheOriginalOrder(t *testing.T) {
	exercise := find(t, domain.GenerateForSense(target(), pool()), domain.KindReorder)

	tokens := exercise.Config["tokens"].([]string)
	answer := exercise.Body["correct_answer"].(string)
	original := strings.Fields(answer)

	assert.ElementsMatch(t, original, tokens, "shuffling must not lose or invent a word")
	assert.NotEqual(t, original, tokens, "an unshuffled reorder is not an exercise")
}

func TestReorder_SkipsSentencesTooShortToBeWorthOrdering(t *testing.T) {
	short := sense("brief", "adjective", "A2", "Short in duration.", "It was brief.")
	for _, exercise := range domain.GenerateForSense(short, pool()) {
		assert.NotEqual(t, domain.KindReorder, exercise.Kind,
			"three tokens can be ordered by accident, which teaches nothing")
	}
}

// ------------------------------------------------------------- matching

func TestGenerateMatch_PairsEveryWordAndShufflesTheColumnsApart(t *testing.T) {
	group := pool()[:4]
	exercise, ok := domain.GenerateMatch(group)
	require.True(t, ok)

	words := exercise.Config["words"].([]map[string]string)
	definitions := exercise.Config["definitions"].([]map[string]string)
	pairs := exercise.Body["correct_pairs"].(map[string]string)

	require.Len(t, words, 4)
	require.Len(t, definitions, 4)
	require.Len(t, pairs, 4)

	// Every key resolves to a rendered row, or the exercise cannot be completed.
	wordIDs := map[string]bool{}
	for _, word := range words {
		wordIDs[word["id"]] = true
	}
	definitionIDs := map[string]bool{}
	for _, definition := range definitions {
		definitionIDs[definition["id"]] = true
	}
	for wordID, definitionID := range pairs {
		assert.True(t, wordIDs[wordID], "pair key %q is not a rendered word", wordID)
		assert.True(t, definitionIDs[definitionID],
			"pair value %q is not a rendered definition", definitionID)
	}

	// If both columns shuffled identically, the visible order would be the
	// answer and the exercise a formality.
	sameOrder := true
	for i := range words {
		if strings.TrimPrefix(words[i]["id"], "w_") != strings.TrimPrefix(definitions[i]["id"], "d_") {
			sameOrder = false
			break
		}
	}
	assert.False(t, sameOrder, "the two columns must not line up")
}

func TestGenerateMatch_NeedsMoreThanOneWord(t *testing.T) {
	_, ok := domain.GenerateMatch(pool()[:1])
	assert.False(t, ok)
	_, ok = domain.GenerateMatch(nil)
	assert.False(t, ok)
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
