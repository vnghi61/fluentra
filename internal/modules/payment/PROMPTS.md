---
module: payment
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [payments, invoices, payment_webhooks, refunds, checkout_sessions]
depends_on: [subscription, audit, job, mailer, storage]
depended_on_by: [subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# payment — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
Use the standard library in `docs/prompts/dev/`. No module-specific development prompts yet.
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/modules/payment/AGENT.md.
Work only inside internal/modules/payment/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
**This module makes no LLM calls.** If you think it should, check whether an algorithm solves the problem better — see [/AI_GUIDE.md](../../../AI_GUIDE.md) §B2.
<!-- END GENERATED: prompts-runtime -->

