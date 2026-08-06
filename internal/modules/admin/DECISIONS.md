---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [feature_flags, admin_notes, moderation_items]
depends_on: [user, rbac, audit, auth, content, cache]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-08-06
---

# admin — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Separate admin service or the same binary? | Same binary, separate route group | A separate service would duplicate every contract call over the network for a handful of screens; the route group plus per-operation permissions gives the isolation that matters |
| Does admin own data? | Only flags, notes and the moderation queue | Anything else would duplicate another module's state and drift from it |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
