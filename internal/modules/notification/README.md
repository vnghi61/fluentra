---
module: notification
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: comm
tables: [notifications, notification_preferences, devices, notification_dedupe]
depends_on: [mailer, job, cache, user]
depended_on_by: [auth, writing, speaking, exam, gamification, subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# notification

Turns domain events into messages a learner actually sees: in-app notifications, push notifications and emails. Owns delivery preferences, quiet hours, digesting, and deduplication.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Retention in a learning product depends on timely, welcome nudges — a due-review reminder, a graded essay, a streak about to break. The same mechanism becomes an annoyance if it is noisy, badly timed, or ignores preferences, so restraint is a first-class requirement.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Notification templates and rendering per channel and locale
- Delivery preferences per category and channel
- Quiet hours in the learner's own timezone
- Digesting: collapsing many events into one message
- Deduplication and rate limiting per user
- In-app inbox: list, mark read, unread count
- Push device registration and token lifecycle
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
