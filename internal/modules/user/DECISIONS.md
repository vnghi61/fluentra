---
module: user
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [users, profiles, user_preferences, learning_profiles, user_deletion_requests, user_exports]
depends_on: [storage, mailer, audit]
depended_on_by: [auth, admin, learning, notification, subscription, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# user — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| One `users` table or split identity and profile? | Split | `users` is referenced by every schema; keeping it narrow and stable means profile churn never touches a table with a dozen inbound foreign keys |
| Hard delete or anonymise? | Hard-delete PII, anonymise statistics | Satisfies erasure while keeping historical aggregates truthful — deleting the rows outright would silently rewrite past reports |
| Who owns cross-module erasure? | Each module, driven by an event | A central deletion script would need to know every module's schema, which is exactly the coupling the architecture exists to prevent |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we need a verified email-change flow in Phase 2, or is support-assisted change acceptable?
<!-- END GENERATED: decisions-open -->
