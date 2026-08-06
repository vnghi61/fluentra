---
module: job
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: ops
tables: [river_job, outbox_events, job_failures]
depends_on: [telemetry]
depended_on_by: [auth, user, audit, notification, mailer, content, writing, speaking, media, ai, srs, analytics, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# job

Background execution: the River queue wiring, the cron scheduler, job middleware (tracing, logging, panic recovery, timeouts), the dead-letter path, and the operational surface for inspecting and retrying work.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Background execution: the River queue wiring, the cron scheduler, job middleware (tracing, logging, panic recovery, timeouts), the dead-letter path, and the operational surface for inspecting and retrying work.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- River client and worker wiring, queue registry, concurrency configuration
- Cron scheduling with distributed locking
- Job middleware: span, structured log, panic recovery, timeout, metrics
- Retry policy and dead-letter handling
- The outbox publisher loop
- Queue depth and age metrics
- Admin operations: inspect, retry, cancel
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

**PLANNED** — planned for delivery phase 1. See [/ROADMAP.md](../../../ROADMAP.md).
