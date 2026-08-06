---
doc_type: folder_index
folder: docs/architecture
last_verified: 2026-08-06
---

# docs/architecture — Architecture

## Purpose

System-wide structure: how the parts fit together, where the boundaries are, and why.

## Contents

- `00-plan-review.md` — the review of the original brief and the optimisations applied (Vietnamese)
- `boundaries.md` — the five module boundary rules and how CI enforces them
- `microservice-migration.md` — trigger conditions, extraction order and mechanics
- `c4/` — Mermaid sources for the context, container and component diagrams
- `quality-attributes.md` — the scenarios the architecture is designed to satisfy
- `capacity-model.md` — traffic assumptions and where they break

## How AI agents should use this folder

Read before proposing **any** structural change: a new module, a new dependency arrow, a new datastore, or a change to how modules communicate. If your change contradicts something here, you need an ADR, not a workaround.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
