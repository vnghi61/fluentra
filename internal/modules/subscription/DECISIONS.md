---
module: subscription
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [plans, entitlements, subscriptions, subscription_events]
depends_on: [payment, user, notification, cache, job, audit]
depended_on_by: [writing, speaking, vocabulary, exam, ai, admin, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# subscription — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Entitlements from subscription state or from payment? | Subscription state | It decouples access from the gateway's availability and makes complimentary grants, trials and grace periods expressible without faking a payment |
| Cut off access immediately on a failed payment? | No — a 7-day grace period | Most renewal failures are expired cards, not intent to churn; cutting access instantly converts a fixable billing problem into a lost learner |
| Downgrade immediately or at period end? | At period end | The learner paid for the period; taking capability away early is both unfair and a support burden |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
