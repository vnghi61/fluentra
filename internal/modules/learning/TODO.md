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
last_verified: 2026-08-24
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

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Item response theory for mastery
- Learned rather than rule-based adaptation
- Offline attempt sync
- Asynchronous progress rollup
<!-- END GENERATED: todo-future -->
