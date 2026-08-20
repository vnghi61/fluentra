---
doc_type: work_package
phase: 2
work_package: WP8
title: "learning — the exercise engine"
tasks: 5
estimate: "~12 days"
blocked_by: WP7
status: ready
last_verified: 2026-08-20
---

# WP8 — `learning`: the exercise engine

**This is the highest-leverage work in the project. Slow down here.**

ROADMAP.md, on Phase 2:

> The shared `content` + exercise engine (ADR-0015) is the highest-leverage work in the
> project — if it is done well, phase 3 is six thin modules; if it is done badly, phase 3 is
> six copies of phase 2.

`learning.ExerciseGrader` is implemented once in WP9 and then five more times in Phase 3 by
`grammar`, `reading`, `listening`, `speaking` and `writing`. Every mistake in its shape is
paid for six times, after the point where changing it is cheap.

| Task | Branch |
|---|---|
| P8.1 | `feat/learning-contracts` |
| P8.2 | `feat/learning-schema` |
| P8.3 | `feat/learning-exercise-engine` |
| P8.4 | `feat/learning-progress` |
| P8.5 | `feat/learning-dashboard-endpoints` |

**Required reading:** `internal/modules/learning/AGENT.md` (all of it),
`docs/adr/ADR-0015-content-exercise-core.md`, `docs/adr/ADR-0009-event-bus-in-process.md`,
`/ERROR_HANDLING.md`.

---

## P8.1 — Contracts and OpenAPI, no implementation `M`

| | |
|---|---|
| **Depends on** | P7.3 |
| **Context** | `learning/AGENT.md` §4, the plan §2.2 |
| **Files** | `internal/modules/learning/contract/`, `api/openapi/openapi.yaml` |
| **Do** | Write the four contract types exactly as `AGENT.md` §4 names them: `ExerciseGrader` with `Grade(ctx, GradeRequest) (GradeResult, error)`; `GradeResult` as `{Score, MaxScore, Correct, Feedback, Async, ReviewItems}`; `ProgressReader.ProgressOf(ctx, userID, scope)`; `UnlockChecker.IsUnlocked(ctx, userID, lessonID)`. Then the OpenAPI paths: `POST /courses/{id}/enroll`, `GET /me/dashboard`, `GET /me/progress`, `POST /activities/{id}/attempts`, `POST /attempts/{id}/submit`, `GET /attempts/{id}`, `POST /me/sessions`, `POST /me/sessions/{id}/complete`. |
| **Acceptance** | `make gen` and `pnpm gen:api` both clean, no handlers yet. A frontend agent can build the dashboard and the lesson runner against generated types from this commit. |
| **Trap — two, and both are expensive** | **(1) `GET /me/dashboard` is documented in `AGENT.md` as returning a `streak`. `gamification` is a Phase 3 module and owns `streaks`. Ship the response without it.** Do not add a `streaks` table to `learn` to fill the gap — that is another module's table in your schema and fails rule DB1. **(2) The idempotency key for `POST /attempts/{id}/submit` is part of the HTTP contract, not a backend detail.** ADR-0015 requires idempotent submission; if the key is invisible to the client, a learner with two tabs open silently creates two attempts. Put it in the spec now (header or body field — decide and record in `learning/DECISIONS.md`), because adding it later means changing a contract the frontend has already built against. |

## P8.2 — `learning` schema `M`

| | |
|---|---|
| **Depends on** | P8.1 |
| **Context** | `learning/AGENT.md` §5, `/DATABASE_GUIDELINE.md` |
| **Files** | `db/migrations/learning/`, `db/queries/learning/` |
| **Do** | Schema `learn`: `enrollments`, `progress`, `attempts`, `learning_sessions`, `placement_results`, `skill_mastery`. `progress` is unique on `(user_id, scope, scope_id)` where scope is activity / lesson / unit / course — that index (`uq_progress_user_scope`) is the read path for every dashboard request in the product, so get it right now. `attempts` is **partitioned monthly** as the module doc specifies; create the partition-management job alongside it, not later. `placement_results` is created empty — the placement flow is Phase 4, but the table belongs to this module's schema and creating it once is cheaper than a second migration. |
| **Acceptance** | Reversible. Every FK indexed. Inserting an attempt for next month lands in the right partition, proven by a test. `EXPLAIN` on the dashboard progress query uses `uq_progress_user_scope`. |
| **Trap** | Monthly partitioning with no automatic partition creation is a production outage on the 1st of a month. The job that creates the next partition ships in this task or the task is not done. |

## P8.3 — `ExerciseGrader`, the registry, and the attempt lifecycle `L`

| | |
|---|---|
| **Depends on** | P8.2 |
| **Context** | ADR-0015, `learning/AGENT.md` §1–4 |
| **Files** | `internal/modules/learning/{domain,service,repository,transport/http,job}/`, `module.go` |
| **Do** | The grader registry maps an activity kind to an `ExerciseGrader`, and is **validated at startup** — ADR-0015 says so explicitly. An activity kind with no registered grader must fail the process at boot, with the kind named in the error, not fail the learner's request at 22:00. Then the lifecycle: start → submit (idempotent) → dispatch to grader → score → persist attempt → roll up progress → publish `activity.completed`, and `lesson.completed` / `course.completed` when the rollup crosses those boundaries. `GradeResult.ReviewItems` is what feeds `srs` in WP9 — it is emitted here even though nothing consumes it yet. `GradeResult.Async` exists for Phase 3's AI graders; honour the flag now (attempt stays `pending`, completed later by an event) even though every Phase 2 grader is synchronous. |
| **Acceptance** | Submitting the same attempt twice with the same idempotency key grades **once** and returns the same result — an integration test with two concurrent goroutines, not a sequential one. A grader registered for an unknown kind fails `cmd/api` at startup. Progress rolls up activity → lesson → unit → course in one transaction. Coverage ≥ 85 %. |
| **Tests** | Concurrency test on double-submit. Table-driven rollup. A fake grader in `domain/` so the lifecycle is testable with no skill module present. |
| **Trap** | The temptation is to make `GradeRequest` carry the whole content version so graders are convenient. That couples every future grader to the content shape and makes the interface impossible to widen. Pass ids and let the grader read what it needs through `content.Reader`. **Getting this wrong is the one thing in Phase 2 that Phase 3 cannot route around.** |

## P8.4 — Enrolment, progress rollup, and "what do I do next" `M`

| | |
|---|---|
| **Depends on** | P8.3 |
| **Context** | `learning/AGENT.md` §4–5, the UI plan §5 (Continue Learning is the dashboard's hero) |
| **Files** | `internal/modules/learning/service/` |
| **Do** | Enrolment. `ProgressReader.ProgressOf` for `gamification`, `admin` and `analytics` to use in Phase 3. `UnlockChecker.IsUnlocked` for `lesson`. Skill mastery updated from attempt outcomes into `skill_mastery` — a level and a confidence, updated incrementally, not recomputed from all history on every attempt. And **next-activity resolution**: given a learner, return the one activity they should do now. That single value is the dashboard's primary action and the answer to the UI plan's question 1. |
| **Acceptance** | A learner with no enrolment, a learner mid-lesson, and a learner who has finished everything each produce a sensible next-activity answer — three tests, because those are the three states the dashboard must render and two of them are the *common* case during the alpha. Learning sessions record minutes and activity counts. |
| **Trap** | Next-activity for a brand-new learner has no answer. Return an explicit "nothing started" state the UI can render, not `null` with no explanation and not a 404. |

## P8.5 — Dashboard and progress endpoints `M`

| | |
|---|---|
| **Depends on** | P8.4 |
| **Context** | The review §A1 — three cards, no streak, no XP |
| **Files** | `internal/modules/learning/transport/http/` |
| **Do** | Implement the paths specified in P8.1. `GET /me/dashboard` returns exactly what Phase 2 owns: next activity (or the "nothing started" state), due-review count (read through the `srs` contract once WP9 lands — until then the field is present and zero), and per-skill mastery. `GET /me/progress` returns progress across courses and skills. |
| **Acceptance** | The spec is unchanged by this commit. One dashboard request issues a bounded number of queries — assert the count; this endpoint is hit on every app open and an N+1 here is the first performance bug of the alpha. Every response state (new learner, mid-course, all done) has a test. |
| **Trap** | Do not add `streak`, `xp`, `daily_goal` or `achievements`, however much the UI mock wants them. They are Phase 3. A field that returns a plausible zero is worse than an absent field: the frontend builds a card around it and the card ships lying. |

---

## Work-package gate

- An attempt can be started, submitted twice concurrently with the same idempotency key,
  and graded exactly once
- A grader registered for an unknown activity kind fails **at startup**
- Score rolls up activity → lesson → unit → course in one transaction
- `activity.completed` and `lesson.completed` are published and observable
- The dashboard response contains no Phase 3 field
- Coverage ≥ 85 % on `learning`; `make check` green
