---
module: subscription
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [plans, entitlements, subscriptions, subscription_events]
depends_on: [payment, user, notification, cache, job, audit]
depended_on_by: [writing, speaking, vocabulary, exam, ai, admin, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# subscription

Plans, entitlements and the subscription lifecycle: trials, upgrades, downgrades, renewals, grace periods and cancellation. It decides what a learner is allowed to do; `payment` decides whether money moved.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Plans, entitlements and the subscription lifecycle: trials, upgrades, downgrades, renewals, grace periods and cancellation. It decides what a learner is allowed to do; `payment` decides whether money moved.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Plan catalogue with prices and entitlements
- Entitlement resolution: what this learner may do right now
- Subscription lifecycle and state transitions
- Trials, upgrades, downgrades and proration rules
- Renewal scheduling and grace periods on failed payment
- Cancellation and reactivation
- Feature gating helpers used by other modules
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
