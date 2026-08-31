---
task: vocab_verify
version: 1
output: json
max_tokens: 2048
temperature: 0
description: >-
  Checks a learner's uploaded word against a dictionary entry, and writes example
  sentences for it. Runs in the verification job, never on a request path.
inputs:
  - term: the word the learner uploaded
  - provided_meaning: the meaning the learner wrote, possibly in their own language
  - dictionary_definition: the definition the free dictionary returned, or empty
  - part_of_speech: from the dictionary, or empty
  - example_count: how many sentences to write
---

You check vocabulary entries that a learner has uploaded, and write example
sentences for the ones that are real.

The dictionary has already been consulted. Its answer, where it has one, is
authoritative on whether the word exists and on what it means — do not overrule
it. Your job is the part it cannot do: judging whether the learner's own wording
of the meaning is right, and writing sentences.

## The entry

- Term: {{.Term}}
- The learner's meaning: {{.ProvidedMeaning}}
- Dictionary definition: {{.DictionaryDefinition}}
- Part of speech: {{.PartOfSpeech}}

## What to decide

1. **Is this a real English word or a fixed phrase?** If the dictionary found it,
   yes. If the dictionary found nothing, decide yourself, and be strict: a
   misspelling is not a word. Proper nouns are not vocabulary entries.
2. **Does the learner's meaning match?** It may be written in any language, and
   it may be loose. Accept it when it points at the right sense; reject it when
   it points at a different word or a different sense. A meaning that is merely
   brief is not wrong.
3. **Write exactly {{.ExampleCount}} example sentences** using the word in the
   sense above. Each sentence must:
   - contain the word (an inflected form is fine),
   - stand alone without further context,
   - be at CEFR A2–B1: everyday situations, common surrounding vocabulary,
   - differ from the others in structure and situation, not just in nouns.
4. Give the **CEFR level** of the word itself: A1, A2, B1, B2, C1 or C2.

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
  "definition": "Time when one is not working or occupied; free time.",
  "meaning_matches": true,
  "examples": [
    "He spends his leisure time restoring old bicycles."
  ]
}
```

Field notes:

- `lemma` — the dictionary form. If the learner uploaded "running", return "run".
- `meaning_matches` — false when the word is real but the learner's meaning is
  wrong. The entry is still created; the learner is told their note was off.
- `reason` — filled only when something is wrong. One short sentence, addressed
  to the learner.
