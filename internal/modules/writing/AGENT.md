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

# writing — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/writing` |
| Schema | `skill` |
| Delivery phase | 3 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Writing tasks, drafts, submissions and AI rubric grading. The most expensive feature in the product per use, and the one learners value most.
<!-- END GENERATED: overview -->

**Context.** Grading is asynchronous by necessity: a frontier model takes 10–30 seconds on a 250-word essay. The learner experience is therefore submit → 202 → streamed feedback, never a spinning request.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Writing tasks: prompt, type (IELTS Task 1/2, email, essay, free), word bounds, time limit, rubric
- Drafts with autosave and revision history
- Submission lifecycle and idempotency
- AI rubric grading orchestration and streamed feedback
- Sub-scores per rubric criterion plus overall band
- Inline annotations mapped to text ranges
- Similarity checking against the learner's own history and a reference corpus

**This module does NOT own:**

- Calling a model directly — it asks `platform/ai` by task name
- The rubric's pedagogy — that lives in `docs/knowledge/cefr.md` and the prompt
- Awarding XP — that is `gamification`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/writing/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/writing/contract/` | You are calling this module from another module |
| `internal/modules/writing/service/` | You are changing behaviour |
| `db/migrations/writing/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/writing/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `writing.Grader` | Implements `learning.ExerciseGrader`; always returns `Async: true` |
| interface | `writing.Reader` | `SubmissionHistory` — used by `analytics` and `admin` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `writing.submission_created` | publishes | `{user_id, submission_id, task_id, word_count}` |
| `writing.graded` | publishes | `{user_id, submission_id, overall_band, criteria}` |
| `writing.grading_failed` | publishes | `{submission_id, reason}` |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `skill` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/writing/` · Queries: `db/queries/writing/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `skill.writing_tasks` | Prompt definitions | Content-versioned. `type`, `prompt`, `min_words`, `max_words`, `time_limit_s`, `rubric_id` |
| `skill.writing_drafts` | Autosaved work in progress | `user_id`, `task_id`, `body`, `word_count`, `updated_at`; one active draft per task |
| `skill.writing_submissions` | Submitted work | `user_id`, `task_id`, `body`, `word_count`, `status`, `overall_band`, `submitted_at`. Immutable body. |
| `skill.writing_feedback` | Grading output | `submission_id`, `criterion`, `score`, `comment`, `annotations` jsonb, `prompt_version`, `provider` |
| `skill.writing_revisions` | Draft history | Snapshot every N minutes or M characters; retained 90 days |


<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `writing`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/writing/tasks` | `content.read.published` | Available tasks by level and type |
| `PUT` | `/api/v1/writing/tasks/{id}/draft` | `self` | Autosave a draft |
| `POST` | `/api/v1/writing/submissions` | `self` | Submit for grading |
| `GET` | `/api/v1/writing/submissions/{id}` | `self` | Submission with feedback when ready |
| `GET` | `/api/v1/writing/submissions/{id}/stream` | `self` | SSE stream of grading progress and partial feedback |
| `GET` | `/api/v1/writing/submissions` | `self` | History with band progression |
| `POST` | `/api/v1/writing/submissions/{id}/dispute` | `self` | Flag a grade for human review |
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
| [`ai`](../../platform/ai/AGENT.md) | → depends on | Rubric grading and quick hints |
| [`job`](../../platform/job/AGENT.md) | → depends on | Grading runs as a job |
| [`content`](../../modules/content/AGENT.md) | → depends on | Task definitions are content |
| [`learning`](../../modules/learning/AGENT.md) | → depends on | Attempt lifecycle and progress |
| [`notification`](../../modules/notification/AGENT.md) | → depends on | Tell the learner feedback is ready |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-WRITING-01** — Grading is always asynchronous. The HTTP request returns 202 with a stream URL and never waits.
2. **BR-WRITING-02** — Word count bounds are enforced server-side before any model call — rejecting a 12-word essay costs nothing and should not cost a grading credit.
3. **BR-WRITING-03** — A submission body is immutable. A revision is a new submission, so band progression over time is real.
4. **BR-WRITING-04** — Quota and budget are checked before the job is enqueued, so a learner is told immediately rather than after a wait.
5. **BR-WRITING-05** — Sub-scores are validated against the rubric's range and checked for consistency with the overall band; an inconsistent result fails the job rather than reaching the learner.
6. **BR-WRITING-06** — Annotations reference character ranges in the immutable submitted body, so highlighting cannot drift.
7. **BR-WRITING-07** — Learner text is passed to the model only inside the untrusted-content wrapper.
8. **BR-WRITING-08** — If the learner has opted out of AI processing, submission is refused with a clear explanation rather than silently degraded.
9. **BR-WRITING-09** — A disputed grade is queued for admin review; the original feedback is retained alongside any correction.
10. **BR-WRITING-10** — Grading failure never counts against the learner's quota — the quota is decremented on success.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a writing task type

1. Add the type and its rubric to `docs/knowledge/cefr.md` and the rubric registry.
2. Create the task content kind and its schema.
3. Extend the runtime prompt input schema if the rubric shape differs — this means a new prompt version.
4. Add golden examples across CEFR levels to the eval suite and re-run it.
5. Add the editor variant in the web app.
6. Verify the band-consistency check still holds for the new rubric.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Grading is single-pass; there is no second-opinion pass or model ensemble in Phase 3.
- Similarity checking covers the learner's own history and a small reference corpus only — it is not plagiarism detection against the web.
- Handwriting or image submission is not supported.
- Annotation ranges assume plain text; rich formatting is stripped before grading.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:writing:task:{task_id}:v1` | 24 h | Task republished |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `SUBMISSION_TOO_SHORT` | 422 | Below the task's minimum word count |
| `SUBMISSION_TOO_LONG` | 422 | Above the maximum |
| `SUBMISSION_ALREADY_GRADED` | 409 | Immutable once graded |
| `GRADING_FAILED` | 500 | All grading attempts failed |
| `DISPUTE_ALREADY_OPEN` | 409 | One open dispute per submission |

### Security considerations

- Essays are sensitive personal content: only the author and an admin with an audited reason may read one.
- The prompt-injection defence is layered — wrapper, hardened preamble, schema validation, band clamping, and a red-team eval suite that must score 1.0.
- Similarity checking never exposes another learner's text; it reports a score, not a source.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/writing/...                    # unit
go test -tags=integration ./internal/modules/writing/...  # integration (testcontainers)
```

**Focus areas**

- Word bounds rejected before any model call
- Idempotent submission
- Async attempt completes only on `writing.graded`, idempotently
- Band clamping and cross-criterion consistency
- Prompt-injection fixtures do not move the band
- Failure does not consume quota
- SSE reconnection resumes from `Last-Event-ID`
- Disputes preserve the original feedback
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not grade synchronously.
- Do not call a provider SDK — use `platform/ai` by task name.
- Do not trust the model's band without clamping and a consistency check.
- Do not charge quota for a failed grading.
- Do not mutate a submitted body.
- Do not put learner text into the instruction block of a prompt.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
