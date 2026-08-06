---
module: learning
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [enrollments, progress, attempts, learning_sessions, placement_results, skill_mastery]
depends_on: [lesson, content, srs, cache, job]
depended_on_by: [gamification, analytics, admin, exam, vocabulary, grammar, reading, listening, speaking, writing]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# learning — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet.
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/modules/learning/AGENT.md.
Work only inside internal/modules/learning/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
| AI task | Prompt directory | Purpose |
|---|---|---|
| `learning.study_plan_suggest` | `docs/prompts/runtime/learning.study_plan_suggest/` | Suggest a weekly plan from progress and goals |
<!-- END GENERATED: prompts-runtime -->

### Rules

- Never inline a prompt string in Go code (rule L11).
- Call `platform/ai` by **task name**; never name a model.
- A prompt change is a new `vN+1` file, gated by its eval suite.
- Learner text goes inside the untrusted-content wrapper, always.
