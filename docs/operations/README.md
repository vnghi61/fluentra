---
doc_type: folder_index
folder: docs/operations
last_verified: 2026-08-06
---

# docs/operations — Operations

## Purpose

Running the system when it misbehaves.

## Contents

- `runbooks/` — one per alert: symptom, impact, diagnosis, mitigation, escalation
- `slos.md` — the service level objectives and their error budgets
- `on-call.md` — rotation, escalation, expectations
- `postmortems/` — blameless incident write-ups
- `capacity-planning.md` — headroom and the triggers to add more

## How AI agents should use this folder

Every alert must have a runbook. If you add an alert without one, you have added a page in the middle of the night with no instructions attached.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
