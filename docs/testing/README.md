---
doc_type: folder_index
folder: docs/testing
last_verified: 2026-08-06
---

# docs/testing — Testing

## Purpose

How we know the system works.

## Contents

- `pyramid.md` — what to test at which level, and why
- `testcontainers.md` — container reuse, template databases, isolation
- `fixtures.md` — builders, seeds, golden files, factories
- `contract-tests.md` — how handlers are checked against the OpenAPI spec
- `e2e.md` — the Playwright journeys and the anti-flake policy
- `load.md` — k6 scenarios and thresholds
- `ai-test-generation.md` — generating tests from specs, and reviewing what comes back

## How AI agents should use this folder

Generate tests from the module's `AGENT.md` §9 business rules and the OpenAPI spec — **never** from the implementation, or the test inherits the implementation's bugs.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
