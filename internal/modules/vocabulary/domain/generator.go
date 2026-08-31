package domain

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

// Exercise generation: turning a dictionary entry into things to practise.
//
// Deterministic throughout. The generator runs on a schedule, and a generator
// that shuffled differently on each run would rewrite the whole catalogue every
// hour, invalidate every cached lesson, and change the exercise under a learner
// who had it open. Every shuffle here is seeded from the data being shuffled, so
// the same words always yield the same lesson.
//
// No LLM. Whether "leisure" means free time, and which four words make plausible
// distractors, are questions the dictionary data already answers; asking a model
// would cost money, add latency to a batch job, and introduce a way for the
// exercise to be wrong.

// GenSense is the input: one word sense, with everything an exercise can be
// built from.
type GenSense struct {
	Lemma        string
	POS          string
	CEFRLevel    string
	IPA          string
	Definition   string
	DefinitionVi string
	AudioURL     string
	Examples     []GenExample
	// ContentVersionID of the sense's own dictionary entry, so a generated
	// exercise can point review cards at the word rather than at itself.
	ContentVersionID string
}

// GenExample is one example sentence and its translation.
type GenExample struct {
	Sentence   string
	SentenceVi string
}

// GenExercise is the output: one activity, ready to be authored.
//
// `Config` is what the learner's browser receives and `Body` is what the grader
// reads. They are separate because the body carries the answer and the config
// must not — the same split the seed and the redaction list already enforce.
type GenExercise struct {
	// Slug identifies the content item. Stable across runs, so regenerating
	// updates the exercise rather than creating a second one.
	Slug   string
	Kind   string
	Config map[string]any
	Body   map[string]any
}

// The activity kinds the generator produces. They are the kinds the runner can
// render; generating one it cannot is an activity a learner reaches and cannot do.
const (
	KindMultipleChoice = "vocab_multiple_choice"
	KindGapFill        = "vocab_gap_fill"
	KindFlashcard      = "vocab_flashcard"
	KindListenType     = "vocab_listen_type"
	KindMatch          = "vocab_match"
	KindReorder        = "vocab_reorder"
	KindContextChoice  = "vocab_context_choice"
)

// The body and config keys, named because they are the contract between this
// generator, the grader and the renderer.
const (
	keyPrompt          = "prompt"
	keyCorrectAnswer   = "correct_answer"
	keyAcceptable      = "acceptable"
	keyCorrectOptionID = "correct_option_id"
	keyCorrectPairs    = "correct_pairs"
	keyWordLemmas      = "word_lemmas"
	keyOptions         = "options"
	keyWords           = "words"
	keyDefinitions     = "definitions"
	keyTokens          = "tokens"
	keySentence        = "sentence"
	keyTargetWord      = "target_word"
	keyIPA             = "ipa"
	keyDefinition      = "definition"
	keyDefinitionVi    = "definition_vi"
	keyAudioText       = "audio_text"
	keyAudioURL        = "audio_url"
	keyHint            = "hint"
	keySentenceBefore  = "sentence_before"
	keySentenceAfter   = "sentence_after"
	keyExpectedAnswer  = "expected_answer"
	keyExampleSentence = "example_sentence"
	keyExamples        = "example_sentences"
	keyOptionID        = "id"
	keyOptionText      = "text"
)

// distractorCount is how many wrong options a choice exercise offers.
const distractorCount = 3

// MatchGroupSize is how many words one matching exercise pairs. Four fits a
// phone screen as two columns without scrolling, which is where most of these
// are answered.
const MatchGroupSize = 4

// GenerateForSense builds every exercise one word can support on its own.
//
// Matching is absent because it needs several words; see GenerateMatch.
// A kind whose input is missing is skipped rather than faked: an exercise built
// from an absent example sentence is an exercise about nothing.
func GenerateForSense(sense GenSense, pool []GenSense) []GenExercise {
	lemma := strings.TrimSpace(sense.Lemma)
	if lemma == "" || strings.TrimSpace(sense.Definition) == "" {
		return nil
	}

	exercises := make([]GenExercise, 0, 5)
	if exercise, ok := genFlashcard(sense); ok {
		exercises = append(exercises, exercise)
	}
	if exercise, ok := genMultipleChoice(sense, pool); ok {
		exercises = append(exercises, exercise)
	}
	if exercise, ok := genGapFill(sense); ok {
		exercises = append(exercises, exercise)
	}
	if exercise, ok := genListenType(sense); ok {
		exercises = append(exercises, exercise)
	}
	if exercise, ok := genContextChoice(sense, pool); ok {
		exercises = append(exercises, exercise)
	}
	if exercise, ok := genReorder(sense); ok {
		exercises = append(exercises, exercise)
	}
	return exercises
}

func genFlashcard(sense GenSense) (GenExercise, bool) {
	examples := make([]map[string]any, 0, len(sense.Examples))
	for _, example := range sense.Examples {
		row := map[string]any{keySentence: example.Sentence}
		if example.SentenceVi != "" {
			row["sentence_vi"] = example.SentenceVi
		}
		examples = append(examples, row)
	}

	config := map[string]any{
		keyPrompt:     "Review this word:",
		keyTargetWord: sense.Lemma,
		keyDefinition: sense.Definition,
	}
	addIf(config, keyIPA, sense.IPA)
	addIf(config, keyDefinitionVi, sense.DefinitionVi)
	addIf(config, keyAudioURL, sense.AudioURL)
	if len(examples) > 0 {
		config[keyExamples] = examples
		config[keyExampleSentence] = sense.Examples[0].Sentence
	}

	return GenExercise{
		Slug:   slugFor(sense, KindFlashcard),
		Kind:   KindFlashcard,
		Config: config,
		Body: map[string]any{
			keyPrompt:        "Flashcard for " + sense.Lemma,
			keyCorrectAnswer: sense.Lemma,
			keyAcceptable:    []string{sense.Lemma},
			keyWordLemmas:    []string{sense.Lemma},
		},
	}, true
}

// genMultipleChoice asks which word carries a definition.
//
// The definition is the question and the words are the options — not the other
// way round. Given a word and four definitions, a learner can often eliminate
// three by register alone; given a definition, they have to know the word.
func genMultipleChoice(sense GenSense, pool []GenSense) (GenExercise, bool) {
	distractors := pickDistractors(sense, pool, distractorCount)
	if len(distractors) < distractorCount {
		return GenExercise{}, false
	}

	correctID := "opt_" + normaliseID(sense.Lemma)
	options := []map[string]string{{keyOptionID: correctID, keyOptionText: sense.Lemma}}
	for _, distractor := range distractors {
		options = append(options, map[string]string{
			keyOptionID:   "opt_" + normaliseID(distractor.Lemma),
			keyOptionText: distractor.Lemma,
		})
	}
	shuffle(options, seedOf(sense.Lemma, KindMultipleChoice))

	return GenExercise{
		Slug: slugFor(sense, KindMultipleChoice),
		Kind: KindMultipleChoice,
		Config: map[string]any{
			keyPrompt:  fmt.Sprintf("Which word means: %s", sense.Definition),
			keyOptions: options,
		},
		Body: map[string]any{
			keyPrompt:          fmt.Sprintf("Which word means: %s", sense.Definition),
			keyCorrectAnswer:   correctID,
			keyCorrectOptionID: correctID,
			keyAcceptable:      []string{sense.Lemma, correctID},
			keyWordLemmas:      []string{sense.Lemma},
		},
	}, true
}

// genGapFill blanks the target word out of one of its own example sentences.
func genGapFill(sense GenSense) (GenExercise, bool) {
	for _, example := range sense.Examples {
		before, after, ok := splitAround(example.Sentence, sense.Lemma)
		if !ok {
			continue
		}
		return GenExercise{
			Slug: slugFor(sense, KindGapFill),
			Kind: KindGapFill,
			Config: map[string]any{
				keyPrompt:         "Fill in the missing word:",
				keySentenceBefore: before,
				keySentenceAfter:  after,
				keyExpectedAnswer: sense.Lemma,
			},
			Body: map[string]any{
				keyPrompt:        "Complete the sentence with " + sense.Lemma,
				keyCorrectAnswer: sense.Lemma,
				keyAcceptable:    []string{sense.Lemma},
				keyWordLemmas:    []string{sense.Lemma},
			},
		}, true
	}
	return GenExercise{}, false
}

func genListenType(sense GenSense) (GenExercise, bool) {
	config := map[string]any{
		keyPrompt:    "Listen and type the word you hear:",
		keyAudioText: sense.Lemma,
		keyHint:      sense.Definition,
	}
	addIf(config, keyIPA, sense.IPA)
	addIf(config, keyAudioURL, sense.AudioURL)

	return GenExercise{
		Slug:   slugFor(sense, KindListenType),
		Kind:   KindListenType,
		Config: config,
		Body: map[string]any{
			keyPrompt:        "Spell the word you heard",
			keyCorrectAnswer: sense.Lemma,
			keyAcceptable:    []string{sense.Lemma},
			keyWordLemmas:    []string{sense.Lemma},
		},
	}, true
}

// genContextChoice shows the word in a sentence and asks what it means there.
func genContextChoice(sense GenSense, pool []GenSense) (GenExercise, bool) {
	if len(sense.Examples) == 0 {
		return GenExercise{}, false
	}
	distractors := pickDistractors(sense, pool, distractorCount)
	if len(distractors) < distractorCount {
		return GenExercise{}, false
	}

	correctID := "opt_" + normaliseID(sense.Lemma)
	options := []map[string]string{{keyOptionID: correctID, keyOptionText: sense.Definition}}
	for _, distractor := range distractors {
		options = append(options, map[string]string{
			keyOptionID:   "opt_" + normaliseID(distractor.Lemma),
			keyOptionText: distractor.Definition,
		})
	}
	shuffle(options, seedOf(sense.Lemma, KindContextChoice))

	// The second example where there is one: the first is already the gap-fill's
	// sentence, and meeting the same sentence twice in a lesson teaches the
	// sentence rather than the word.
	example := sense.Examples[0]
	if len(sense.Examples) > 1 {
		example = sense.Examples[1]
	}

	return GenExercise{
		Slug: slugFor(sense, KindContextChoice),
		Kind: KindContextChoice,
		Config: map[string]any{
			keyPrompt:     "What does the word mean in this sentence?",
			keySentence:   example.Sentence,
			keyTargetWord: sense.Lemma,
			keyOptions:    options,
		},
		Body: map[string]any{
			keyPrompt:          "Meaning of " + sense.Lemma + " in context",
			keyCorrectAnswer:   correctID,
			keyCorrectOptionID: correctID,
			keyAcceptable:      []string{sense.Lemma, correctID},
			keyWordLemmas:      []string{sense.Lemma},
		},
	}, true
}

// genReorder shuffles an example sentence's words.
//
// Skipped for very short sentences: three tokens can be put in order by
// accident, and a learner gets no signal from having done so.
func genReorder(sense GenSense) (GenExercise, bool) {
	const minTokens = 5

	// The first example long enough to be worth reordering. Its index is not in
	// the slug: keying on it would move the exercise's identity whenever an
	// earlier short example was replaced.
	for _, example := range sense.Examples {
		tokens := strings.Fields(example.Sentence)
		if len(tokens) < minTokens {
			continue
		}
		shuffled := append([]string(nil), tokens...)
		shuffle(shuffled, seedOf(sense.Lemma, KindReorder))
		// A shuffle that happens to land on the original order is not an
		// exercise; nudging it keeps the deterministic seed and fixes the case.
		if equalStrings(shuffled, tokens) {
			shuffled[0], shuffled[len(shuffled)-1] = shuffled[len(shuffled)-1], shuffled[0]
		}

		return GenExercise{
			Slug: slugFor(sense, KindReorder),
			Kind: KindReorder,
			Config: map[string]any{
				keyPrompt:     "Put the words in the right order:",
				keyTokens:     shuffled,
				keyTargetWord: sense.Lemma,
			},
			Body: map[string]any{
				keyPrompt:        "Rebuild the sentence",
				keyCorrectAnswer: example.Sentence,
				keyWordLemmas:    []string{sense.Lemma},
			},
		}, true
	}
	return GenExercise{}, false
}

// GenerateMatch pairs a group of words with their definitions.
func GenerateMatch(group []GenSense) (GenExercise, bool) {
	if len(group) < 2 {
		return GenExercise{}, false
	}

	words := make([]map[string]string, 0, len(group))
	definitions := make([]map[string]string, 0, len(group))
	pairs := map[string]string{}
	lemmas := make([]string, 0, len(group))

	for _, sense := range group {
		id := normaliseID(sense.Lemma)
		words = append(words, map[string]string{keyOptionID: "w_" + id, keyOptionText: sense.Lemma})
		definitions = append(definitions,
			map[string]string{keyOptionID: "d_" + id, keyOptionText: sense.Definition})
		pairs["w_"+id] = "d_" + id
		lemmas = append(lemmas, sense.Lemma)
	}

	// The two columns are shuffled with different seeds, or the answer is the
	// visible order and the exercise is a formality.
	key := strings.Join(lemmas, "-")
	shuffle(words, seedOf(key, "match-words"))
	shuffle(definitions, seedOf(key, "match-definitions"))

	return GenExercise{
		Slug: "gen-vocab-match-" + normaliseID(key),
		Kind: KindMatch,
		Config: map[string]any{
			keyPrompt:      "Match each word with its meaning:",
			keyWords:       words,
			keyDefinitions: definitions,
		},
		Body: map[string]any{
			keyPrompt:       "Match each word with its meaning",
			keyCorrectPairs: pairs,
			keyWordLemmas:   lemmas,
		},
	}, true
}

// pickDistractors chooses plausible wrong answers.
//
// Same part of speech first, then nearest CEFR level: "leisure" against three
// other nouns at B1 is a question about meaning, while "leisure" against a verb
// and two A1 words is a question about grammar the learner can win without
// knowing the word.
func pickDistractors(sense GenSense, pool []GenSense, want int) []GenSense {
	// Two tiers, not one sorted list. Sorting by part of speech and then
	// shuffling the whole thing destroys the sort — which is what happened, and
	// what put a verb among three nouns in an exercise about a noun. Shuffling
	// happens *within* a tier, so variety never costs the preference.
	var same, other []GenSense
	for _, candidate := range pool {
		if strings.EqualFold(candidate.Lemma, sense.Lemma) {
			continue
		}
		if strings.TrimSpace(candidate.Definition) == "" {
			continue
		}
		if candidate.POS == sense.POS {
			same = append(same, candidate)
		} else {
			other = append(other, candidate)
		}
	}

	// Within a tier: nearest CEFR level first, then alphabetically so the order
	// is defined rather than whatever the query returned.
	byCloseness := func(tier []GenSense) {
		sort.SliceStable(tier, func(i, j int) bool {
			iNear := cefrDistance(tier[i].CEFRLevel, sense.CEFRLevel)
			jNear := cefrDistance(tier[j].CEFRLevel, sense.CEFRLevel)
			if iNear != jNear {
				return iNear < jNear
			}
			return tier[i].Lemma < tier[j].Lemma
		})
	}
	byCloseness(same)
	byCloseness(other)

	// A shortlist of the closest few, shuffled, so the same word is not the
	// distractor in every exercise while still being a plausible one.
	shortlist := func(tier []GenSense, salt string) []GenSense {
		limit := want * 3
		if len(tier) > limit {
			tier = tier[:limit]
		}
		picked := append([]GenSense(nil), tier...)
		shuffle(picked, seedOf(sense.Lemma, "distractors", salt))
		return picked
	}

	// Same part of speech first, and only then anything else: a same-POS
	// distractor asks about meaning, while a cross-POS one can be eliminated on
	// grammar alone.
	picked := append(shortlist(same, "same"), shortlist(other, "other")...)
	if len(picked) > want {
		picked = picked[:want]
	}
	return picked
}

var cefrOrder = map[string]int{"A1": 1, "A2": 2, "B1": 3, "B2": 4, "C1": 5, "C2": 6}

func cefrDistance(a, b string) int {
	left, okLeft := cefrOrder[a]
	right, okRight := cefrOrder[b]
	if !okLeft || !okRight {
		return 99
	}
	if left > right {
		return left - right
	}
	return right - left
}

// splitAround finds the target word in a sentence and returns what surrounds it.
//
// Matches an inflected form — "achieved" for "achieve" — because the sentences
// use the word naturally rather than in its dictionary form. The blank then
// expects the inflection, which is the harder and more useful question.
func splitAround(sentence, lemma string) (before, after string, ok bool) {
	words := strings.Fields(sentence)
	target := strings.ToLower(lemma)

	for i, word := range words {
		bare := strings.ToLower(strings.Trim(word, ".,!?;:\"'"))
		if bare == target || (strings.HasPrefix(bare, target) && len(bare)-len(target) <= 3) {
			return strings.Join(words[:i], " "), strings.Join(words[i+1:], " "), true
		}
	}
	return "", "", false
}

// shuffle is a deterministic Fisher-Yates driven by the given seed.
func shuffle[T any](items []T, seed uint64) {
	state := seed | 1
	for i := len(items) - 1; i > 0; i-- {
		// xorshift64: enough mixing for presentation order, and reproducible
		// without pulling in math/rand and its global state.
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		//nolint:gosec // the modulus bounds this by i+1, which is a slice length
		j := int(state % uint64(i+1))
		items[i], items[j] = items[j], items[i]
	}
}

func seedOf(parts ...string) uint64 {
	hash := fnv.New64a()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return hash.Sum64()
}

func slugFor(sense GenSense, kind string) string {
	return fmt.Sprintf("gen-%s-%s-%s", kind, normaliseID(sense.Lemma), normaliseID(sense.POS))
}

// normaliseID reduces a word to something safe in a slug and an option id.
func normaliseID(s string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			out.WriteByte('-')
		}
	}
	return out.String()
}

func addIf(target map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		target[key] = value
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// MarshalBody encodes an exercise's authored side.
func (e GenExercise) MarshalBody() (json.RawMessage, error) {
	return json.Marshal(e.Body)
}

// MarshalConfig encodes the learner-facing side.
func (e GenExercise) MarshalConfig() (json.RawMessage, error) {
	return json.Marshal(e.Config)
}
