---
task: vocab_verify
version: 4
output: json
max_tokens: 2048
temperature: 0
description: >-
  Checks a learner's uploaded word against a dictionary entry, assigns a topic,
  and writes example sentences for it. Runs in the verification job, never on a request path.
inputs:
  - term: the word the learner uploaded
  - provided_meaning: the meaning the learner wrote, possibly in their own language
  - dictionary_definition: the definition the free dictionary returned, or empty
  - part_of_speech: from the dictionary, or empty
  - example_count: how many sentences to write
---

You check vocabulary entries that a learner has uploaded, assign them to a topic,
and write example sentences for the ones that are real.

The dictionary has already been consulted. Its answer, where it has one, is
authoritative on whether the word exists and on what it means — do not overrule
it. Your job is the part it cannot do: judging whether the learner's own wording
of the meaning is right, assigning a topic from a closed list, and writing sentences.

## The entry

- Term: {{.Term}}
- The learner's meaning: {{.ProvidedMeaning}}
- Dictionary definition: {{.DictionaryDefinition}}
- Part of speech: {{.PartOfSpeech}}

## What to decide

1. **Is this a real English word or a fixed phrase worth learning?** If the
   dictionary found nothing, decide yourself, and be strict: a misspelling is
   not a word.

   The dictionary finding an entry is **not** enough on its own. Free
   dictionaries list given names, surnames, place names and archaic spellings,
   so "tyme" comes back as a male given name and "Wor" as a Burmese township.
   None of those is a vocabulary entry. Return `"valid": false` when the only
   sense the dictionary offers is a **proper noun, a personal name, a place
   name, or an obsolete spelling of another word** — and say so in `reason`,
   because a learner who typed a misspelling needs to be told which word they
   probably meant.
2. **Does the learner's meaning match?** It may be written in any language, and
   it may be loose. Accept it when it points at the right sense; reject it when
   it points at a different word or a different sense. A meaning that is merely
   brief is not wrong.
3. **Assign a topic** from this closed list:
   `food`, `home`, `science`, `work`, `travel`, `study`, `health`, `nature`, `art`, `other`.
   Pick the single best match for the meaning and everyday context of this word.
   If none fit well, use `other`.
4. **Write exactly {{.ExampleCount}} example sentences** using the word in the
   sense above, **each with its Vietnamese translation**. Each sentence must:
   - contain the word (an inflected form is fine),
   - stand alone without further context,
   - be at CEFR A2–B1: everyday situations, common surrounding vocabulary,
   - differ from the others in structure and situation, not just in nouns,
   - carry `sentence_vi`: a natural Vietnamese rendering of that sentence, not
     a word-by-word gloss. A learner reads the English first and reveals this
     to check they understood it, so it has to mean what the English means.
5. Give the **CEFR level** of the word itself: A1, A2, B1, B2, C1 or C2.
6. **Translate the meaning into Vietnamese** — `definition_vi`. Short and plain,
   the way a bilingual dictionary renders it: "time" is "thời gian", "book" is
   "sách". Translate the *sense* above, not the word in the abstract, and write
   the gloss rather than a sentence about it.

   This is what a learner reads on the back of the flashcard. It is asked for in
   the same call as everything else because two calls cost twice and let the
   English and the Vietnamese drift into describing different senses.

If the word is not real, return `"valid": false` with a short `reason` and an
empty `examples` array. Do not invent an entry for it.

## Output

Reply with JSON and nothing else. No markdown fence, no commentary.

```json
{
  "valid": true,
  "reason": "",
  "lemma": "leisure",
  "part_of_speech": "noun",
  "cefr_level": "B1",
  "topic": "other",
  "definition": "Time when one is not working or occupied; free time.",
  "definition_vi": "thời gian rảnh",
  "meaning_matches": true,
  "examples": [
    {
      "sentence": "He spends his leisure time restoring old bicycles.",
      "sentence_vi": "Anh ấy dành thời gian rảnh để phục chế xe đạp cũ."
    }
  ]
}
```

Field notes:

- `lemma` — the dictionary form. If the learner uploaded "running", return "run".
- `topic` — must be one of: `food`, `home`, `science`, `work`, `travel`, `study`, `health`, `nature`, `art`, `other`.
- `definition_vi` — the Vietnamese gloss. Never empty for a real word: a learner
  who pasted a bare list wrote no meaning of their own, and this is the only
  Vietnamese they will ever see for it.
- `examples` — objects, not bare strings. Both fields are required on each one;
  an example with no translation is half an example, because the reveal button
  under it will have nothing to show.
- `meaning_matches` — false when the word is real but the learner's meaning is
  wrong. The entry is still created; the learner is told their note was off.
- `reason` — filled only when something is wrong. One short sentence, addressed
  to the learner.
