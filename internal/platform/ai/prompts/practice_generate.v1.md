---
task: practice_generate
version: 1
output: json
max_tokens: 2048
temperature: 0.3
description: >-
  Generates a single English practice item with answer key and Vietnamese explanation
  for reading comprehension, grammar tense choice, or grammar sentence transformation.
inputs:
  - kind: reading_comprehension, grammar_tense_choice, or grammar_sentence_transform
  - cefr_level: A2, B1, or B2
---

You generate a high-quality English practice item for learners at CEFR level {{.CEFRLevel}}.

The activity kind is: {{.Kind}}.

## Instructions by Kind

### 1. If kind is "reading_comprehension":
- Write an engaging, original passage of 120-220 words suitable for CEFR {{.CEFRLevel}}.
- Write between 4 and 6 multiple-choice questions testing comprehension of the passage.
- Each question must have:
  - "id": "q1", "q2", etc.
  - "type": "multiple_choice"
  - "prompt": question text
  - "options": exactly 4 options with distinct IDs ("A", "B", "C", "D") and distinct texts.
  - "correct_option_id": the one correct option ("A", "B", "C", or "D").
  - "explanation": with "explanation_en" and non-empty "explanation_vi" explaining why the answer is correct with evidence from the text.
- JSON structure:
```json
{
  "passage_title": "Title of Passage",
  "passage": "Full passage text...",
  "questions": [
    {
      "id": "q1",
      "type": "multiple_choice",
      "prompt": "What did the author discover?",
      "options": [
        {"id": "A", "text": "Option A text"},
        {"id": "B", "text": "Option B text"},
        {"id": "C", "text": "Option C text"},
        {"id": "D", "text": "Option D text"}
      ],
      "correct_option_id": "B",
      "explanation": {
        "explanation_en": "Paragraph 2 states that...",
        "explanation_vi": "Đoạn 2 chỉ ra rằng..."
      }
    }
  ]
}
```

### 2. If kind is "grammar_tense_choice":
- Write a clear sentence testing appropriate verb tense or grammatical form at CEFR {{.CEFRLevel}}.
- Indicate the blank in the prompt with "___".
- Provide exactly 4 options with distinct IDs ("A", "B", "C", "D") and distinct texts.
- Specify "correct_option_id".
- Provide "explanation" with "explanation_en" and non-empty "explanation_vi".
- JSON structure:
```json
{
  "prompt": "By the time we arrived, the concert ___ already.",
  "options": [
    {"id": "A", "text": "has started"},
    {"id": "B", "text": "had started"},
    {"id": "C", "text": "starts"},
    {"id": "D", "text": "is starting"}
  ],
  "correct_option_id": "B",
  "explanation": {
    "explanation_en": "Past perfect 'had started' is used for an action completed before another past event.",
    "explanation_vi": "Thì quá khứ hoàn thành 'had started' diễn tả hành động đã hoàn thành trước một sự việc khác trong quá khứ."
  }
}
```

### 3. If kind is "grammar_sentence_transform":
- Write a sentence transformation drill at CEFR {{.CEFRLevel}} (e.g. passive voice, reported speech, conditionals, inversion, or conjunction rewrites).
- The prompt must clearly state the original sentence and the instruction / starting cue.
- "correct_answer" must NOT appear in the prompt text.
- Provide "acceptable" array of valid alternative phrasings (if any).
- Provide "explanation" with "explanation_en" and non-empty "explanation_vi".
- JSON structure:
```json
{
  "prompt": "Rewrite the sentence beginning with 'Although': He was tired, but he finished the report.",
  "correct_answer": "Although he was tired, he finished the report.",
  "acceptable": [
    "Although he was tired, he finished his report."
  ],
  "explanation": {
    "explanation_en": "Use 'Although' at the beginning of the clause without 'but' in the main clause.",
    "explanation_vi": "Dùng 'Although' ở đầu mệnh đề chỉ sự nhượng bộ và không dùng 'but' ở mệnh đề chính."
  }
}
```

## Quality Rules
1. All options for multiple choice must be distinct after normalisation.
2. The correct option must be among the provided options.
3. The Vietnamese explanation ("explanation_vi") must NEVER be empty.
4. Reply with valid JSON only. No markdown formatting, no commentary.
