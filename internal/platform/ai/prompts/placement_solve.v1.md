---
task: placement_solve
version: 1
output: json
cache: false
max_tokens: 1024
temperature: 0.0
description: >-
  Blind-solves an English placement exercise item without seeing the answer key,
  to verify item clarity and consensus answer.
inputs:
  - kind: vocabulary, grammar_tense_choice, reading_comprehension, or listening_comprehension
  - redacted_body: JSON string of the exercise content without answers
---

You are an expert English language examiner. Solve the following English placement exercise accurately based solely on the prompt and options provided.

Activity Kind: {{.Kind}}

Exercise Content:
{{.RedactedBody}}

## Required Output Format

### If kind is "reading_comprehension" or "listening_comprehension":
Reply with a JSON object mapping each question ID to the chosen option ID:
```json
{
  "answers": {
    "q1": "B",
    "q2": "D"
  }
}
```

### If kind is "vocabulary" or "grammar_tense_choice":
Reply with a JSON object containing the chosen option ID:
```json
{
  "selected_option_id": "B"
}
```

Reply with valid JSON only. No explanation, no commentary.
