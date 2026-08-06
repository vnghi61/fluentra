---
module: auth
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [credentials, sessions, refresh_tokens, mfa_secrets, verification_tokens, login_attempts, oauth_identities]
depends_on: [user, rbac, audit, mailer, cache]
depended_on_by: [admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# auth — Prompts

Two kinds. Do not confuse them — see [`/PROMPT_LIBRARY.md`](../../../PROMPT_LIBRARY.md).

## 1. Development prompts — for building this module

<!-- BEGIN GENERATED: prompts-dev -->
| Task | Prompt |
|---|---|
| Add an endpoint | `docs/prompts/dev/backend/generate-crud.md` |
| Add a security integration test | `docs/prompts/dev/testing/generate-integration-test.md` |
<!-- END GENERATED: prompts-dev -->

### Context to give the agent

```
Read /AGENT.md, then internal/modules/auth/AGENT.md.
Work only inside internal/modules/auth/.
Obey rules L1–L12. Business rules are in AGENT.md §9.
Do not touch other modules; if you need something from one, use its contract package
and say so in your summary.
When done: make check, then update AGENT.md and TODO.md.
```

## 2. Runtime prompts — LLM calls this module makes

<!-- BEGIN GENERATED: prompts-runtime -->
**This module makes no LLM calls.** If you think it should, check whether an algorithm solves the problem better — see [/AI_GUIDE.md](../../../AI_GUIDE.md) §B2.
<!-- END GENERATED: prompts-runtime -->

