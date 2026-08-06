---
module: payment
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [payments, invoices, payment_webhooks, refunds, checkout_sessions]
depends_on: [subscription, audit, job, mailer, storage]
depended_on_by: [subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# payment — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Hosted checkout or our own card form? | Hosted | It keeps PCI scope to SAQ-A, removes an entire class of liability, and the conversion difference does not justify the risk |
| Trust webhooks or poll? | Webhooks, reconciled daily | Webhooks are timely but can be missed or duplicated; daily reconciliation against the gateway catches both without polling constantly |
| Store raw webhooks? | Yes | Replay after a bug fix is the difference between a five-minute recovery and asking a payment provider to resend a week of events |
| Who decides access after a payment? | `subscription`, via an event | Keeping the money boundary separate from the access boundary means a gateway problem cannot silently change what learners can do |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Which gateway — VNPay/MoMo for Vietnam, Stripe for international, or both behind the adapter? (plan review Q1)
<!-- END GENERATED: decisions-open -->
