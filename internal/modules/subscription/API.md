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

# subscription — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `subscription`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/billing/plans` | `public` | Plan catalogue with prices |
| `GET` | `/api/v1/me/subscription` | `self` | Current subscription and entitlements |
| `POST` | `/api/v1/me/subscription` | `self` | Start a subscription or trial (delegates checkout to `payment`) |
| `POST` | `/api/v1/me/subscription/change` | `self` | Upgrade or downgrade |
| `POST` | `/api/v1/me/subscription/cancel` | `self` | Cancel at period end |
| `POST` | `/api/v1/me/subscription/reactivate` | `self` | Undo a pending cancellation |
| `GET` | `/api/v1/admin/subscriptions` | `billing.read` | Search subscriptions |
| `POST` | `/api/v1/admin/subscriptions/{id}/grant` | `billing.manage` | Grant a complimentary period |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/billing/plans`

Plan catalogue with prices

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/subscription`

Current subscription and entitlements

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/subscription`

Start a subscription or trial (delegates checkout to `payment`)

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | `SUBSCRIPTION_ALREADY_ACTIVE`, `PLAN_NOT_AVAILABLE` |

### `POST /api/v1/me/subscription/change`

Upgrade or downgrade

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `INVALID_STATE_TRANSITION` |

### `POST /api/v1/me/subscription/cancel`

Cancel at period end

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/subscription/reactivate`

Undo a pending cancellation

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/subscriptions`

Search subscriptions

| | |
|---|---|
| Permission | `billing.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/subscriptions/{id}/grant`

Grant a complimentary period

| | |
|---|---|
| Permission | `billing.manage` |
| Success | 200 |
| Errors | standard set |
| Notes | Audited with a required reason |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `SUBSCRIPTION_ALREADY_ACTIVE` | 409 | Duplicate subscribe |
| `PLAN_NOT_AVAILABLE` | 409 | Plan withdrawn or region-restricted |
| `TRIAL_ALREADY_USED` | 409 | One trial per learner |
| `ENTITLEMENT_REQUIRED` | 403 | The feature requires a higher plan |
| `INVALID_STATE_TRANSITION` | 409 | e.g. reactivating an expired subscription |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
