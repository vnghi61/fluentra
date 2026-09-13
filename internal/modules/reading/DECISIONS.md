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
last_verified: 2026-09-10
---

# reading — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Where do reading questions live? | In content version bodies | Passage and questions are bundled directly into self-contained `content.versions` rows, avoiding separate tables; revisit when `exam` is built |
| Measure reading speed how? | First render to reading-complete | Including question time would conflate reading with reasoning and make the metric useless for progress |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
