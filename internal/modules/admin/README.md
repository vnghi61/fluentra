---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [admin_actions, feature_flags, admin_notes, moderation_items]
depends_on: [user, rbac, audit, auth, content, cache]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-08-06
---

# admin

The back-office surface. It owns almost no data of its own; it composes other modules' contracts into the screens an administrator needs: dashboards, user management, content moderation, feature flags, and operational tooling.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Running the platform must not require database access. Every routine operational action — suspend a user, republish a lesson, re-run a failed grading job, flip a feature flag — has a screen, and every one of them is audited.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Admin dashboard aggregation across modules
- User management screens (list, search, inspect, suspend, reinstate, impersonate)
- Moderation queues (flagged content, reported names, flagged submissions)
- Feature flag management
- Operational actions: retry a job, replay a webhook, invalidate a cache key
- System health summary for non-engineers
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
