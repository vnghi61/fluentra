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

# exam — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 4

- [ ] Exam and section definitions with navigation rules
- [ ] Attempt with server-authoritative timing and autosave
- [ ] Auto-submit job
- [ ] Objective scoring and band conversion
- [ ] Async dispatch to writing and speaking
- [ ] Score report generation and storage
- [ ] Integrity signal collection
- [ ] Exam player UI with a faithful timer
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Adaptive placement variant
- Percentile comparison once the population is large enough
- Proctoring integration
- Section-only practice mode
<!-- END GENERATED: todo-future -->
