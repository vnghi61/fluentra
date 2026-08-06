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
| `POST` | `/api/v1/billing/checkout` | `self` | Create a hosted checkout session and return its redirect URL |
| `GET` | `/api/v1/billing/checkout/{id}` | `self` | Poll a checkout session's status after redirect |
| `POST` | `/api/v1/webhooks/payment/{provider}` | `public` | Gateway webhook |
| `GET` | `/api/v1/me/invoices` | `self` | Invoice history |
| `GET` | `/api/v1/me/invoices/{id}/pdf` | `self` | Signed link to the invoice PDF |
| `POST` | `/api/v1/admin/payments/{id}/refund` | `billing.refund` | Issue a refund |
| `POST` | `/api/v1/admin/webhooks/{id}/replay` | `billing.manage` | Replay a stored webhook |
| `GET` | `/api/v1/admin/reconciliation` | `billing.read` | Discrepancies between our records and the gateway |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/billing/checkout`

Create a hosted checkout session and return its redirect URL

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `PLAN_NOT_AVAILABLE`, `SUBSCRIPTION_ALREADY_ACTIVE` |
| Notes | Requires an `Idempotency-Key` |

### `GET /api/v1/billing/checkout/{id}`

Poll a checkout session's status after redirect

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/webhooks/payment/{provider}`

Gateway webhook

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |
| Notes | Signature verified on the raw body before parsing; always acknowledged quickly, processed in a job |

### `GET /api/v1/me/invoices`

Invoice history

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/me/invoices/{id}/pdf`

Signed link to the invoice PDF

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/admin/payments/{id}/refund`

Issue a refund

| | |
|---|---|
| Permission | `billing.refund` |
| Success | 202 |
| Errors | `REFUND_WINDOW_CLOSED`, `ALREADY_REFUNDED` |


### `POST /api/v1/admin/webhooks/{id}/replay`

Replay a stored webhook

| | |
|---|---|
| Permission | `billing.manage` |
| Success | 202 |
| Errors | standard set |


### `GET /api/v1/admin/reconciliation`

Discrepancies between our records and the gateway

| | |
|---|---|
| Permission | `billing.read` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `PAYMENT_FAILED` | 402 | Gateway declined |
| `CHECKOUT_EXPIRED` | 409 | Session expired before completion |
| `ALREADY_REFUNDED` | 409 | Payment already fully refunded |
| `REFUND_WINDOW_CLOSED` | 409 | Outside the refund policy period |
| `WEBHOOK_SIGNATURE_INVALID` | 400 | Signature verification failed |
| `RECONCILIATION_MISMATCH` | 409 | Our record disagrees with the gateway |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
