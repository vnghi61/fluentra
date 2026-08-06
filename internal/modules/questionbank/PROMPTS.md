---
module: questionbank
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_options, question_sets, question_set_items, question_stats]
depends_on: [content, ai, audit, search]
depended_on_by: [exam, reading, listening, grammar, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet.
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/modules/questionbank/AGENT.md.
Work only inside internal/modules/questionbank/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
| AI task | Prompt directory | Purpose |
|---|---|---|
| `questionbank.generate_items` | `docs/prompts/runtime/questionbank.generate_items/` | Draft items from a topic, level and type, for admin review |
<!-- END GENERATED: prompts-runtime -->

### Rules

- Never inline a prompt string in Go code (rule L11).
- Call `platform/ai` by **task name**; never name a model.
- A prompt change is a new `vN+1` file, gated by its eval suite.
- Learner text goes inside the untrusted-content wrapper, always.
