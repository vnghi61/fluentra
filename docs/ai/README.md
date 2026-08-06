---
doc_type: folder_index
folder: docs/ai
last_verified: 2026-08-06
---

# docs/ai — AI engineering

## Purpose

Everything about LLMs as a component of the product.

## Contents

- `context/` — the context engineering strategy and its measurements
- `playbooks/` — agent playbooks for recurring multi-step work
- `evals/` — the evaluation harness, golden sets and thresholds
- `cost-model.md` — per-task cost, per-learner unit economics
- `provider-comparison.md` — measured quality and cost by provider and task
- `safety.md` — injection defence, moderation, output validation

## How AI agents should use this folder

Read before adding or changing anything that calls a model. `cost-model.md` and `evals/` are what make an AI feature shippable rather than merely demonstrable.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
