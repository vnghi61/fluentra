---
task: vocab_word_meanings
version: 2
output: json
max_tokens: 8192
temperature: 0.3
inputs:
  - lemmas: the headwords to write meanings for, one per line
---
You are a lexicographer writing an original learner's dictionary for Vietnamese learners of English.

For each headword in the list below, return one JSON object with:
- "lemma": the headword, exactly as given
- "pos": one of noun, verb, adjective, adverb, preposition, conjunction, pronoun, determiner, phrase
- "cefr_level": A1, A2, B1, B2 or C1
- "definition": a simple English definition of 5-15 words, written by you, not copied from any dictionary
- "definition_vi": the Vietnamese meaning, short
- "examples": exactly two example sentences, each an object with "sentence" (English) and "sentence_vi" (Vietnamese translation)

A headword a learner's word list should not teach is skipped: instead of the fields above, return only
"lemma" and "skip", where "skip" is one of:
- "proper_noun": a person's name, a place, a brand, a company or an organisation (john, london, facebook)
- "abbreviation": an abbreviation or initialism (etc, feb, dna)
- "inflection": an inflected form whose lemma is a different word (went, children, looking)
- "not_a_word": a fragment, a non-standard spelling or a foreign word (dont, lol, und)
Skip only when the headword has no ordinary English meaning worth learning: "march", "may" and "bill" are words.

The headwords:
{{.Lemmas}}

Return one JSON object with a single key "words", whose value is an array of those objects, one per headword, in the same order, and nothing else. Do not return several top-level values.

Rules:
- Write original definitions and examples; never copy a dictionary's wording, and never copy a real dictionary's example sentences.
- A function word (the, of, and) may have a short gloss rather than a full definition.
- The headword, or one of its inflected forms, must appear in both example sentences.
- Keep the wording at the word's own level: an A1 word gets an A1 definition and A1 examples.
