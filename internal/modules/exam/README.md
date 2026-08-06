---
module: exam
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [exams, exam_sections, exam_attempts, attempt_answers, score_reports, integrity_events]
depends_on: [questionbank, job, ai, writing, speaking, learning]
depended_on_by: [learning, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# exam

Timed mock exams in real formats (IELTS, TOEIC): sections, strict timing, auto-submit, integrity signals, scoring and score reports.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Timed mock exams in real formats (IELTS, TOEIC): sections, strict timing, auto-submit, integrity signals, scoring and score reports.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Exam definitions: sections, item sources, timing, scoring model
- Attempt orchestration with server-authoritative timing
- Auto-submit on expiry, including when the client disappears
- Section navigation rules
- Scoring and band conversion per exam format
- Score reports with per-section breakdown and comparison
- Integrity signals: tab changes, paste events, timing anomalies
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
