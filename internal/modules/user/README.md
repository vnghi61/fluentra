---
module: user
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [users, profiles, user_preferences, learning_profiles, user_deletion_requests, user_exports]
depends_on: [storage, mailer, audit]
depended_on_by: [auth, admin, learning, notification, subscription, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# user

Owns the user record and everything descriptive about a person: profile, preferences, locale, timezone, avatar, learning goals, and the account lifecycle including export and deletion. It is the canonical source of `core.users`, which every other module references by ID.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
A learner's experience is personalised by locale, level and goals. A user must be able to see, correct, export and delete everything we hold about them — both because it is right and because privacy regulation requires it.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- The `core.users` record: identity, status, timestamps
- Profile: display name, avatar, country, timezone, locale, date of birth (for the age gate)
- Preferences: notification settings, daily goal, UI theme, AI processing opt-out
- Learning profile: self-declared level, goals, target exam, study reminders
- Account status transitions: active, suspended, pending deletion
- Data export (async) and erasure (30-day grace, then anonymisation)
- Avatar upload coordination through `platform/storage`
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
