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
