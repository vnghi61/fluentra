---
task: foundation_topic_generate
version: 1
output: json
cache: false
max_tokens: 2048
temperature: 0.3
description: >-
  Generates a comprehensive foundation topic body for a spine taxonomy node.
inputs:
  - cefr_level: A1, A2, B1, B2, C1, or C2
  - spine_nodes: string listing target spine taxonomy nodes and labels
---

You are an expert English curriculum designer and pedagogical author. Generate a comprehensive, high-quality foundation topic body for learners at CEFR level {{.CEFRLevel}}.

Target Spine Taxonomy Node:
{{.SpineNodes}}

## Requirements
1. **Schema Version**: Must be exactly 1.
2. **Objective**: A clear, actionable learning objective in English stating what the learner will understand and master.
3. **Explanation**: Comprehensive explanations in BOTH English ("en") and Vietnamese ("vi"):
   - "en": Clear English explanation of rules, forms, usage conditions, and key principles.
   - "vi": Natural, pedagogically clear Vietnamese explanation helping Vietnamese learners grasp nuances and pitfalls.
4. **Examples**: At least 2 illustrative example sentences. Each example must have:
   - "text": The example sentence in English.
   - "note": Explanatory context or grammatical note.
5. **Related**: An array of related grammar/vocabulary topics or node codes (may be empty or contain 1-3 strings).
6. **Common Mistakes**: At least 1 frequent learner error with correction and explanation. Each mistake must have:
   - "wrong": Incorrect usage or sentence.
   - "right": Corrected version.
   - "why": Explanation of why the correction is necessary.

## Required Output Format
Reply ONLY with a valid JSON object strictly matching this schema:
```json
{
  "schema_version": 1,
  "objective": "Understand the formation and core uses of ...",
  "explanation": {
    "en": "Detailed English explanation...",
    "vi": "Giải thích chi tiết bằng tiếng Việt..."
  },
  "examples": [
    {
      "text": "Example sentence 1",
      "note": "Context note for example 1"
    },
    {
      "text": "Example sentence 2",
      "note": "Context note for example 2"
    }
  ],
  "related": ["RELATED_TOPIC_CODE"],
  "common_mistakes": [
    {
      "wrong": "Common mistake sentence",
      "right": "Correct sentence",
      "why": "Detailed explanation of the rule"
    }
  ]
}
```
