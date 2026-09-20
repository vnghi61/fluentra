---
module: studio
tier: commerce
group: modules
status: ACTIVE
phase: 3
owner: "@commerce-team"
schema: studio
tables: [creator_profiles, payout_accounts, course_drafts, submissions, listings, purchases, creator_ledger]
depends_on: [content, lesson, learning, payment, job]
depended_on_by: [admin, lesson, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# studio — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `studio`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/studio/creator/profile` | `self` | Get current creator profile |
| `POST` | `/api/v1/studio/creator/profile` | `self` | Create or update creator profile |
| `GET` | `/api/v1/studio/creator/payout-account` | `self` | Get payout account info |
| `POST` | `/api/v1/studio/creator/payout-account` | `self` | Set payout account |
| `GET` | `/api/v1/studio/courses` | `self` | List creator course drafts |
| `POST` | `/api/v1/studio/courses` | `self` | Create course draft |
| `GET` | `/api/v1/studio/courses/{id}` | `self` | Get course draft |
| `PUT` | `/api/v1/studio/courses/{id}` | `self` | Update course draft |
| `POST` | `/api/v1/studio/courses/{id}/submit` | `self` | Submit draft for verification |
| `POST` | `/api/v1/courses/{id}/claim` | `self` | Claim access to a free community course |
| `POST` | `/api/v1/courses/{id}/purchase` | `self` | Initiate purchase of a paid course via VietQR |
| `GET` | `/api/v1/me/purchases` | `self` | List courses purchased or claimed by learner |
| `POST` | `/api/v1/me/purchases/{id}/refund` | `self` | Self-service refund for course purchase |
| `GET` | `/api/v1/me/studio/earnings` | `self` | What this creator has earned, been paid, and is still owed |
| `POST` | `/api/v1/me/studio/payouts` | `self` | Request a payout of the creator balance |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/studio/creator/profile`

Get current creator profile

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/studio/creator/profile`

Create or update creator profile

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/studio/creator/payout-account`

Get payout account info

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/studio/creator/payout-account`

Set payout account

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/studio/courses`

List creator course drafts

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/studio/courses`

Create course draft

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | standard set |

### `GET /api/v1/studio/courses/{id}`

Get course draft

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `PUT /api/v1/studio/courses/{id}`

Update course draft

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/studio/courses/{id}/submit`

Submit draft for verification

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/courses/{id}/claim`

Claim access to a free community course

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/courses/{id}/purchase`

Initiate purchase of a paid course via VietQR

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/purchases`

List courses purchased or claimed by learner

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/purchases/{id}/refund`

Self-service refund for course purchase

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/studio/earnings`

What this creator has earned, been paid, and is still owed

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/studio/payouts`

Request a payout of the creator balance

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `DRAFT_NOT_FOUND` | 404 | Course draft not found |
| `SUBMISSION_NOT_FOUND` | 404 | Submission not found |
| `CANNOT_REVIEW_OWN_SUBMISSION` | 403 | Reviewer submitted this course |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
