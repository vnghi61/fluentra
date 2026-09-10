---
task: vocab_enrich_examples
version: 1
output: json
max_tokens: 1024
temperature: 0.3
description: >-
  Generates additional, non-repetitive example sentences with Vietnamese translations
  for an already verified vocabulary sense.
inputs:
  - term: the vocabulary word or phrase
  - part_of_speech: part of speech
  - definition: meaning of the word in this sense
  - existing_examples: previously generated sentences that should not be repeated
  - count: number of new sentences to write
---

You write example sentences for an English vocabulary entry that is already verified.

## The word and sense

- Term: {{.Term}}
- Part of speech: {{.PartOfSpeech}}
- Definition: {{.Definition}}

## Existing examples (DO NOT REPEAT OR CLOSELY COPY THESE)

{{.ExistingExamples}}

## Instructions

1. Write exactly {{.Count}} new, distinct example sentences using the term "{{.Term}}" in the sense defined above.
2. Each sentence must:
   - contain the word or a common inflected form,
   - stand alone without further context,
   - be at CEFR A2–B1: natural, everyday English,
   - be structurally different from each other and completely different from the existing examples listed above.
3. For each sentence, provide an accurate, natural Vietnamese translation.

## Output

Reply with JSON and nothing else. No markdown fence, no commentary.

```json
{
  "examples": [
    {
      "sentence": "She was meticulous about keeping her desk organized.",
      "sentence_vi": "Cô ấy rất tỉ mỉ trong việc giữ bàn làm việc ngăn nắp."
    }
  ]
}
```
