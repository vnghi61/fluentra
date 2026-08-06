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
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
