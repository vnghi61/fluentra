---
module: srs
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [review_cards, review_logs, srs_params, review_daily_stats]
depends_on: [cache, job, content]
depended_on_by: [learning, vocabulary, grammar, gamification, notification, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# srs — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet.
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/modules/srs/AGENT.md.
Work only inside internal/modules/srs/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
**This module makes no LLM calls.** If you think it should, check whether an algorithm solves the problem better — see [/AI_GUIDE.md](../../../AI_GUIDE.md) §B2.
<!-- END GENERATED: prompts-runtime -->

