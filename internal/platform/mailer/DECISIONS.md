---
module: mailer
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: comm
tables: [email_log, email_suppressions]
depends_on: [job, telemetry, storage]
depended_on_by: [auth, user, notification, subscription, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# mailer — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| SMTP or provider API? | One interface, both implementations | SMTP works everywhere including local development; a provider API gives better deliverability signals in production. The choice becomes configuration |
| Template format? | MJML compiled at build time | Email HTML is a compatibility minefield; MJML solves it and the compile step keeps runtime rendering cheap |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
