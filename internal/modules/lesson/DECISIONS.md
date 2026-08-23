---
module: lesson
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [courses, course_units, lessons, activities, lesson_prerequisites]
depends_on: [content, cache]
depended_on_by: [learning, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# lesson — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Bind activities to items or versions? | Versions | A learner's attempt must be interpretable against exactly what they saw; item binding would let a republish change the meaning of past results |
| Where do unlocking rules live? | Defined here, evaluated by `learning` | The rule is structural (a property of the curriculum); the state is personal (a property of the learner). Splitting them keeps both modules honest |
| Are the course and lesson reads public? | No — signed in, with `content.read.published` | A lesson response carries its activities and their resolved content versions, so an anonymous `/lessons/{id}` publishes the whole course and removes the surface Phase 4 attaches entitlements to. The `user` role holds the read permission; writing courses and reordering activities stays with `admin` |
| How to invalidate the catalogue cache without wildcards? | Generation counter in Redis | The catalogue key has one hash per level/limit/offset permutation. Storing a generation counter in Redis and bumping it on publish invalidates all filter combinations instantly without an expensive SCAN in the request path (P7.5 Trap 1) |
| When does cache invalidation take place relative to publish? | Synchronously in service post-commit, plus outbox event backstop | Outbox events reach subscribers through `cmd/worker`, so an event-only invalidation leaves a window in which an author reloads the course and still sees the pre-publish tree. `PublishLesson` therefore deletes the keys itself once the transaction commits; Redis is shared, so that delete is visible to every process immediately. A `lesson.published` consumer in the worker repeats it as the backstop for a delete that failed while Redis was unreachable — a failed delete only warns, it never fails the publish (P7.5 Traps 2 and 3) |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
