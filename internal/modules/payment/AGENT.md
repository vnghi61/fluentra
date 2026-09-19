---
module: payment
tier: commerce
group: modules
status: ACTIVE
phase: 3
owner: "@backend-team"
schema: billing
tables: [orders, sepay_transactions, payment_webhooks, refunds, payouts]
depends_on: [audit, job]
depended_on_by: [studio, admin]
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
| Status | **ACTIVE** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Money: SePay bank transfer integration, orders, webhook receipt and matching, VietQR generation, refunds and daily reconciliation. No card data exists — payment is a bank transfer the payer initiates via VietQR or banking app.
<!-- END GENERATED: overview -->

**Context.** The provider is not yet chosen (plan review Q1: VNPay/MoMo for Vietnam, Stripe for international). Everything here is written against an adapter interface so that decision is a configuration change plus one adapter file.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Bank transfer order creation and VietQR image generation
- SePay webhook receipt with API key authentication and fast acknowledgement
- Idempotent transaction matching on alphanumeric transfer content
- Reconciliation between SePay transactions and our billing database
- Order expiry sweep
- Admin unmatched queue for manual resolution

**This module does NOT own:**

- Deciding what access a payment buys — that is `studio` or `subscription`
- Holding card data — no card exists
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
| interface | `payment.OrderCreator` | CreateOrder(ctx, in CreateOrderInput) (*Order, error) |
| interface | `payment.OrderReader` | GetOrder(ctx, id uuid.UUID) (*Order, error) |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `payment.succeeded` | publishes | `{user_id, order_id, subject_kind, subject_id, amount_vnd}` |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `billing` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/payment/` · Queries: `db/queries/payment/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `billing.orders` | Bank transfer orders | `user_id`, `reference` UNIQUE, `amount_vnd` bigint, `status`, `expires_at`, `paid_at` |
| `billing.sepay_transactions` | Incoming SePay transactions | `sepay_id` UNIQUE, `gateway`, `transaction_date`, `account_number`, `transfer_amount` bigint, `matched_order_id`, `unmatched_reason` |
| `billing.payment_webhooks` | Raw webhook log | `sepay_id` UNIQUE, `payload` jsonb, `processed_at` |
| `billing.refunds` | Refund records | `order_id`, `amount_vnd` bigint, `reason`, `actor_id` |
| `billing.payouts` | Creator payout records | `creator_id`, `amount_vnd` bigint, `status` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `payment`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/webhooks/payment/sepay` | `public` | Ingest SePay incoming bank transfer webhook |
| `GET` | `/api/v1/me/orders/{id}` | `self` | Poll order status |
| `GET` | `/api/v1/admin/payments/unmatched` | `admin.dashboard` | List unmatched incoming transactions for operator resolution |
| `GET` | `/api/v1/admin/billing/payouts` | `admin.dashboard` | List creator payout requests |
| `GET` | `/api/v1/admin/billing/payouts/{id}` | `admin.dashboard` | Get payout request detail and creator bank account |
| `POST` | `/api/v1/admin/billing/payouts/{id}/fulfill` | `admin.dashboard` | Mark creator payout as fulfilled with bank reference |
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
| [`audit`](../../modules/audit/AGENT.md) | → depends on | Every money movement is audited |
| [`job`](../../platform/job/AGENT.md) | → depends on | Webhook transaction matching, order expiry sweep and daily reconciliation |
| [`studio`](../../modules/studio/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-PAYMENT-01** — BR-PAYMENT-01: No card exists. Payment is a bank transfer the payer initiates; we hold a reference, an amount and what the bank told us.
2. **BR-PAYMENT-02** — BR-PAYMENT-02: Webhook authentication uses an API key verified in constant time. Empty key disables the webhook route.
3. **BR-PAYMENT-03** — BR-PAYMENT-03: Webhook processing is idempotent on `sepay_id`; duplicate deliveries are acknowledged and dropped.
4. **BR-PAYMENT-04** — BR-PAYMENT-04: The acknowledgement budget is SePay's 30 seconds, and the body must be `{"success": true}`; we answer in under two seconds.
5. **BR-PAYMENT-05** — BR-PAYMENT-05: Every webhook is stored raw in `billing.payment_webhooks` for replay and audit.
6. **BR-PAYMENT-06** — BR-PAYMENT-06: Matching extracts the order reference case-insensitively after stripping non-alphanumerics from transfer content.
7. **BR-PAYMENT-07** — BR-PAYMENT-07: The bank is the source of truth; daily reconciliation catches missed webhooks and flags discrepancies.
8. **BR-PAYMENT-08** — BR-PAYMENT-08: Order lifetime is 24 hours. A late payment arriving against an expired order reopens it.
9. **BR-PAYMENT-09** — BR-PAYMENT-09: Hourly sweep marks expired unpaid orders using advisory lock 1_700_000_751.
10. **BR-PAYMENT-10** — BR-PAYMENT-10: Daily reconciliation uses advisory lock 1_700_000_752 with rate limiting at max 3 requests/sec.
11. **BR-PAYMENT-11** — BR-PAYMENT-11: Amounts are `bigint` in VND with no subunit decimals. Floating point is forbidden.
12. **BR-PAYMENT-12** — BR-PAYMENT-12: An amount that does not match exactly is never partially credited.
13. **BR-PAYMENT-13** — BR-PAYMENT-13: A payment against an expired order is honoured; the money is real.
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

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
