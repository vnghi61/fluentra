---
module: grammar
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [grammar_points, grammar_rules, grammar_exercises, error_tags, user_grammar_state]
depends_on: [content, srs, ai, learning]
depended_on_by: [writing, speaking, learning, exam]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# grammar

Grammar as a structured, teachable system: a rule taxonomy, error tagging, targeted drills, and AI explanations that always cite a rule from our own taxonomy rather than improvising.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Grammar as a structured, teachable system: a rule taxonomy, error tagging, targeted drills, and AI explanations that always cite a rule from our own taxonomy rather than improvising.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Grammar point taxonomy with CEFR levels and prerequisites
- Rule definitions with canonical examples and common errors
- Error tagging of learner output, reusable across writing and speaking
- Drill types: gap-fill, transformation, error correction, sentence ordering
- AI explanation grounded in a cited rule
- Per-learner grammar weakness profile
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

**PLANNED** — planned for delivery phase 3. See [/ROADMAP.md](../../../ROADMAP.md).
