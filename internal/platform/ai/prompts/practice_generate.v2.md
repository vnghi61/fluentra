---
task: practice_generate
version: 2
output: json
cache: false
max_tokens: 2048
temperature: 0.3
description: >-
  Generates a single English practice or exam item with answer key and Vietnamese explanation
  for reading comprehension, grammar tense choice, grammar sentence transformation, writing prompt,
  or speaking task.
inputs:
  - kind: reading_comprehension, grammar_tense_choice, grammar_sentence_transform, writing_prompt, or speaking_task
  - cefr_level: A2, B1, or B2
  - task_type: read_aloud or respond (when kind is speaking_task)
---

You generate a high-quality English practice or exam item for learners at CEFR level {{.CEFRLevel}}.

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

### 4. If kind is "writing_prompt":
- Write an essay prompt suitable for a 150-word written response at CEFR {{.CEFRLevel}}.
- Include a high-scoring model answer of 130-180 words.
- JSON structure:
```json
{
  "prompt": "Some people think that working from home is more effective than working in an office. Discuss your opinion and give reasons.",
  "model_answer": "In recent years, remote work has become widely adopted across many industries. In my opinion, working from home provides notable benefits, particularly regarding flexibility and productivity. Without daily commuting, professionals save significant time and energy, allowing them to focus better on their tasks. However, in-person collaboration remains valuable for team cohesion. Overall, a balanced hybrid model offers the greatest advantages.",
  "min_words": 150,
  "time_limit_minutes": 20,
  "topic": "Work & Careers",
  "explanation": {
    "explanation_en": "A strong response clearly takes a position and provides supporting arguments and examples.",
    "explanation_vi": "Một bài viết tốt cần nêu rõ quan điểm cá nhân và đưa ra các lý lẽ, ví dụ hỗ trợ."
  }
}
```

### 5. If kind is "speaking_task":
- Check the task type: either "read_aloud" or "respond" (defaults to "respond" if unspecified).
- For "read_aloud":
  - Provide a cohesive, natural paragraph of 35-60 words suitable to be read aloud in 45 seconds.
  - Set "task_type": "read_aloud".
  - Set "reference_text": the text to be read.
  - Set "prompt": "Read the following text aloud clearly and naturally."
  - Set "speaking_time_seconds": 45.
- For "respond":
  - Provide a clear, engaging spoken response prompt suitable for a 45-second spoken answer.
  - Set "task_type": "respond".
  - Set "prompt": the question/prompt text.
  - Set "speaking_time_seconds": 45.
- JSON structure for "read_aloud":
```json
{
  "task_type": "read_aloud",
  "prompt": "Read the following text aloud clearly and naturally.",
  "reference_text": "Good morning and welcome to the regional museum. Please be reminded that photography with flash is prohibited in all exhibition galleries. Audio guides are available at the front desk in five languages.",
  "speaking_time_seconds": 45,
  "explanation": {
    "explanation_en": "Focus on clear pronunciation, rhythm, and natural sentence pauses.",
    "explanation_vi": "Tập trung vào phát âm rõ ràng, nhịp điệu và ngắt nghỉ tự nhiên theo dấu câu."
  }
}
```
- JSON structure for "respond":
```json
{
  "task_type": "respond",
  "prompt": "Describe an important celebration or holiday in your country. What do people usually do on this day?",
  "speaking_time_seconds": 45,
  "explanation": {
    "explanation_en": "State the holiday, explain its significance, and describe typical customs and food.",
    "explanation_vi": "Nêu tên ngày lễ, giải thích ý nghĩa và miêu tả các phong tục, món ăn đặc trưng."
  }
}
```

## Quality Rules
1. All options for multiple choice must be distinct after normalisation.
2. The correct option must be among the provided options.
3. The Vietnamese explanation ("explanation_vi") must NEVER be empty.
4. Reply with valid JSON only. No markdown formatting outside json, no commentary.
