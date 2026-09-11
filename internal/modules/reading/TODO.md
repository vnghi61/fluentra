---
module: reading
tier: learning
group: modules
status: IMPLEMENTED
phase: 3
owner: "@learning-team"
schema: skill
tables: []
depends_on: [content, questionbank, vocabulary, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-09-11
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

## Shipped in work orders 10 & 11

The boxes above are unticked because docgen renders every generated item that
way. What exists in code today:

- [x] `reading.Grader` implements `learning.ExerciseGrader` for
      `reading_comprehension`, registered through `cmd/api/modules.go` and
      rendered by `ExerciseReading.tsx`.
- [x] Passages and question sets are authored and generated as self-contained `content.versions` bodies.
- [x] Practice pool generation and daily practice set integration (§3.11).

### Schema rationalisation (tables retired)

- `passages` and `passage_questions`: retired because reading passages and question sets are stored directly in `content.versions` (kind `reading_comprehension`), eliminating separate passage tables.
- `reading_attempts`: retired per ADR-0015; all attempt lifecycles are centralized in `learn.attempts`.

The module `reading` owns 0 database tables (`tables: []`).
