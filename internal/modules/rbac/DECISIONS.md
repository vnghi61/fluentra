---
module: rbac
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [roles, permissions, role_permissions, user_roles]
depends_on: [cache, audit]
depended_on_by: [auth, admin, content, questionbank, exam, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# rbac — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Casbin, OPA, or a table? | A table plus a guard function | Two roles and ~40 permissions do not justify a policy engine, a policy language, and its evaluation semantics. Casbin becomes worth it at resource-scoped or hierarchical policies — recorded in ADR-0008 so the option is not forgotten |
| Roles in the JWT or looked up per request? | Role in the token, permissions resolved from cache | The role is stable and small; permissions can change without reissuing tokens, and a 5-minute cache bounds staleness |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0008](../../../docs/adr/ADR-0008-rbac-simple-policy.md) — Table-driven permissions, no Casbin/OPA
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
