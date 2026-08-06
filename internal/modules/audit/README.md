---
module: audit
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: audit
tables: [audit_logs, security_events]
depends_on: [job]
depended_on_by: [auth, user, rbac, admin, content, questionbank, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# audit

Records what happened, who did it, and what changed — immutably. Two streams: an administrative audit trail (every state change made by an admin, and every significant change made by a user to their own data) and a security event stream (authentication anomalies, permission denials, suspicious patterns).

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
When a learner says "my essay disappeared" or a regulator asks "who accessed this account", the answer must exist and must be trustworthy. Audit is also the raw material for the security dashboard.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Persisting audit entries with actor, action, target, before/after diff, IP, user agent, trace ID
- Persisting security events with a severity
- Enforcing append-only semantics
- Retention and partition management
- Query API for the admin UI, with filters and export
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
