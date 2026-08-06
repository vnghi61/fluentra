---
module: learning
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [enrollments, progress, attempts, learning_sessions, placement_results, skill_mastery]
depends_on: [lesson, content, srs, cache, job]
depended_on_by: [gamification, analytics, admin, exam, vocabulary, grammar, reading, listening, speaking, writing]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# learning — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| One engine or per-skill flows? | One engine, pluggable graders | Attempt lifecycle, idempotency, progress rollup and event emission are identical across skills; only scoring differs. Six copies would drift and each would need its own idempotency bug fixed separately |
| Where do review items come from? | The grader returns them | Only the grader knows what was actually tested; `srs` should not have to reverse-engineer that from a score |
| Server-side scoring only? | Yes, without exception | Client-side scoring is trivially manipulable, and progress data that cannot be trusted is worthless for both the learner and analytics |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0015](../../../docs/adr/ADR-0015-content-exercise-core.md) — Shared content + exercise engine
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we move to item response theory for mastery in Phase 5, and what would it require from `questionbank`?
<!-- END GENERATED: decisions-open -->
