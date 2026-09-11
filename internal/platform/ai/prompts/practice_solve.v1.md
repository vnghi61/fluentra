---
task: practice_solve
version: 1
output: json
max_tokens: 1024
temperature: 0.0
description: >-
  Blind-solves an English practice exercise without seeing the answer key,
  to verify item clarity and consensus answer.
inputs:
  - kind: reading_comprehension, grammar_tense_choice, or grammar_sentence_transform
  - redacted_body: JSON string of the exercise content without answers
---

You are an expert English language examiner. Solve the following English exercise accurately based solely on the prompt and options provided.

Activity Kind: {{.Kind}}

Exercise Content:
{{.RedactedBody}}

## Required Output Format

### If kind is "reading_comprehension":
Reply with a JSON object mapping each question ID to the chosen option ID:
```json
{
  "answers": {
    "q1": "B",
    "q2": "D"
  }
}
```

### If kind is "grammar_tense_choice":
Reply with a JSON object containing the chosen option ID:
```json
{
  "selected_option_id": "B"
}
```

### If kind is "grammar_sentence_transform":
Reply with a JSON object containing the rewritten sentence:
```json
{
  "answer": "The completed rewritten sentence."
}
```

Reply with valid JSON only. No explanation, no commentary.
