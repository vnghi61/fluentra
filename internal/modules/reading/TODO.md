---
module: reading
tier: learning
group: modules
status: IMPLEMENTED
phase: 3
owner: "@learning-team"
schema: skill
tables: [passages, passage_questions, reading_attempts]
depends_on: [content, questionbank, vocabulary, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-09-10
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

## Shipped in work order 10

The boxes above are unticked because docgen renders every generated item that
way. What exists in code today is the **grader**, and only that:

- [x] `reading.Grader` implements `learning.ExerciseGrader` for
      `reading_comprehension`, registered through `cmd/api/modules.go` and
      rendered by `ExerciseReading.tsx`.
- [x] Passages are authored `content` bodies, not rows of this module's own.

`status` stays `PLANNED` on purpose. The front matter still names `passages`,
`passage_questions` and `reading_attempts`, and none of them exist — ADR-0015
settles the attempt table, and the passage lives in `content` because a passage
is authored material like any other. Marking the module DONE while its own
front matter lists three tables nobody wrote is the kind of record this
repository keeps having to correct. Retire the table list, or build it, and then
change the status deliberately.

Not built: WPM tracking, adaptive selection, evidence spans.
