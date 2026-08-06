---
module: speaking
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_tasks, speaking_attempts, pronunciation_scores, speaking_feedback]
depends_on: [media, ai, storage, job, content, learning]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# speaking — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] Tasks, attempts and scores model
- [ ] Consent flow and recording UI with timers
- [ ] Presigned upload plus post-upload verification
- [ ] Media pipeline orchestration
- [ ] Pronunciation heat map rendering data
- [ ] AI coaching via `speaking.feedback`
- [ ] Retention job and recording deletion
- [ ] Score progression history
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Real-time streaming feedback
- Noise suppression before ASR
- Accent-aware scoring calibration
- Conversation practice with a model
<!-- END GENERATED: todo-future -->
