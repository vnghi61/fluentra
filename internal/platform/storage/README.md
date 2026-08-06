---
module: storage
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry, job]
depended_on_by: [user, content, media, speaking, writing, analytics, audit]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# storage

A facade over MinIO (S3 API): bucket policy, presigned upload and download URLs, object verification, lifecycle rules, and orphan collection. Binary data never flows through the Go API — this module is how that rule is kept.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
A facade over MinIO (S3 API): bucket policy, presigned upload and download URLs, object verification, lifecycle rules, and orphan collection. Binary data never flows through the Go API — this module is how that rule is kept.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Bucket definitions, policies and lifecycle rules
- Presigned PUT (upload) and GET (download) with pinned content type, size and expiry
- Post-upload verification: existence, size, content type
- Object key conventions
- Copy, move and delete operations
- Orphan garbage collection
- Storage metrics per bucket
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
