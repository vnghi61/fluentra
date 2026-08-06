---
module: search
tier: platform
group: platform
status: PLANNED
phase: 4
owner: "@platform-team"
schema: none
tables: []
depends_on: [cache, job]
depended_on_by: [content, vocabulary, questionbank, lesson, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# search

A thin abstraction over full-text search. Backed by PostgreSQL `tsvector` in v1, with an interface shaped so a dedicated engine can replace it without touching callers.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
A thin abstraction over full-text search. Backed by PostgreSQL `tsvector` in v1, with an interface shaped so a dedicated engine can replace it without touching callers.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Index definition and maintenance per searchable entity
- Query building: tokenisation, prefix matching, ranking, highlighting
- Reindex jobs, full and incremental
- Language configuration for English content
- Search latency and result-quality metrics
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

**PLANNED** — planned for delivery phase 4. See [/ROADMAP.md](../../../ROADMAP.md).
