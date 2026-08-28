---
module: learning
tier: learning
group: modules
status: DONE
phase: 2
owner: "@learning-team"
schema: learn
tables: [enrollments, progress, attempts, learning_sessions, placement_results, skill_mastery]
depends_on: [lesson, content, srs, cache, job]
depended_on_by: [gamification, analytics, admin, exam, vocabulary, grammar, reading, listening, speaking, writing]
spec_version: 1.0.0
last_verified: 2026-08-25
---

# learning — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/learning` |
| Schema | `learn` |
| Delivery phase | 2 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
The learner's journey and the **exercise engine**. Owns enrolment, progress, attempts, session tracking, placement and the adaptive path — and defines the `ExerciseGrader` interface that every skill module implements.
<!-- END GENERATED: overview -->

**Context.** The exercise engine is the shared machinery: start an attempt, collect a response, dispatch to the right grader, record a score, update progress, emit events. Skill modules supply only the grader. This is the second half of ADR-0015 and the reason the six skill modules stay small.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Enrolment in courses
- Progress: per activity, lesson, unit, course and skill
- The exercise engine: attempt lifecycle, response collection, grader dispatch, scoring
- Learning sessions: start, resume, complete, time tracking
- Placement test orchestration and level assignment
- The adaptive daily plan: what to study next and how much
- Unlocking evaluation against the rules `lesson` defines
- Skill radar and mastery estimation

**This module does NOT own:**

- How a specific skill is graded — each skill module implements `ExerciseGrader`
- Review scheduling — that is `srs`
- XP, streaks and badges — that is `gamification`
- Curriculum structure — that is `lesson`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/learning/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/learning/contract/` | You are calling this module from another module |
| `internal/modules/learning/service/` | You are changing behaviour |
| `db/migrations/learning/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/learning/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `learning.ExerciseGrader` | `Grade(ctx, GradeRequest) (GradeResult, error)` — **implemented by every skill module**, registered by activity kind |
| struct | `learning.GradeResult` | `{Score, MaxScore, Correct, Feedback, Async, ReviewItems}` — `ReviewItem` is a `learning` type, not an `srs` one, so a grader in any skill module can return it |
| interface | `learning.ProgressReader` | `ProgressOf(ctx, userID, scope)` — used by `gamification`, `admin`, `analytics` |
| interface | `learning.UnlockChecker` | `IsUnlocked(ctx, userID, lessonIDs)` — used by `lesson` (batched to prevent N+1 queries) |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `activity.completed` | publishes | `{user_id, activity_id, score, skill, duration_ms}` |
| `lesson.completed` | publishes | `{user_id, lesson_id, score, skill_focus}` |
| `course.completed` | publishes | `{user_id, course_id}` |
| `placement.completed` | publishes | `{user_id, level, per_skill}` |
| `learning.session_completed` | publishes | `{user_id, minutes, activities}` |
| `writing.graded` | consumes | Complete the asynchronous attempt and roll up progress |
| `speaking.scored` | consumes | Same, for speaking |
| `user.deleted` | consumes | Anonymise progress and attempts |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `learn` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/learning/` · Queries: `db/queries/learning/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `learn.enrollments` | Learner ↔ course | `user_id`, `course_id`, `status`, `started_at`, `completed_at` |
| `learn.progress` | Rolled-up state | `user_id`, `scope` (activity/lesson/unit/course), `scope_id`, `status`, `score`, `completed_at`. Unique on (user, scope, scope_id) |
| `learn.attempts` | One try at an activity | Partitioned monthly. `user_id`, `activity_id`, `response` jsonb, `score`, `max_score`, `grader`, `duration_ms`, `status` |
| `learn.learning_sessions` | A study session | `user_id`, `started_at`, `ended_at`, `activities_completed`, `minutes` |
| `learn.placement_results` | Placement outcome | `user_id`, `estimated_level`, `per_skill` jsonb, `taken_at` |
| `learn.skill_mastery` | Per-skill mastery estimate | `user_id`, `skill`, `level`, `confidence`, `updated_at` |

**Indexes of note**

- `uq_progress_user_scope` — the read path for every dashboard
- `idx_attempts_user_activity_time` — attempt history
- `idx_attempts_activity_time` — item statistics for `questionbank`
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `learning`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/courses/{id}/enroll` | `self` | Enrol |
| `GET` | `/api/v1/me/dashboard` | `self` | Today's plan, due reviews, continue-where-you-left-off |
| `GET` | `/api/v1/me/progress` | `self` | Progress across courses and skills |
| `POST` | `/api/v1/activities/{id}/attempts` | `self` | Start an attempt |
| `POST` | `/api/v1/attempts/{id}/submit` | `self` | Submit a response for grading |
| `GET` | `/api/v1/attempts/{id}` | `self` | Attempt state and result |
| `POST` | `/api/v1/activities/{id}/grade` | `public` | Grade a response without recording anything |
| `POST` | `/api/v1/me/sessions` | `self` | Start a study session |
| `POST` | `/api/v1/me/sessions/{id}/complete` | `self` | End a session |
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
| [`lesson`](../../modules/lesson/AGENT.md) | → depends on | Structure, activities, unlocking rules |
| [`content`](../../modules/content/AGENT.md) | → depends on | Rendering activity content |
| [`srs`](../../modules/srs/AGENT.md) | → depends on | Push review items produced by a graded attempt |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | Dashboard and progress reads |
| [`job`](../../platform/job/AGENT.md) | → depends on | Asynchronous grading completion and rollups |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`vocabulary`](../../modules/vocabulary/AGENT.md) | ← used by | consumes this module's contract |
| [`grammar`](../../modules/grammar/AGENT.md) | ← used by | consumes this module's contract |
| [`reading`](../../modules/reading/AGENT.md) | ← used by | consumes this module's contract |
| [`listening`](../../modules/listening/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-LEARNING-01** — Scoring happens on the server. The client never sends a score, only a response.
2. **BR-LEARNING-02** — An attempt is immutable once graded. A retake creates a new attempt; the history is kept.
3. **BR-LEARNING-03** — A grader is selected by activity kind from a registry populated at startup; an unknown kind is a startup error.
4. **BR-LEARNING-04** — Asynchronous graders (writing, speaking) return `Async: true`; the attempt stays `grading` until the completing event arrives.
5. **BR-LEARNING-05** — Submitting requires an `Idempotency-Key`; a replay returns the original result rather than creating a second attempt.
6. **BR-LEARNING-06** — Progress rolls up on completion: activity → lesson → unit → course, in one transaction.
7. **BR-LEARNING-07** — A lesson is complete when every required activity is complete; optional activities do not block it.
8. **BR-LEARNING-08** — Attempts older than the activity's time limit are expired by a job and cannot be submitted.
9. **BR-LEARNING-09** — Skill mastery is an exponentially weighted estimate over recent attempts, not a raw average — recent performance must dominate.
10. **BR-LEARNING-10** — The placement test adapts: item difficulty follows the running estimate, and it stops when the confidence interval is narrow enough or the item budget is exhausted.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a skill grader

1. Implement `learning.ExerciseGrader` in the skill module.
2. Register it by activity kind in `cmd/api` and `cmd/worker`.
3. Decide synchronous or asynchronous — anything over ~2 seconds is asynchronous.
4. Return `ReviewItems` if the material should enter spaced repetition.
5. Add golden-file tests for the scoring, and a test that the registry contains the kind.

### Add a progress dimension

1. Extend the `scope` enum and the rollup function.
2. Backfill with a job, not a migration, if the table is large.
3. Add it to the dashboard composition and invalidate the cache key on the relevant event.
4. Verify the dashboard query count has not grown.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Mastery estimation is exponentially weighted, not item-response-theory based; it is good enough to drive a plan, not to certify a level.
- The adaptive path is rule-based rather than learned.
- Progress rollups are synchronous within the submit transaction; at high volume this becomes a candidate for asynchronous rollup.
- There is no offline attempt sync.
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
| `fluentra:{env}:learning:dashboard:{user_id}:v1` | 2 min | A graded submission, an enrolment, a completed session |
| `fluentra:{env}:learning:progress:{user_id}:v1` | 5 min | The same three |

Both keys are per learner, not per course: `GET /me/progress` answers for every enrolled
course in one response, so a key per course would have to be reassembled from N entries to
serve one request. Invalidation happens **after** the transaction commits and a failed delete
is logged rather than returned — a Redis outage must not fail a submit. One staleness window
survives that: `Cache.GetOrLoad` writes its loaded value back on a goroutine, so a read that
began before a concurrent submit can land ahead of the delete, and the TTL is what clears it.

The per-lesson unlock cache this table used to name is deferred; see `TODO.md`.

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `ALREADY_ENROLLED` | 409 | Duplicate enrolment |
| `LESSON_LOCKED` | 403 | Prerequisites not met |
| `ACTIVITY_ALREADY_COMPLETED` | 409 | Re-submission not allowed for this activity kind |
| `ATTEMPT_EXPIRED` | 409 | Time limit exceeded |
| `ATTEMPT_NOT_FOUND` | 404 | No such attempt |
| `FORBIDDEN` | 403 | The attempt belongs to another learner — an idempotency key is not an authorisation token |
| `ALREADY_GRADED` | 409 | The attempt has left `in_progress` and a fresh submission cannot claim it |
| `IDEMPOTENCY_CONFLICT` | 409 | A different `Idempotency-Key` was presented for an attempt already claimed — a conflict, not a retry |
| `INVALID_IDEMPOTENCY_KEY` | 422 | `Idempotency-Key` header missing or not a UUID |
| `UNSUPPORTED_ACTIVITY_KIND` | 422 | No grader is registered for the activity kind. The kinds a deployment declares are validated at boot, so reaching this means the data asked for a kind that was never declared — one request fails, not the process |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **85% service, 90% domain (the engine is the most reused code in the product)**

```bash
go test ./internal/modules/learning/...                    # unit
go test -tags=integration ./internal/modules/learning/...  # integration (testcontainers)
```

**Focus areas**

- Idempotent submission: a replayed key returns the original result
- Grader dispatch selects the right grader and fails loudly for an unknown kind
- Async attempt completes only on the corresponding event, and is idempotent on redelivery
- Progress rollup correctness including optional activities
- Unlock evaluation matches the prerequisite graph
- Expiry job does not expire an attempt that was submitted a moment earlier
- Placement adaptation terminates and produces a defensible level
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not accept a score from the client.
- Do not implement skill-specific grading here — write a grader in the skill module.
- Do not mutate a graded attempt.
- Do not let the dashboard grow into an N+1 across five modules — it is cached and its query count is asserted in a test.
- Do not couple to a skill module's internals; graders are registered through the interface.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
