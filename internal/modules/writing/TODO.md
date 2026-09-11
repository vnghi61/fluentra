---
module: writing
tier: learning
group: modules
status: IMPLEMENTED
phase: 3
owner: "@learning-team"
schema: skill
tables: [writing_feedback]
depends_on: [ai, job, content, learning, notification]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-09-11
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

## Shipped in work orders 10 & 11

The boxes above are unticked because docgen renders every generated item that
way. What exists in code today:

- [x] `writing.Grader` implements `learning.ExerciseGrader` for
      `writing_prompt`, registered through `cmd/api/modules.go` and rendered by
      `ExerciseWriting.tsx`.
- [x] `writing_grade.v1.md` and `writing_grade.v2.md` prompts.
- [x] Background grading via River worker (`writingjob.GradeSubmissionWorker`).
- [x] `skill.writing_feedback` table storing structured IELTS feedback (criteria, located annotations, band scores).
- [x] `GET /writing/attempts/{id}/feedback` endpoint for reading learner feedback.

### Schema rationalisation (tables retired)

- `writing_tasks`: prompts and rubrics are authored directly into `content.versions` (kind `writing_prompt`), eliminating a duplicate tasks table.
- `writing_drafts`: drafts are saved client-side in localStorage (`fluentra.writing_draft.<activityId>`), eliminating ephemeral database writes.
- `writing_revisions`: draft revisions are handled locally; server-side snapshot history is deferred until high-stakes exam tracking is built.

The only table `writing` owns is `writing_feedback`.
