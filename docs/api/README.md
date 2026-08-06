---
doc_type: folder_index
folder: docs/api
last_verified: 2026-08-06
---

# docs/api — API

## Purpose

HTTP contract standards and the reasoning behind them.

## Contents

- `rest-standards.md` — resources, methods, status codes (mirrors `/API_GUIDELINE.md`)
- `versioning.md` — the versioning and deprecation policy
- `pagination.md` — the cursor format and client guidance
- `errors.md` — the Problem Details contract and the full code catalogue
- `idempotency.md` — where keys are required and how replay works
- `streaming.md` — SSE conventions for long-running operations
- `webhooks.md` — inbound webhook verification and replay

## How AI agents should use this folder

Read before adding or changing an endpoint. The spec at `api/openapi/openapi.yaml` is the contract; these documents explain the rules that spec must obey.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
