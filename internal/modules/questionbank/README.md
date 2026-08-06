---
module: questionbank
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_options, question_sets, question_set_items, question_stats]
depends_on: [content, ai, audit, search]
depended_on_by: [exam, reading, listening, grammar, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank

The reusable item bank: authoring, typing, tagging, difficulty statistics, review workflow, and AI-assisted generation. One item, many uses — in a lesson, in a drill, in an exam.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
The reusable item bank: authoring, typing, tagging, difficulty statistics, review workflow, and AI-assisted generation. One item, many uses — in a lesson, in a drill, in an exam.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Question items across all supported types
- Options, correct answers and per-option feedback
- Question sets: reusable ordered groups
- Tagging by skill, level, topic and exam relevance
- Difficulty and discrimination statistics from real attempts
- Authoring and review workflow
- AI-assisted item generation for admin review
- Item exposure control so the same items are not overused
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

**PLANNED** — planned for delivery phase 4. See [/ROADMAP.md](../../../ROADMAP.md).
