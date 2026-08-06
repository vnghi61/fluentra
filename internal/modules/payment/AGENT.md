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

# payment — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `commerce` |
| Path | `internal/modules/payment` |
| Schema | `billing` |
| Delivery phase | 4 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Money: gateway adapters, hosted checkout sessions, webhook processing, invoices, refunds and reconciliation. Card data never touches our systems — the gateway's hosted fields do, which keeps PCI scope minimal.
<!-- END GENERATED: overview -->

**Context.** The provider is not yet chosen (plan review Q1: VNPay/MoMo for Vietnam, Stripe for international). Everything here is written against an adapter interface so that decision is a configuration change plus one adapter file.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Gateway adapters behind one interface
- Checkout session creation and redirect handling
- Webhook receipt, signature verification, idempotent processing and replay
- Payment and invoice records
- Refunds, partial and full
- Reconciliation between our records and the gateway's
- Dunning: retry schedule and communications on failed renewals

**This module does NOT own:**

- Deciding what access a payment buys — that is `subscription`
- Holding card data — the gateway does, and we never proxy it
- Tax calculation in v1
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/payment/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/payment/contract/` | You are calling this module from another module |
| `internal/modules/payment/service/` | You are changing behaviour |
| `db/migrations/payment/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/payment/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `payment.Checkout` | `CreateSession(ctx, userID, planID)` — used by `subscription` |
| interface | `payment.Gateway` | The adapter Strategy interface; internal to this module |
| interface | `payment.Refunder` | `Refund(ctx, paymentID, amount, reason)` — admin only |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `payment.succeeded` | publishes | `{user_id, payment_id, plan_code, amount}` |
| `payment.failed` | publishes | `{user_id, payment_id, failure_code}` |
| `payment.refunded` | publishes | `{user_id, payment_id, amount}` |
| `subscription.expiring` | consumes | Schedule the renewal charge |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `billing` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/payment/` · Queries: `db/queries/payment/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `billing.payments` | One gateway transaction | `user_id`, `provider`, `provider_payment_id` UNIQUE, `amount` numeric + `currency`, `status`, `failure_code` |
| `billing.invoices` | Billing document | `user_id`, `number` UNIQUE, `period`, `lines` jsonb, `total`, `status`, `pdf_object_key` |
| `billing.payment_webhooks` | Raw webhook log | `provider`, `provider_event_id` UNIQUE, `payload` jsonb, `signature_valid`, `processed_at`. Enables replay. |
| `billing.refunds` | Refund records | `payment_id`, `amount`, `reason`, `actor_id`, `provider_refund_id` |
| `billing.checkout_sessions` | In-flight checkouts | `user_id`, `plan_id`, `provider_session_id`, `status`, `expires_at` |


<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `payment`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/billing/checkout` | `self` | Create a hosted checkout session and return its redirect URL |
| `GET` | `/api/v1/billing/checkout/{id}` | `self` | Poll a checkout session's status after redirect |
| `POST` | `/api/v1/webhooks/payment/{provider}` | `public` | Gateway webhook |
| `GET` | `/api/v1/me/invoices` | `self` | Invoice history |
| `GET` | `/api/v1/me/invoices/{id}/pdf` | `self` | Signed link to the invoice PDF |
| `POST` | `/api/v1/admin/payments/{id}/refund` | `billing.refund` | Issue a refund |
| `POST` | `/api/v1/admin/webhooks/{id}/replay` | `billing.manage` | Replay a stored webhook |
| `GET` | `/api/v1/admin/reconciliation` | `billing.read` | Discrepancies between our records and the gateway |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`subscription`](../../modules/subscription/AGENT.md) | → depends on | Knows what is being purchased and what a successful payment should activate |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | Every money movement is audited |
| [`job`](../../platform/job/AGENT.md) | → depends on | Webhook processing, dunning and reconciliation run asynchronously |
| [`mailer`](../../platform/mailer/AGENT.md) | → depends on | Receipts and dunning emails |
| [`storage`](../../platform/storage/AGENT.md) | → depends on | Invoice PDFs |
| [`subscription`](../../modules/subscription/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-PAYMENT-01** — Card data never reaches our servers. Checkout is hosted by the gateway; we hold only tokens and identifiers.
2. **BR-PAYMENT-02** — Webhook signatures are verified against the **raw** body before any parsing. An unverified webhook is logged and rejected.
3. **BR-PAYMENT-03** — Webhook processing is idempotent on `provider_event_id`; a duplicate is acknowledged without reprocessing.
4. **BR-PAYMENT-04** — Webhooks are acknowledged within 2 seconds and processed in a job — a slow handler causes the gateway to retry and duplicate.
5. **BR-PAYMENT-05** — Every webhook is stored raw, so it can be replayed after a bug fix without asking the gateway to resend.
6. **BR-PAYMENT-06** — Checkout creation requires an `Idempotency-Key`; a network retry must not create a second session or a second charge.
7. **BR-PAYMENT-07** — The gateway is the source of truth for whether money moved; our records are reconciled against it daily, and a discrepancy raises an alert rather than being auto-corrected.
8. **BR-PAYMENT-08** — Refunds require a reason and a permission, and are always audited.
9. **BR-PAYMENT-09** — Dunning follows a fixed schedule (day 1, 3, 5) with clear communications, and stops immediately on success.
10. **BR-PAYMENT-10** — Amounts are `numeric` with an explicit currency; floating point is never used for money anywhere in this module.
11. **BR-PAYMENT-11** — A payment failure never revokes access directly — it emits an event and `subscription` decides.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a payment provider

1. Implement `payment.Gateway` in `gateway/<provider>/`; the SDK may be imported only there.
2. Implement signature verification against the raw body, and test it with a forged payload.
3. Map the provider's event names onto our internal event set.
4. Add sandbox credentials for CI and a fixture set covering success, decline, timeout and duplicate webhook.
5. Run the reconciliation job against the sandbox.
6. Get a second reviewer — this module requires two (rule S11).
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- No tax or VAT calculation in v1; prices are tax-inclusive and this is stated at checkout.
- No dunning personalisation beyond the fixed schedule.
- Invoice PDFs are generated from a simple template with no localisation beyond currency formatting.
- Reconciliation is daily, so a discrepancy can persist for up to a day before it is visible.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->


### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `PAYMENT_FAILED` | 402 | Gateway declined |
| `CHECKOUT_EXPIRED` | 409 | Session expired before completion |
| `ALREADY_REFUNDED` | 409 | Payment already fully refunded |
| `REFUND_WINDOW_CLOSED` | 409 | Outside the refund policy period |
| `WEBHOOK_SIGNATURE_INVALID` | 400 | Signature verification failed |
| `RECONCILIATION_MISMATCH` | 409 | Our record disagrees with the gateway |

### Security considerations

- PCI scope is limited to the gateway's hosted fields; we never see, transmit, or store a card number.
- Webhook endpoints are public by necessity and are therefore rate-limited, signature-verified and logged in full.
- Refund and grant permissions are separate from general billing read access.
- Every state change in this module is audited with the actor, and admin actions require a stated reason.
- Two reviewers are required for any change to this module (rule S11).

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **90% service (money-handling code carries the highest correctness bar in the repository)**

```bash
go test ./internal/modules/payment/...                    # unit
go test -tags=integration ./internal/modules/payment/...  # integration (testcontainers)
```

**Focus areas**

- Webhook signature verification rejects a forged or replayed-with-changes payload
- Duplicate webhook produces exactly one payment record
- Idempotent checkout creation under a network retry
- Refund partial and full, with window enforcement
- Reconciliation detects a seeded discrepancy
- Money arithmetic never uses floating point (asserted by a lint rule and a test)
- A payment failure does not itself revoke access
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not accept or store card data.
- Do not parse a webhook before verifying its signature on the raw body.
- Do not process a webhook synchronously in the request.
- Do not use floating point for money.
- Do not revoke access from this module — publish an event.
- Do not auto-correct a reconciliation discrepancy; raise it.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
