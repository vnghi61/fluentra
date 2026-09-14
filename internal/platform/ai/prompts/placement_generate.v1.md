---
task: placement_generate
version: 1
output: json
cache: false
max_tokens: 2048
temperature: 0.3
description: >-
  Generates a single English placement exercise item across A1 to C1 for vocabulary,
  grammar_tense_choice, reading_comprehension, listening_comprehension, writing_prompt, or speaking_task.
inputs:
  - kind: vocabulary, grammar_tense_choice, reading_comprehension, listening_comprehension, writing_prompt, or speaking_task
  - cefr_level: A1, A2, B1, B2, or C1
---

You generate a high-quality English placement test item for learners at CEFR level {{.CEFRLevel}}.

The activity kind is: {{.Kind}}.

## Instructions by Kind

### 1. If kind is "vocabulary":
- Write a single multiple-choice question testing vocabulary knowledge appropriate for CEFR {{.CEFRLevel}}.
- Output in the multiple-choice format with exactly 4 options ("A", "B", "C", "D").
- Schema:
```json
{
  "prompt": "Choose the word that best completes the sentence: The company needs to ___ costs to remain profitable.",
  "options": [
    {"id": "A", "text": "curtail"},
    {"id": "B", "text": "expand"},
    {"id": "C", "text": "prolong"},
    {"id": "D", "text": "elevate"}
  ],
  "correct_option_id": "A",
  "explanation": {
    "explanation_en": "'Curtail' means to reduce or restrict, fitting the business context of saving money.",
    "explanation_vi": "'Curtail' nghĩa là cắt giảm hoặc hạn chế, phù hợp với ngữ cảnh doanh nghiệp tiết kiệm chi phí."
  }
}
```

### 2. If kind is "grammar_tense_choice":
- Write a single multiple-choice question testing grammatical structures appropriate for CEFR {{.CEFRLevel}}.
- Output with exactly 4 options ("A", "B", "C", "D").
- Schema:
```json
{
  "prompt": "If she ___ the train on time, she would have arrived before noon.",
  "options": [
    {"id": "A", "text": "had caught"},
    {"id": "B", "text": "caught"},
    {"id": "C", "text": "has caught"},
    {"id": "D", "text": "catches"}
  ],
  "correct_option_id": "A",
  "explanation": {
    "explanation_en": "Third conditional requires past perfect ('had caught') in the if-clause.",
    "explanation_vi": "Câu điều kiện loại 3 dùng quá khứ hoàn thành ('had caught') trong mệnh đề if."
  }
}
```

### 3. If kind is "reading_comprehension":
- Write an original passage of 60–180 words suited for CEFR {{.CEFRLevel}}.
- Write exactly 3 multiple-choice questions testing comprehension.
- Each question must have:
  - "id": "q1", "q2", "q3"
  - "type": "multiple_choice"
  - "prompt": question text
  - "options": 4 options with IDs "A", "B", "C", "D"
  - "correct_option_id": correct option ID
  - "explanation": with "explanation_en" and non-empty "explanation_vi"
- Schema:
```json
{
  "passage_title": "Urban Greening Projects",
  "passage": "Across major cities, urban greening initiatives are transforming concrete spaces into micro-parks...",
  "questions": [
    {
      "id": "q1",
      "type": "multiple_choice",
      "prompt": "What is the primary objective of urban greening?",
      "options": [
        {"id": "A", "text": "To increase real estate taxes"},
        {"id": "B", "text": "To reduce urban heat and enhance biodiversity"},
        {"id": "C", "text": "To replace all roads with pathways"},
        {"id": "D", "text": "To prevent people from commuting"}
      ],
      "correct_option_id": "B",
      "explanation": {
        "explanation_en": "The passage highlights cooling cities and supporting urban wildlife.",
        "explanation_vi": "Đoạn văn nhấn mạnh việc giảm nhiệt độ đô thị và hỗ trợ đa dạng sinh học."
      }
    }
  ]
}
```

### 4. If kind is "listening_comprehension":
- Write a concise title, an audio script of 40–120 words (20–60 seconds spoken) appropriate for CEFR {{.CEFRLevel}}, voice "en-US-Standard-C", and exactly 3 multiple-choice questions.
- Schema:
```json
{
  "title": "Train Station Platform Announcement",
  "script": "Attention passengers on platform 3. The 10:15 express service to Manchester has been delayed by 15 minutes due to signaling problems...",
  "voice": "en-US-Standard-C",
  "questions": [
    {
      "id": "q1",
      "type": "multiple_choice",
      "prompt": "Why is the train delayed?",
      "options": [
        {"id": "A", "text": "Signaling problems"},
        {"id": "B", "text": "Severe snowstorms"},
        {"id": "C", "text": "Engine maintenance"},
        {"id": "D", "text": "Driver sickness"}
      ],
      "correct_option_id": "A",
      "explanation": {
        "explanation_en": "The announcement states signaling problems as the delay cause.",
        "explanation_vi": "Thông báo nêu rõ sự cố tín hiệu là nguyên nhân gây chậm chuyến."
      }
    }
  ]
}
```

### 5. If kind is "writing_prompt":
- Write a 60–100 word task prompt appropriate for CEFR {{.CEFRLevel}}.
- Include "prompt", "model_answer", "min_words" (60), "time_limit_minutes" (15), "explanation" with "explanation_en" and "explanation_vi".
- Schema:
```json
{
  "prompt": "Write a short email to your professor explaining why you cannot attend tomorrow's lecture and asking for the class notes.",
  "model_answer": "Dear Professor Smith,\n\nI am writing to apologize that I will be unable to attend tomorrow's lecture due to a sudden doctor's appointment. Could you please let me know if the lecture slides or notes will be posted online? I will ensure I review them thoroughly.\n\nThank you for your understanding.\n\nSincerely,\nAlex",
  "min_words": 60,
  "time_limit_minutes": 15,
  "explanation": {
    "explanation_en": "A formal and polite apology email covering reason, request for material, and courteous sign-off.",
    "explanation_vi": "Email xin phép lịch sự và trang trọng nêu rõ lý do, xin tài liệu học và lời chào kết phù hợp."
  }
}
```

### 6. If kind is "speaking_task":
- Write a speaking task prompt requiring a 45-second spoken response at CEFR {{.CEFRLevel}}.
- Output with "task_type": "respond", "prompt", "speaking_time_seconds": 45, "explanation".
- Schema:
```json
{
  "task_type": "respond",
  "prompt": "Describe a memorable celebration or holiday you attended with your friends or family. Mention when it happened and why it was memorable to you.",
  "speaking_time_seconds": 45,
  "explanation": {
    "explanation_en": "Clear narrative structure covering the occasion, setting, and personal significance within the time limit.",
    "explanation_vi": "Cấu trúc kể chuyện rõ ràng nêu được dịp lễ, bối cảnh và ý nghĩa cá nhân trong thời gian quy định."
  }
}
```

## Quality Rules
1. Reply with valid JSON only. Do NOT wrap in markdown quotes if possible, or use standard JSON.
2. Every multiple choice question must have exactly ONE unambiguous correct option.
3. Every explanation must contain a non-empty explanation_vi.
