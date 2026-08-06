---
module: notification
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: comm
tables: [notifications, notification_preferences, devices, notification_dedupe]
depends_on: [mailer, job, cache, user]
depended_on_by: [auth, writing, speaking, exam, gamification, subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# notification — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| One notification module or per-channel modules? | One | Preferences, quiet hours, dedupe and digesting are channel-independent policy; splitting by channel would triplicate the policy and let the three copies diverge |
| Write in-app notifications even when other channels are disabled? | Yes | The inbox is the durable record; suppressing it would lose information the learner may want later |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
