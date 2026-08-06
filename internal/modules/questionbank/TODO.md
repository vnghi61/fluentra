---
module: questionbank
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_options, question_sets, question_set_items, question_stats]
depends_on: [content, ai, audit, search]
depended_on_by: [exam, reading, listening, grammar, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 4

- [ ] Items, options, sets and tagging
- [ ] Authoring and review workflow with self-approval blocking
- [ ] Learner DTO without answers, structurally enforced
- [ ] Statistics job computing p-value and discrimination
- [ ] Sampling with exposure control
- [ ] AI generation into the review queue
- [ ] Admin authoring UI with bulk import
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
