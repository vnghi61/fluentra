---
module: telemetry
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: []
depended_on_by: [ai, cache, storage, job, media, search, mailer]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# telemetry

OpenTelemetry setup and the middleware that makes every request, query, job and external call observable: tracer, meter and logger providers, resource attributes, propagation, correlation IDs, and the standard metric instruments.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
OpenTelemetry setup and the middleware that makes every request, query, job and external call observable: tracer, meter and logger providers, resource attributes, propagation, correlation IDs, and the standard metric instruments.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Tracer, meter and logger provider construction and shutdown
- OTLP exporter configuration and resource attributes
- HTTP middleware: trace context extraction, request ID, span naming, panic recovery
- slog handler with the OTLP bridge and the PII redaction allowlist
- Standard instruments (HTTP, DB, cache, storage, jobs) so modules do not each invent their own
- Sampling configuration
- Health and readiness endpoints
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
