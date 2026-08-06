---
doc_type: folder_index
folder: docs/frontend
last_verified: 2026-08-06
---

# docs/frontend — Frontend

## Purpose

React and TypeScript conventions for the SPA.

## Contents

- `structure.md` — feature slices and the import rules between them
- `state.md` — the state classification table and why server state never enters a store
- `data-fetching.md` — TanStack Query patterns, query keys, invalidation
- `forms.md` — React Hook Form + Zod, and mapping Problem Details onto fields
- `routing.md` — TanStack Router, typed search params, loaders
- `accessibility.md` — the WCAG 2.2 AA baseline and the exercise-specific requirements
- `performance.md` — the bundle budget and how it is enforced

## How AI agents should use this folder

Read `structure.md` and the file for your concern before writing components. Never hand-write an API type — regenerate from the OpenAPI spec.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
