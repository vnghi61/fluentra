---
module: srs
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [review_cards, review_logs, srs_params, review_daily_stats]
depends_on: [cache, job, content]
depended_on_by: [learning, vocabulary, grammar, gamification, notification, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# srs

Spaced repetition. Owns review cards, the FSRS scheduler, review logs, due queues and retention forecasting. Any material worth remembering long-term enters this module and comes back at the right time.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Vocabulary and grammar are retention problems, not exposure problems. A learner who reviews at the right moment remembers; one who reviews at the wrong moment wastes their time. This module is the single largest lever on learning outcomes in the product.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Review cards: one per (learner, learnable item)
- FSRS scheduling: stability, difficulty, retrievability, next due date
- Review logs: every grade, for scheduler improvement and analytics
- Due queue construction with daily limits and interleaving
- Suspend, bury and reset operations
- Retention forecasting and workload projection
- Per-learner parameter optimisation (later phase)
<!-- END GENERATED: readme-resp -->

## Where things are

<!-- BEGIN GENERATED: readme-folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: readme-folders -->

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**PLANNED** — planned for delivery phase 2. See [/ROADMAP.md](../../../ROADMAP.md).
