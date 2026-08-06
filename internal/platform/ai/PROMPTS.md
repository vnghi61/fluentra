---
module: ai
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@ai-team"
schema: ai
tables: [ai_requests, ai_usage, prompt_versions, ai_cache_entries, ai_budgets]
depends_on: [cache, telemetry, job]
depended_on_by: [writing, speaking, grammar, questionbank, content, reading, media, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# ai — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
| Task | Prompt |
|---|---|
| Add a provider adapter | `docs/prompts/dev/ai/generate-ai-provider.md` |
| Write a runtime prompt | `docs/prompts/dev/ai/generate-prompt-template.md` |
| Build an eval suite | `docs/prompts/dev/ai/generate-eval-suite.md` |
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/platform/ai/AGENT.md.
Work only inside internal/platform/ai/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
| AI task | Prompt directory | Purpose |
|---|---|---|
| `writing.grade_essay` | `docs/prompts/runtime/writing.grade_essay/` | CEFR/IELTS rubric grading with actionable feedback |
| `writing.quick_hint` | `docs/prompts/runtime/writing.quick_hint/` | Cheap in-editor hint while drafting |
| `speaking.feedback` | `docs/prompts/runtime/speaking.feedback/` | Turn ASR output and pronunciation scores into coaching |
| `grammar.explain` | `docs/prompts/runtime/grammar.explain/` | Explain an error, citing a rule from our own taxonomy |
| `vocabulary.example_sentence` | `docs/prompts/runtime/vocabulary.example_sentence/` | Level-appropriate example sentences |
| `questionbank.generate_items` | `docs/prompts/runtime/questionbank.generate_items/` | Draft items for admin review |
| `content.level_estimate` | `docs/prompts/runtime/content.level_estimate/` | Estimate the CEFR level of a passage |
<!-- END GENERATED: prompts-runtime -->

### Rules

- Never inline a prompt string in Go code (rule L11).
- Call `platform/ai` by **task name**; never name a model.
- A prompt change is a new `vN+1` file, gated by its eval suite.
- Learner text goes inside the untrusted-content wrapper, always.
