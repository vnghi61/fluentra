---
module: writing
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [writing_tasks, writing_drafts, writing_submissions, writing_feedback, writing_revisions]
depends_on: [ai, job, content, learning, notification]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# writing

Writing tasks, drafts, submissions and AI rubric grading. The most expensive feature in the product per use, and the one learners value most.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Writing tasks, drafts, submissions and AI rubric grading. The most expensive feature in the product per use, and the one learners value most.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Writing tasks: prompt, type (IELTS Task 1/2, email, essay, free), word bounds, time limit, rubric
- Drafts with autosave and revision history
- Submission lifecycle and idempotency
- AI rubric grading orchestration and streamed feedback
- Sub-scores per rubric criterion plus overall band
- Inline annotations mapped to text ranges
- Similarity checking against the learner's own history and a reference corpus
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
