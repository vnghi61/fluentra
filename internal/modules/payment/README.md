---
module: payment
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [payments, invoices, payment_webhooks, refunds, checkout_sessions]
depends_on: [subscription, audit, job, mailer, storage]
depended_on_by: [subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# payment

Money: gateway adapters, hosted checkout sessions, webhook processing, invoices, refunds and reconciliation. Card data never touches our systems — the gateway's hosted fields do, which keeps PCI scope minimal.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Money: gateway adapters, hosted checkout sessions, webhook processing, invoices, refunds and reconciliation. Card data never touches our systems — the gateway's hosted fields do, which keeps PCI scope minimal.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Gateway adapters behind one interface
- Checkout session creation and redirect handling
- Webhook receipt, signature verification, idempotent processing and replay
- Payment and invoice records
- Refunds, partial and full
- Reconciliation between our records and the gateway's
- Dunning: retry schedule and communications on failed renewals
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
