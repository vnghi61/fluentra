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
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0015](../../../docs/adr/ADR-0015-content-exercise-core.md) — Shared content + exercise engine
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we move to item response theory for mastery in Phase 5, and what would it require from `questionbank`?
<!-- END GENERATED: decisions-open -->
