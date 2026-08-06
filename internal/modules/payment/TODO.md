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

# payment — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 4

- [ ] Gateway interface and the first adapter
- [ ] Hosted checkout session creation with idempotency
- [ ] Webhook endpoint with raw-body signature verification and fast acknowledgement
- [ ] Idempotent webhook processing job
- [ ] Payments, invoices and PDF generation
- [ ] Refunds with window and permission enforcement
- [ ] Daily reconciliation job with discrepancy alerts
- [ ] Dunning schedule and communications
- [ ] Admin replay and reconciliation screens
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Second gateway for redundancy
- Tax calculation
- Metered billing for AI top-ups
- Localised invoices
<!-- END GENERATED: todo-future -->
