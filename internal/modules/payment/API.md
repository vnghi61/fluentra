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

# payment — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `payment`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/webhooks/payment/sepay` | `public` | Ingest SePay incoming bank transfer webhook |
| `GET` | `/api/v1/me/orders/{id}` | `self` | Poll order status |
| `GET` | `/api/v1/admin/payments/unmatched` | `billing.read` | List unmatched incoming transactions for operator resolution |
| `GET` | `/api/v1/admin/billing/refunds` | `billing.read` | Refunds owed to learners, which an admin pays by bank transfer |
| `POST` | `/api/v1/admin/billing/refunds/{id}/sent` | `billing.manage` | Record that a refund has been transferred |
| `GET` | `/api/v1/admin/billing/payouts` | `billing.read` | Payouts owed to creators; bank details are not in this response |
| `GET` | `/api/v1/admin/billing/payouts/{id}` | `billing.manage` | One payout with the creator bank account, so an admin can transfer it |
| `POST` | `/api/v1/admin/billing/payouts/{id}/fulfill` | `billing.manage` | Record that a creator payout has been transferred |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/webhooks/payment/sepay`

Ingest SePay incoming bank transfer webhook

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/orders/{id}`

Poll order status

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/payments/unmatched`

List unmatched incoming transactions for operator resolution

| | |
|---|---|
| Permission | `billing.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/billing/refunds`

Refunds owed to learners, which an admin pays by bank transfer

| | |
|---|---|
| Permission | `billing.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/billing/refunds/{id}/sent`

Record that a refund has been transferred

| | |
|---|---|
| Permission | `billing.manage` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/billing/payouts`

Payouts owed to creators; bank details are not in this response

| | |
|---|---|
| Permission | `billing.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/billing/payouts/{id}`

One payout with the creator bank account, so an admin can transfer it

| | |
|---|---|
| Permission | `billing.manage` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/billing/payouts/{id}/fulfill`

Record that a creator payout has been transferred

| | |
|---|---|
| Permission | `billing.manage` |
| Success | 200 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `ORDER_NOT_FOUND` | 404 | Order does not exist |
| `ORDER_EXPIRED` | 409 | Order has expired |
| `INVALID_WEBHOOK_KEY` | 401 | SePay API key invalid or unauthorized |
| `RATE_LIMIT_EXCEEDED` | 429 | SePay API rate limit exceeded (3 req/s) |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
