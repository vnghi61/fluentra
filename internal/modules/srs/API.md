---
module: srs
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [review_cards, review_logs, srs_params, review_daily_stats]
depends_on: [cache, job, content]
depended_on_by: [learning, vocabulary, grammar, gamification, notification, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# srs — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `srs`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/reviews/session` | `self` | Build a review session from due cards |
| `GET` | `/api/v1/reviews/due-count` | `self` | Badge count |
| `POST` | `/api/v1/reviews/{card_id}/answer` | `self` | Record a grade and reschedule |
| `POST` | `/api/v1/reviews/session/complete` | `self` | Close the session |
| `POST` | `/api/v1/reviews/{card_id}/suspend` | `self` | Stop scheduling this card |
| `POST` | `/api/v1/reviews/{card_id}/reset` | `self` | Treat as new again |
| `GET` | `/api/v1/reviews/forecast` | `self` | Projected workload for the next 30 days |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/reviews/session`

Build a review session from due cards

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/reviews/due-count`

Badge count

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/reviews/{card_id}/answer`

Record a grade and reschedule

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `REVIEW_CARD_SUSPENDED`, `REVIEW_NOT_DUE` |


### `POST /api/v1/reviews/session/complete`

Close the session

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/reviews/{card_id}/suspend`

Stop scheduling this card

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/reviews/{card_id}/reset`

Treat as new again

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/reviews/forecast`

Projected workload for the next 30 days

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `REVIEW_CARD_SUSPENDED` | 409 | Card is suspended |
| `REVIEW_NOT_DUE` | 409 | Answered a card outside the session, before it was due |
| `DAILY_LIMIT_REACHED` | 409 | New-card limit reached for today |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
