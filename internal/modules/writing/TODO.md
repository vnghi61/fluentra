---
module: writing
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [writing_tasks, writing_drafts, writing_submissions, writing_feedback, writing_revisions]
depends_on: [ai, job, content, learning, notification]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# writing — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] Tasks, drafts with autosave, revision snapshots
- [ ] Submission with idempotency and server-side bounds
- [ ] Async grading job calling `writing.grade_essay`
- [ ] SSE streaming with partial persistence and reconnection
- [ ] Band and criterion validation with clamping
- [ ] Feedback rendering with inline annotations
- [ ] History view with band progression
- [ ] Dispute flow and admin review queue
- [ ] Red-team eval suite for injection
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Second-opinion grading for borderline bands
- Model ensemble for high-stakes submissions
- Guided revision with tracked improvement
- Peer review
<!-- END GENERATED: todo-future -->
