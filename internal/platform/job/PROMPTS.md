---
module: job
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: ops
tables: [river_job, outbox_events, job_failures]
depends_on: [telemetry]
depended_on_by: [auth, user, audit, notification, mailer, content, writing, speaking, media, ai, srs, analytics, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# job — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet.
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/platform/job/AGENT.md.
Work only inside internal/platform/job/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
**This module makes no LLM calls.** If you think it should, check whether an algorithm solves the problem better — see [/AI_GUIDE.md](../../../AI_GUIDE.md) §B2.
<!-- END GENERATED: prompts-runtime -->

