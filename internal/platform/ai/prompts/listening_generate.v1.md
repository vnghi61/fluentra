---
task: listening_generate
version: 1
output: json
cache: false
max_tokens: 2048
temperature: 0.3
description: >-
  Generates a single English listening comprehension item with audio script,
  voice selection, and comprehension questions with answer keys and Vietnamese explanations.
inputs:
  - cefr_level: A2, B1, or B2
---

You generate a high-quality English listening comprehension exercise for learners at CEFR level {{.CEFRLevel}}.

## Requirements

1. **Title**: A concise title describing the conversation or monologue (e.g. "Airport Announcement", "Booking a Train Ticket").
2. **Script**: An engaging, natural spoken dialogue or monologue of 100-200 words suitable for audio synthesis at CEFR {{.CEFRLevel}}.
3. **Voice**: Default to "en-US-Standard-C".
4. **Questions**: Between 4 and 5 multiple-choice questions testing comprehension of the script.
   - Each question must have:
     - "id": "q1", "q2", etc.
     - "type": "multiple_choice"
     - "prompt": question text
     - "options": exactly 4 options with distinct IDs ("A", "B", "C", "D") and distinct texts.
     - "correct_option_id": the one correct option ("A", "B", "C", or "D").
     - "explanation": with "explanation_en" and non-empty "explanation_vi" explaining why the answer is correct based on the script.

## JSON Format

Reply with valid JSON only in the following schema:
```json
{
  "title": "Booking a Conference Room",
  "script": "Receptionist: Good afternoon, how can I help you today? Guest: Hi, I would like to reserve the small meeting room for tomorrow morning from nine to eleven. Receptionist: Certainly. Let me check the schedule for room B...",
  "voice": "en-US-Standard-C",
  "questions": [
    {
      "id": "q1",
      "type": "multiple_choice",
      "prompt": "What does the guest want to reserve?",
      "options": [
        {"id": "A", "text": "A hotel room"},
        {"id": "B", "text": "A conference room"},
        {"id": "C", "text": "A table at a restaurant"},
        {"id": "D", "text": "A flight ticket"}
      ],
      "correct_option_id": "B",
      "explanation": {
        "explanation_en": "The guest asks to reserve the meeting room for tomorrow.",
        "explanation_vi": "Khách muốn đặt phòng họp cho buổi sáng ngày mai."
      }
    }
  ]
}
```

## Quality Rules
1. All 4 options per question must be completely distinct.
2. The correct option ID must exist among the options.
3. The Vietnamese explanation ("explanation_vi") must NEVER be empty.
4. Reply with valid JSON only. No markdown formatting outside json, no commentary.
