---
module: gamification
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: learn
tables: [xp_events, streaks, badges, badges_earned, quests, user_quests, leaderboard_snapshots]
depends_on: [learning, srs, cache, job, notification]
depended_on_by: [notification, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# gamification

Motivation mechanics: XP, levels, streaks, badges, quests and leaderboards. It reacts to learning events and never drives them.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Retention in consumer learning products is decided in the first two weeks, and streaks plus visible progress are the strongest levers available. Used carelessly they produce anxiety and churn instead, so the rules below lean towards forgiveness.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- XP awards per learning action, with anti-farming rules
- Levels derived from cumulative XP
- Streaks with freezes and timezone-correct day boundaries
- Badges and their unlock conditions
- Quests: time-boxed multi-step goals
- Leaderboards (opt-in, league-based)
- Daily goal tracking
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

**PLANNED** — planned for delivery phase 3. See [/ROADMAP.md](../../../ROADMAP.md).
