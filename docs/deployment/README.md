---
doc_type: folder_index
folder: docs/deployment
last_verified: 2026-08-06
---

# docs/deployment — Deployment

## Purpose

Running the system, locally and in production.

## Contents

- `configuration.md` — **every** environment variable, its default, and whether it is required
- `compose-topology.md` — services, networks, volumes, startup order
- `production-checklist.md` — what must be true before a first production deploy
- `backup-restore.md` — procedures and drill results
- `scaling.md` — the staged scaling path and its triggers

## How AI agents should use this folder

`configuration.md` is authoritative. If a config key is not there, it does not exist — add it deliberately rather than inventing one.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
