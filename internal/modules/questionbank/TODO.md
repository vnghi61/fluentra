---
module: questionbank
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_stats]
depends_on: [content, lesson, learning, rbac]
depended_on_by: [exam]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Open

- [ ] A job computing `question_stats` from real attempts (the table exists; nothing writes it)
- [ ] Bank course layout per exam part: lessons are per kind today, not per part as WO 19 F.2 describes
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Item response theory calibration
- Automatic distractor quality analysis
- Near-duplicate detection by embedding
- Item retirement policy from statistics
<!-- END GENERATED: todo-future -->
