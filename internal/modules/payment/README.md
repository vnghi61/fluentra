---
module: payment
tier: commerce
group: modules
status: ACTIVE
phase: 3
owner: "@backend-team"
schema: billing
tables: [orders, sepay_transactions, payment_webhooks, refunds, payouts]
depends_on: [audit, job]
depended_on_by: [studio, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# payment

Money: gateway adapters, hosted checkout sessions, webhook processing, invoices, refunds and reconciliation. Card data never touches our systems — the gateway's hosted fields do, which keeps PCI scope minimal.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Money: SePay bank transfer integration, orders, webhook receipt and matching, VietQR generation, refunds and daily reconciliation. No card data exists — payment is a bank transfer the payer initiates via VietQR or banking app.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Bank transfer order creation and VietQR image generation
- SePay webhook receipt with API key authentication and fast acknowledgement
- Idempotent transaction matching on alphanumeric transfer content
- Reconciliation between SePay transactions and our billing database
- Order expiry sweep
- Admin unmatched queue for manual resolution
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
