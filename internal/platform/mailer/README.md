---
module: mailer
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: comm
tables: [email_log, email_suppressions]
depends_on: [job, telemetry, storage]
depended_on_by: [auth, user, notification, subscription, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# mailer

Email transport: template rendering, localisation, sending through SMTP or a provider API, bounce and complaint handling, suppression lists, and a delivery log.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Email transport: template rendering, localisation, sending through SMTP or a provider API, bounce and complaint handling, suppression lists, and a delivery log.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- MJML/HTML template rendering with a plain-text alternative
- Localisation of subject and body
- Sending via SMTP or a provider API, behind one interface
- Retry with backoff on transient failures
- Bounce and complaint handling; suppression list
- Delivery log for support and debugging
- A development mailbox (Mailpit) so no real email leaves a developer machine
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
