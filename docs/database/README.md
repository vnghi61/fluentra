---
doc_type: folder_index
folder: docs/database
last_verified: 2026-08-06
---

# docs/database — Database

## Purpose

Schema conventions, indexing strategy and the ER diagrams.

## Contents

- `conventions.md` — naming, types, constraints (mirrors `/DATABASE_GUIDELINE.md` with worked examples)
- `indexing.md` — how to choose an index, with `EXPLAIN` walkthroughs
- `migrations.md` — goose usage, expand→migrate→contract, concurrent index creation
- `partitioning.md` — which tables, why, and how partitions are managed
- `er/` — one ER diagram per schema
- `data-inventory.md` — every table that holds personal data, and its retention

## How AI agents should use this folder

Read `conventions.md` and `migrations.md` before writing a migration. Read `er/<schema>.md` to understand a schema without opening every migration file.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
