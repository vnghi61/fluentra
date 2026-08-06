---
module: learning
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [enrollments, progress, attempts, learning_sessions, placement_results, skill_mastery]
depends_on: [lesson, content, srs, cache, job]
depended_on_by: [gamification, analytics, admin, exam, vocabulary, grammar, reading, listening, speaking, writing]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# learning

The learner's journey and the **exercise engine**. Owns enrolment, progress, attempts, session tracking, placement and the adaptive path — and defines the `ExerciseGrader` interface that every skill module implements.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Learning is a sequence of small, measurable interactions. Recording them uniformly is what makes progress visible, spaced repetition possible, and analytics meaningful.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Enrolment in courses
- Progress: per activity, lesson, unit, course and skill
- The exercise engine: attempt lifecycle, response collection, grader dispatch, scoring
- Learning sessions: start, resume, complete, time tracking
- Placement test orchestration and level assignment
- The adaptive daily plan: what to study next and how much
- Unlocking evaluation against the rules `lesson` defines
- Skill radar and mastery estimation
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

**PLANNED** — planned for delivery phase 2. See [/ROADMAP.md](../../../ROADMAP.md).
