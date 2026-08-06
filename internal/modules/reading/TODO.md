---
module: reading
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [passages, passage_questions, reading_attempts]
depends_on: [content, questionbank, vocabulary, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# reading — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] Passages with difficulty metadata and attribution
- [ ] Question set binding to `questionbank`
- [ ] Timed attempt with WPM measurement
- [ ] Graders for MCQ, T/F/NG, matching, gap-fill and span
- [ ] Inline glossing with lookup recording
- [ ] Reading player in the web app
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Adaptive passage selection
- Extensive-reading mode with a library
- Automatic question generation reviewed by an admin
<!-- END GENERATED: todo-future -->
