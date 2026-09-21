---
module: questionbank
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_stats]
depends_on: [content, lesson, learning, rbac]
depended_on_by: [exam]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank

The reusable item bank: authoring, typing, tagging, difficulty statistics, review workflow, and AI-assisted generation. One item, many uses — in a lesson, in a drill, in an exam.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
The exam item bank: generated questions tagged to the spine and to an exam part, each a content version drawn as an activity, with provenance, a fingerprint and empirical statistics.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Bank metadata for exam questions: kind, skill, CEFR level, exam part, questions per group, provenance
- Generating draft questions for a part and spine nodes through `learning.Generator`
- A normalised fingerprint per question, unique in the database
- Moving an approved question into the bank course (`pool-bank`) once its content is published
- Answering which published questions an exam can draw for a part
- Holding `question_stats` for empirical difficulty
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

**PLANNED** — planned for delivery phase 4. See [/ROADMAP.md](../../../ROADMAP.md).
