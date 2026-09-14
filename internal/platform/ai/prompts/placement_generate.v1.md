---
task: placement_generate
version: 1
output: json
cache: false
max_tokens: 2048
temperature: 0.3
description: >-
  Writes one English placement test item at a CEFR level from A1 to C1: a
  vocabulary or grammar question, a reading passage or listening script with
  three questions, a short writing task, or a 45-second speaking task.
inputs:
  - Kind: vocabulary, grammar_tense_choice, reading_comprehension, listening_comprehension, writing_prompt or speaking_task
  - CEFRLevel: A1, A2, B1, B2 or C1
---

You write one item for an English placement test that places Vietnamese learners
between A1 and C1. The item is at CEFR level {{.CEFRLevel}}: a learner at that
level should find it hard but manageable, a learner one level below should
mostly fail it, and a learner one level above should mostly pass it.

The kind of item is: {{.Kind}}.

Reply with one JSON object and nothing else. Every explanation has a non-empty
"explanation_en" and "explanation_vi". Every multiple-choice question has exactly
one correct option, and no option is a trick or a near-duplicate of another.

## vocabulary

One question testing a word or phrase a {{.CEFRLevel}} learner should know, with
exactly four options "A" to "D". Use this shape; it is graded as a single choice.

```json
{
  "prompt": "I can't pay for lunch today. Could you ___ me some money?",
  "options": [
    {"id": "A", "text": "lend"},
    {"id": "B", "text": "borrow"},
    {"id": "C", "text": "owe"},
    {"id": "D", "text": "spend"}
  ],
  "correct_option_id": "A",
  "explanation": {
    "explanation_en": "You lend money to someone; you borrow it from someone.",
    "explanation_vi": "Lend là cho ai mượn; borrow là mượn của ai."
  }
}
```

## grammar_tense_choice

One question testing a grammatical structure of {{.CEFRLevel}}, with exactly four
options "A" to "D", in the same shape as vocabulary.

## reading_comprehension

An original passage and exactly three questions about it, each with three or
four options. The passage length depends on the level:

| Level | Words |
|---|---|
| A1 | 60–90 |
| A2 | 80–110 |
| B1 | 100–140 |
| B2 | 120–160 |
| C1 | 140–180 |

```json
{
  "passage_title": "A new library",
  "passage": "…",
  "questions": [
    {
      "id": "q1",
      "type": "multiple_choice",
      "prompt": "Why did the town build a new library?",
      "options": [
        {"id": "A", "text": "…"},
        {"id": "B", "text": "…"},
        {"id": "C", "text": "…"},
        {"id": "D", "text": "…"}
      ],
      "correct_option_id": "B",
      "explanation": {"explanation_en": "…", "explanation_vi": "…"}
    }
  ]
}
```

The question ids are "q1", "q2" and "q3". A question is answered from the passage,
never from general knowledge.

## listening_comprehension

A script for a clip of 20 to 60 seconds, spoken by one voice, and exactly three
questions in the reading shape. The script length depends on the level:

| Level | Words |
|---|---|
| A1 | 45–70 |
| A2 | 55–85 |
| B1 | 70–105 |
| B2 | 85–125 |
| C1 | 100–140 |

```json
{
  "title": "Platform announcement",
  "script": "…",
  "voice": "en-US-Standard-C",
  "questions": [ … ]
}
```

Write the script as it is heard: no stage directions, no speaker labels.

## writing_prompt

A task asking for 60 to 100 words at {{.CEFRLevel}}: an email, a message, a short
opinion or a description. The prompt is at least fifteen words. The model answer
is at least eighty words and is what a strong {{.CEFRLevel}} learner would write.

```json
{
  "prompt": "Write an email to a friend about a trip you took last month: where you went, what you did and whether you would go again.",
  "model_answer": "…",
  "min_words": 60,
  "time_limit_minutes": 8,
  "explanation": {"explanation_en": "…", "explanation_vi": "…"}
}
```

"min_words" is between 60 and 100.

## speaking_task

A prompt for a spoken response of 45 seconds at {{.CEFRLevel}}, at least eight
words long, asking the learner to describe, explain or give an opinion.

```json
{
  "task_type": "respond",
  "prompt": "Describe a place in your city you like to visit and explain why you like it.",
  "speaking_time_seconds": 45,
  "explanation": {"explanation_en": "…", "explanation_vi": "…"}
}
```
