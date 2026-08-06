---
module: audit
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: audit
tables: [audit_logs, security_events]
depends_on: [job]
depended_on_by: [auth, user, rbac, admin, content, questionbank, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# audit — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Postgres or a log stream for audit? | Postgres | Audit is queryable business record with retention and access-control requirements; Loki is optimised for cheap, short-lived telemetry and is not a system of record |
| Synchronous or asynchronous recording? | Both, by criticality | Permission and money changes go through the outbox so they cannot be lost; reads are best-effort so an audit outage cannot break browsing |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
