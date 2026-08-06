---
module: cache
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry]
depended_on_by: [auth, rbac, user, content, lesson, learning, srs, gamification, ai, notification, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# cache

A typed facade over Redis: key building, serialisation, TTL policy, single-flight, distributed locks, and rate limiting. Modules never touch the Redis client directly, so key conventions and degradation behaviour are enforced in one place.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
A typed facade over Redis: key building, serialisation, TTL policy, single-flight, distributed locks, and rate limiting. Modules never touch the Redis client directly, so key conventions and degradation behaviour are enforced in one place.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Typed get/set/delete with a generic `Cache[T]`
- Key construction following the repository convention, including schema versioning
- Cache-aside and write-through helpers
- Single-flight to prevent stampedes
- Jittered TTLs
- Distributed locks (`SET NX PX`) with safe release
- Rate limiting (GCRA via `redis_rate`)
- Graceful degradation when Redis is unavailable
- Hit-ratio and latency metrics per module
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

**PLANNED** — planned for delivery phase 1. See [/ROADMAP.md](../../../ROADMAP.md).
