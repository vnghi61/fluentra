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

# learning — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| One engine or per-skill flows? | One engine, pluggable graders | Attempt lifecycle, idempotency, progress rollup and event emission are identical across skills; only scoring differs. Six copies would drift and each would need its own idempotency bug fixed separately |
| Where do review items come from? | The grader returns them | Only the grader knows what was actually tested; `srs` should not have to reverse-engineer that from a score |
| Server-side scoring only? | Yes, without exception | Client-side scoring is trivially manipulable, and progress data that cannot be trusted is worthless for both the learner and analytics |
| Where does the idempotency key travel on attempt submission? | Required `Idempotency-Key` HTTP header with UUIDv7 format | ADR-0015 requires idempotent submission. Carrying it in the standard HTTP header makes it part of the formal OpenAPI contract rather than an internal backend detail, enabling clients to safely retry on network failure (P8.1 Trap 2) |
| Single-lesson or batched lock checking in `UnlockChecker`? | Batched (`IsUnlocked(ctx, userID, lessonIDs)` returning `map[uuid.UUID]bool`) | Rendering a course detail fetches lock status for all lessons in the course (e.g. 40 lessons). Batching avoids an N+1 call into learning and matches the pattern established by `content.Reader.GetManyVersions` (P8.1 Trap 3) |
| What is the asynchronous attempt state called? | `grading` | WP8 P8.3 calls it `pending` in prose. The wire name is `grading`, because the client shows "being marked" and `pending` is what an unsubmitted attempt would sound like. Recorded because a name that differs between the work package and the spec is found the day the handler is written (P8.1) |
| Which module owns `ReviewItem` returned by `ExerciseGrader`? | `learning` module in `internal/modules/learning/contract` | All six skill modules implement `ExerciseGrader` and return `GradeResult`. If `ReviewItem` came from `srs`, modules like reading/listening/speaking/writing would violate architecture boundary rules by depending on `c_srs`. Boundary mapping to `srs` occurs in `learning` service (P8.1 Trap 1) |
| How is submission idempotency enforced on partitioned `attempts`? | A conditional claim on the attempt row, not a unique index on the key | A unique constraint on a partitioned table must include the partition key, so `UNIQUE (idempotency_key)` is unavailable and `UNIQUE (idempotency_key, created_at)` would hold only within one calendar month. The invariant that matters is narrower: **one attempt row is graded at most once**. `ClaimAttemptForGrading` carries `AND status = 'in_progress'`, so of two concurrent submissions exactly one updates the row and the other returns no rows — the database arbitrates, not a check-then-write in the service. The loser reads the attempt back: a matching `idempotency_key` means its own retry already succeeded and the stored result is the answer, a different one is a conflict. Proved by `TestLearningSchema_OneAttemptIsGradedAtMostOnce`, which fails with 2 of 2 claims when the status guard is removed (P8.2 Trap 1) |
| What mechanism and lock ID schedules monthly partition rotation? | `job.CronJob` with Postgres advisory lock `1_700_000_210` | Repository schedules periodic maintenance through `internal/platform/job` with advisory locks, not River. Lock ID `1_700_000_210` matches migration `1700000210` and is unique across all modules. Rotation runs periodically and at worker boot to prevent partition lapse outages (P8.2 Traps 3 and 4) |
| How is an activity resolved across course hierarchy without cross-module SQL joins? | Extend `lesson/contract.Reader` with `ResolveActivity(ctx, activityID)` | Resolving an activity ID to its kind, content_version_id, lesson_id, unit_id, course_id and skill_focus in one call avoids three round trips per attempt on the hottest path in the product, preserving DB1/DB2 module boundaries without cross-schema joins (P8.3 Trap 1) |
| How is the grader registry validated at startup — against the kinds in the database, or the kinds the module declares? | Registry-declared: `Deps.DeclaredKinds` checked against the registered graders in `New` | ADR-0015 requires the registry validated at startup and P8.3 §4 offers two readings. Registry-complete would have `cmd/api` read `learn.activities.kind` at boot, which makes a content author able to stop the next deploy — `kind` is free text bounded only by length. Registry-declared splits the two failures by who caused them: a wiring mistake, where a deployment claims a kind it registered no grader for, stops the process in `New` with the kind named; data asking for a kind nobody declared fails that one request with `UNSUPPORTED_ACTIVITY_KIND` (422), also naming the kind. What neither reading permits is a registry returning nil for an unknown kind, so `Get` reports absence and the service never dispatches to nil. Phase 2 declares nothing because WP9 writes the first real grader, and `New` registers nothing it was not handed (P8.3 Trap 2) |
| Is `m_learning` one arch-lint component or four? | Four, against P8.3 §2's instruction to inherit the single one | The brief says `m_learning` is one component and "you inherit that". It stops being possible the moment the module has more than one package: with `in: modules/learning/**`, go-arch-lint treats every package under it as its own import target, so module.go importing `learning/service` is a component-to-component dependency and is rejected. Naming the component in its own `mayDependOn` does not help — both forms were tried and both produce the same ten violations. Every module in this repository that has real layered code (`auth`, `rbac`, `audit`, `admin`, `content`, `lesson`) is split the same way; the `**` form survives only while a module is a single package, which is what `learning` was when the brief was written. The package layout still follows `lesson`'s, which is what §9 actually asks for (P8.3 §2) |
| Why does `GradeRequest` carry only IDs? | Pass IDs only, let graders read what they need | Carrying resolved content in GradeRequest couples all six skill modules to content representations and makes interface evolution impossible; passing IDs preserves clean boundaries (P8.3 Trap 3) |
| How does asynchronous grading behave on submit? | Returns 202 Accepted with status `grading`, leaving score unset | Synchronous graders answer 200 with score; AI-based graders (writing, speaking) return Async: true, which preserves grading status until completed by asynchronous event consumers in Phase 3 (P8.3 Trap 5) |
| What does a replayed submission return for `feedback`? | Null — the stored attempt has no prose to read back | P8.3 Trap 4 asks the loser of the claim to return the stored result rather than a fresh grade. `learn.attempts` stores `score`, `max_score`, `grader`, `duration_ms` and `status`, and P8.2 created no column for the grader's prose, so score and status replay exactly and `feedback` cannot. It briefly returned the constant `domain.FakeGrader` happens to emit, which matched the winner's body only while the fake was the only grader and told a learner who scored 0 that they were correct. Null is the honest answer; making the bodies identical needs a `feedback` column, which is a schema change this task does not own (P8.3 Trap 4) |
| How is progress rollup and event emission coordinated? | Single database transaction enclosing attempt update, progress rollup, and outbox event writes | Atomic progress update across activity, lesson, unit, and course boundaries ensures partial rollups cannot occur; outbox writes inside the transaction guarantee event delivery without dual-write race conditions (P8.3 Trap 6) |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0015](../../../docs/adr/ADR-0015-content-exercise-core.md) — Shared content + exercise engine
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we move to item response theory for mastery in Phase 5, and what would it require from `questionbank`?
<!-- END GENERATED: decisions-open -->
