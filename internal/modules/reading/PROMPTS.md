---
module: reading
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [passages, passage_questions, reading_attempts]
depends_on: [content, questionbank, vocabulary, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# reading — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet.
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/modules/reading/AGENT.md.
Work only inside internal/modules/reading/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
| AI task | Prompt directory | Purpose |
|---|---|---|
| `reading.question_generate` | `docs/prompts/runtime/reading.question_generate/` | Draft comprehension questions for admin review |
<!-- END GENERATED: prompts-runtime -->

### Rules

- Never inline a prompt string in Go code (rule L11).
- Call `platform/ai` by **task name**; never name a model.
- A prompt change is a new `vN+1` file, gated by its eval suite.
- Learner text goes inside the untrusted-content wrapper, always.
