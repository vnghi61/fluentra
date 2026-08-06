---
module: analytics
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@data-team"
schema: analytics
tables: [analytics_events, daily_rollups, cohorts, funnel_steps, learner_outcomes]
depends_on: [job, cache]
depended_on_by: [admin, notification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# analytics

Turns the event stream into answers: event ingestion, daily rollups, funnels, cohorts, retention curves and the admin KPI dashboards. It reads events; it never writes to another module's data.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Product decisions in a learning platform are only as good as the retention and outcome data behind them. This module exists so that "does this feature help people learn?" is answerable with numbers rather than opinion.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Event ingestion from the outbox into an analytics store
- Daily and weekly rollups by learner, cohort, skill and level
- Funnels: signup → placement → first lesson → day-7 return → subscription
- Retention curves and cohort analysis
- Learning outcome metrics: level progression, review accuracy, band improvement
- AI cost per active learner
- Admin KPI dashboards and scheduled reports
- The weekly learner progress email payload
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
