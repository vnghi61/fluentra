---
module: search
tier: platform
group: platform
status: PLANNED
phase: 4
owner: "@platform-team"
schema: none
tables: []
depends_on: [cache, job]
depended_on_by: [content, vocabulary, questionbank, lesson, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# search — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Postgres FTS or a search engine? | Postgres FTS | Our corpus is small, our relevance needs are simple, and avoiding a second datastore avoids a synchronisation problem. The interface keeps the door open — and the p95 metric is the objective trigger to walk through it |
| Who owns the index? | The module that owns the data | A central index would need to know every module's schema — precisely the coupling the architecture forbids |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
