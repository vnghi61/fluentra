---
module: learning
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [enrollments, progress, attempts, learning_sessions, placement_results, skill_mastery, answer_explanations]
depends_on: [lesson, content, srs, cache, job]
depended_on_by: [gamification, analytics, admin, exam, vocabulary, grammar, reading, listening, speaking, writing]
spec_version: 1.0.0
last_verified: 2026-09-03
---

# learning — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] Enrolment, progress and attempt tables with partitioning
- [ ] Exercise engine with grader registry and idempotent submission
- [ ] Synchronous grading path end to end with the vocabulary grader
- [ ] Progress rollup activity → lesson → unit → course
- [ ] Dashboard endpoint with caching and a query-count assertion
- [ ] Session tracking
- [ ] Unlock evaluation

## Phase 3

- [ ] Asynchronous grading path for writing and speaking
- [ ] Skill mastery estimation and the skill radar
- [ ] Adaptive daily plan

## Phase 4

- [ ] Adaptive placement test
- [ ] Personalised path generation from placement plus mastery
<!-- END GENERATED: todo -->

## Progress

The list above is generated from `tools/docgen/data/learning.json`, so its checkboxes cannot be
ticked by hand — `make docs` rewrites the block and `make docs-check` fails until it matches.
Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P8.1 | 2026-08-24 | Contract types in `internal/modules/learning/contract/contract.go` (`ExerciseGrader`, `GradeResult`, `ProgressReader`, batched `UnlockChecker`, `ReviewItem`, `GradeRequest`, and 5 published event payloads); OpenAPI 3.1 surface with 8 paths, `Idempotency-Key` header parameter on attempt submit, component schemas in `api/openapi/components/learning.yaml` expressing both new-learner dashboard states and async grading results; generated Go server/client interfaces and TypeScript API types. |
| P8.2 | 2026-08-24 | Database schema in `db/migrations/learning/1700000210_create_learning_tables.sql` for the 6 `learn` tables (`enrollments`, `progress`, `attempts` with monthly range partitioning, `learning_sessions`, `placement_results`, `skill_mastery`); partition management function `learn.ensure_partitions`; `ClaimAttemptForGrading`, the conditional claim that makes submission idempotent without a unique index the partitioning forbids; sqlc queries and generated Go code; worker cron job and start-up partition rotation wired in `cmd/worker/main.go` with advisory lock `1_700_000_210`; architecture lint boundaries configured; full schema integration test suite in `db/migrations/learning/schema_integration_test.go`. |
| P8.3 | 2026-08-24 | Exercise engine and attempt lifecycle: grader registry with startup validation (Trap 2); fake synchronous and asynchronous graders in `learning/domain`; atomic claim `ClaimAttemptForGrading` enforcing at-most-once grading and concurrent double-submission idempotency (Trap 4); asynchronous grading support returning 202 Accepted (Trap 5); single-transaction progress rollup (`activity` → `lesson` → `unit` → `course`) with outbox events `activity.completed`, `lesson.completed`, `course.completed` (Trap 6); `lesson/contract.Reader` extended with `ResolveActivity` to avoid cross-module SQL joins (Trap 1); HTTP handlers mounted in `cmd/api/modules.go`; comprehensive unit, boundary, and concurrent race tests. |
| P8.4 | 2026-08-24 | Enrolment, unlocking, mastery, sessions and next-activity resolution: `ProgressReader` and the batched `UnlockChecker` implemented and wired into `lesson` through a lazy adapter in `cmd/api` (Trap 1), with `min_score` enforced; `lesson/contract.Reader` grew `ListPrerequisitesForLessons` and `ListUnitsByCourseID`; enrolment restricted to the statuses both the CHECK constraint and the OpenAPI enum allow, with `ALREADY_ENROLLED` and `COURSE_NOT_FOUND` mapped from the constraints (Trap 2); `rollupCourse` rewritten to require every unit complete and to close the enrolment in the same transaction (Trap 3); EWMA skill mastery written inside the grading transaction, skipping an unrecognised `skill_focus` rather than failing the submission (Trap 4); next-activity resolution in three explicit states with reads bounded by units, not lessons (Trap 5); `StartAttempt` now enforces enrolment and unlocking, sessions timed server-side with `learning.session_completed` in the completing transaction (Trap 6); enrol and session endpoints mounted. |
| P8.5 | 2026-08-24 | Learning dashboard and progress endpoints: `GET /me/dashboard` and `GET /me/progress` implemented and mounted; `lesson/contract.Reader` grew `ListActivitiesByCourseIDs` with single-round-trip batched resolution in `lesson` queries (Trap 1); bounded query budget assertions verifying constant database queries regardless of course size (30 vs 400 lessons fixture) (Trap 2); Redis caching with 2 min TTL for dashboard and 5 min TTL for progress, and post-commit cache invalidation in attempt submission, enrollment, and session completion (Trap 3); `due_reviews_count` explicitly returns 0 with no srs imports (Trap 4); JSON marshaling asserts empty array `[]` (never `null`), omitted `next_activity` for `not_started` and `completed` states, and status mappings (Trap 5); zero-division protection and percentage integer rounding tested (Trap 6). |
| WP17 | 2026-09-03 | Phase 3 WP17 AI Answer Explanations: Migration `1700000450_answer_explanations.sql` creating `learn.answer_explanations` with unique index `(content_version_id, user_answer)`; bilingual AI prompt `explain_answer.v1.md` generating English and Vietnamese explanations in a single model completion; lazy generation on first encounter with permanent database caching across all learners; graceful quota fallback with zero disruption; OpenAPI schema and client codegen; full frontend exercise runner integration with `ExerciseFeedback` and `LessonPage`. |

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
- A per-lesson unlock cache (`fluentra:{env}:learning:unlock:{user_id}:{lesson_id}:v1`). `IsUnlocked` is two batched reads for a whole course tree, so the cache would save less than the invalidation it needs: a lesson completion changes the verdict for every lesson downstream of it, and the keys are per lesson. Revisit if the course-detail page shows up in latency work (P8.5)
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Item response theory for mastery
- Learned rather than rule-based adaptation
- Offline attempt sync
- Asynchronous progress rollup
<!-- END GENERATED: todo-future -->
