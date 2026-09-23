---
module: exam
tier: learning
group: modules
status: DONE
phase: 3
owner: "@learning-team"
schema: assess
tables: [exams, exam_sections, exam_attempts, score_reports, integrity_events, exam_versions, exam_parts, blueprints, mock_tests]
depends_on: [questionbank, job, ai, writing, speaking, learning, lesson, listening]
depended_on_by: [learning, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# exam — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/exam` |
| Schema | `assess` |
| Delivery phase | 4 |
| Status | **DONE** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Timed mock exams in real formats (IELTS, TOEIC): sections, strict timing, auto-submit, integrity signals, scoring and score reports.
<!-- END GENERATED: overview -->

**Context.** An exam is not a lesson with a timer. It has section-level time limits, no going back, one listening play, no dictionary, and a hard stop. Learners take mock exams to rehearse those constraints, so simulating them faithfully is the feature.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Exam definitions: sections, item sources, timing, scoring model
- Attempt orchestration with server-authoritative timing
- Auto-submit on expiry, including when the client disappears
- Section navigation rules
- Scoring and band conversion per exam format
- Score reports with per-section breakdown and comparison
- Integrity signals: tab changes, paste events, timing anomalies

**This module does NOT own:**

- Authoring items — that is `questionbank`
- Grading writing and speaking sections — those delegate to their skill modules
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/exam/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/exam/contract/` | You are calling this module from another module |
| `internal/modules/exam/service/` | You are changing behaviour |
| `db/migrations/exam/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/exam/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `exam.Reader` | `AttemptHistory`, `LatestBand` — used by `learning` and `analytics` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `exam.attempt_started` | publishes | `{user_id, exam_id, attempt_id}` |
| `exam.attempt_finished` | publishes | `{user_id, attempt_id, band, per_section}` |
| `writing.graded` | consumes | Complete the writing section score |
| `speaking.scored` | consumes | Complete the speaking section score |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `assess` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/exam/` · Queries: `db/queries/exam/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `assess.exams` | Exam definition | Content-versioned. `format`, `total_minutes`, `scoring_model` |
| `assess.exam_sections` | Timed part | `exam_id`, `position`, `skill`, `minutes`, `question_set_id`, `navigation` (linear/free) |
| `assess.exam_attempts` | One sitting | `user_id`, `exam_id`, `mode`, `chosen_duration_minutes`, `started_at`, `deadline_at`, `status`, `section_activities`, `draft_answers`, `submitted_at`, `submitted_by` |
| `assess.score_reports` | Learner-facing result | `attempt_id`, `overall_band`, `per_section` jsonb, `feedback`, `percentile` |
| `assess.integrity_events` | Signals during an attempt | `attempt_id`, `kind`, `occurred_at` — informational, never punitive automatically |
| `assess.exam_versions` | Verified exam versions | `exam_family`, `code`, `title`, `total_minutes`, `scoring`, `source_url`, `verified_at`, `is_current` |
| `assess.exam_parts` | Exam parts per version | `version_id`, `section`, `part_number`, `kind`, `question_count`, `group_size` |
| `assess.blueprints` | Mock test blueprints | `version_id`, `name`, `cefr_distribution`, `node_distribution` |
| `assess.mock_tests` | Composed mock tests | `blueprint_id`, `mode`, `number` (fixed only), `seed`, `composition`, `owner_id` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `exam`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/exams` | `content.read.published` | Available mock exams |
| `POST` | `/api/v1/exams/{id}/attempts` | `self` | Start a sitting |
| `GET` | `/api/v1/exam-attempts` | `self` | List past sittings for current user |
| `GET` | `/api/v1/exam-attempts/{id}` | `self` | Current state with server time remaining |
| `PUT` | `/api/v1/exam-attempts/{id}/answers` | `self` | Save answers (autosave) |
| `POST` | `/api/v1/exam-attempts/{id}/sections/{n}/complete` | `self` | Finish a section |
| `POST` | `/api/v1/exam-attempts/{id}/submit` | `self` | Submit the whole exam |
| `GET` | `/api/v1/exam-attempts/{id}/report` | `self` | Score report when ready |
| `GET` | `/api/v1/exam-versions` | `content.read.published` | Current verified exam versions and their blueprints |
| `POST` | `/api/v1/mock-tests` | `self` | Compose a mock test |
| `POST` | `/api/v1/mock-tests/{id}/attempts` | `self` | Start or retake a mock test attempt |
| `GET` | `/api/v1/admin/exams/versions/{id}/coverage` | `questionbank.read` | Exam version coverage report |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`questionbank`](../../modules/questionbank/AGENT.md) | → depends on | see its contract |
| [`job`](../../platform/job/AGENT.md) | → depends on | see its contract |
| [`ai`](../../platform/ai/AGENT.md) | → depends on | see its contract |
| [`writing`](../../modules/writing/AGENT.md) | → depends on | see its contract |
| [`speaking`](../../modules/speaking/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | → depends on | see its contract |
| [`lesson`](../../modules/lesson/AGENT.md) | → depends on | see its contract |
| [`listening`](../../modules/listening/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-EXAM-01** — Time is server-authoritative. The client displays a countdown; the server decides when it is over.
2. **BR-EXAM-02** — An expired attempt is auto-submitted by a job — a learner who closes the tab still gets their answers scored.
3. **BR-EXAM-03** — Section navigation follows the exam format: linear sections cannot be revisited, free sections can.
4. **BR-EXAM-04** — One attempt in progress per learner per exam.
5. **BR-EXAM-05** — Answers autosave continuously; a lost connection must not lose work.
6. **BR-EXAM-06** — Sections graded by AI (writing, speaking) mark the report incomplete until their events arrive.
7. **BR-EXAM-07** — Band conversion uses the published table for the format, versioned so historical reports remain reproducible.
8. **BR-EXAM-08** — Integrity signals are recorded and shown to the learner as self-awareness information; they never automatically invalidate an attempt.
9. **BR-EXAM-09** — Items are sampled with exposure control so a learner does not see the same items in consecutive attempts.
10. **BR-EXAM-10** — The score report is generated once and stored — it must not change if the scoring table is later revised.
11. **BR-EXAM-11** — Exam structures are seeded data with an `https` source and a verification date.
12. **BR-EXAM-12** — A retake replays its composition; only a new test draws again.
13. **BR-EXAM-13** — A fixed test is a numbered, stored composition shared by everyone; the fixed tests of one blueprint share no question.
14. **BR-EXAM-14** — A part is filled with exactly its question count, in whole groups; a part the bank cannot fill is refused with the part named.
15. **BR-EXAM-15** — The number of distinct tests shown is the coverage report's number.
16. **BR-EXAM-16** — A mock test's composition is stored on the attempt and served from there, so its activity configs are redacted at composition time: a key that reaches the composition reaches the learner.
17. **BR-EXAM-17** — A practice sitting may opt out of a visible time limit; the server still applies a generous backstop deadline so an abandoned sitting cannot block a future attempt indefinitely.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add an exam format

1. Define sections, timings and navigation rules.
2. Add the band conversion table, versioned.
3. Ensure `questionbank` has enough approved items per section, or the exam cannot be built.
4. Add the score report template.
5. Test the full sitting including auto-submit and a failed AI section.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- No proctoring; integrity signals are advisory.
- Speaking sections require the learner to have granted microphone access before starting.
- Percentile comparison needs a population; it is suppressed until enough attempts exist.
- No offline exam mode.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `ATTEMPT_IN_PROGRESS` | 409 | Finish or abandon the existing attempt first |
| `EXAM_ALREADY_SUBMITTED` | 409 | Terminal state |
| `EXAM_WINDOW_CLOSED` | 403 | Outside the availability window |
| `SECTION_TIME_EXPIRED` | 409 | Section time is over |
| `INSUFFICIENT_ITEMS` | 409 | Not enough approved items to build the exam |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/exam/...                    # unit
go test -tags=integration ./internal/modules/exam/...  # integration (testcontainers)
```

**Focus areas**

- Server-authoritative timing cannot be extended by the client
- Auto-submit fires for an abandoned attempt
- Section navigation rules enforced
- Autosave preserves answers across a disconnect
- Band conversion reproducible from the versioned table
- Partial report when an AI section fails
- Exposure control across consecutive attempts
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not trust client-reported time.
- Do not let an abandoned attempt sit unsubmitted.
- Do not recompute a stored score report.
- Do not invalidate an attempt automatically from integrity signals.
- Do not duplicate question storage — sample from `questionbank`.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
