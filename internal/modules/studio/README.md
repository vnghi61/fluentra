---
module: studio
tier: commerce
group: modules
status: ACTIVE
phase: 3
owner: "@commerce-team"
schema: studio
tables: [creator_profiles, payout_accounts, course_drafts, submissions, listings, purchases, creator_ledger, takedowns]
depends_on: [content, lesson, learning, payment, job]
depended_on_by: [admin, lesson, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# studio

Creator Studio: authoring community courses, draft editing, submissions, automated Gate 1 checks, Gate 2 human moderation, and course publishing.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Creator Studio: authoring community courses, draft editing, submissions, automated Gate 1 checks, Gate 2 human moderation, and course publishing.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Creator profile and payout account registration
- Course draft CRUD and structure validation
- Gate 1 automated verification (structure, CEFR, safety, runner kinds)
- Gate 2 human moderation queue and approval/rejection decisions
- Course publishing to catalogue with content versioning
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

**ACTIVE** — planned for delivery phase 3. See [/ROADMAP.md](../../../ROADMAP.md).
