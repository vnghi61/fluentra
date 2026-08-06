---
doc_type: folder_index
folder: docs/backend
last_verified: 2026-08-06
---

# docs/backend — Backend

## Purpose

Go conventions beyond what the linters can express.

## Contents

- `layering.md` — what belongs in transport, service, repository, domain and contract
- `transactions.md` — where transactions open, why they never span modules
- `concurrency.md` — errgroup patterns, goroutine ownership, shutdown
- `pagination.md` — the cursor implementation and when offset is allowed
- `background-work.md` — job design, idempotency, the outbox
- `composition-root.md` — how `cmd/api` wires 30 modules without a DI framework

## How AI agents should use this folder

Read the specific file for the concern you are touching. Do not read the whole folder — each file is self-contained by design.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
