---
module: listening
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [audio_items, transcripts, listening_attempts]
depends_on: [content, media, questionbank, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# listening — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] Audio items with transcripts and metadata
- [ ] Attempt with server-side play counting
- [ ] Graders for MCQ, gap-fill, dictation and ordering
- [ ] Transcript reveal policy
- [ ] Study-mode segment replay
- [ ] Player UI for both modes
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Adaptive speed for study mode
- Accent-targeted practice
- Shadowing exercises combining listening and speaking
<!-- END GENERATED: todo-future -->
