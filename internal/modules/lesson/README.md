---
module: lesson
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [courses, course_units, lessons, activities, lesson_prerequisites]
depends_on: [content, cache]
depended_on_by: [learning, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# lesson

Structure: courses contain units, units contain lessons, lessons contain activities, and activities point at content versions. Owns prerequisites, ordering and unlocking rules.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
A learner needs a path, not a pile of exercises. This module is what turns a content library into a curriculum with a beginning, an order, and a sense of progress.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Course, unit, lesson and activity structure and ordering
- Activity → content version binding
- Prerequisites and unlocking rules
- Estimated duration and skill/level metadata
- Course catalogue and lesson detail queries
- Draft/published state for structure, mirroring content
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

**PLANNED** — planned for delivery phase 2. See [/ROADMAP.md](../../../ROADMAP.md).
