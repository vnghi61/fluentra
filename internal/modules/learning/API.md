---
module: learning
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [enrollments, progress, attempts, learning_sessions, placement_results, skill_mastery]
depends_on: [lesson, content, srs, cache, job]
depended_on_by: [gamification, analytics, admin, exam, vocabulary, grammar, reading, listening, speaking, writing]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# learning — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `learning`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/courses/{id}/enroll` | `self` | Enrol |
| `GET` | `/api/v1/me/dashboard` | `self` | Today's plan, due reviews, continue-where-you-left-off |
| `GET` | `/api/v1/me/progress` | `self` | Progress across courses and skills |
| `POST` | `/api/v1/activities/{id}/attempts` | `self` | Start an attempt |
| `POST` | `/api/v1/attempts/{id}/submit` | `self` | Submit a response for grading |
| `GET` | `/api/v1/attempts/{id}` | `self` | Attempt state and result |
| `POST` | `/api/v1/me/sessions` | `self` | Start a study session |
| `POST` | `/api/v1/me/sessions/{id}/complete` | `self` | End a session |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/courses/{id}/enroll`

Enrol

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | `ALREADY_ENROLLED` |

### `GET /api/v1/me/dashboard`

Today's plan, due reviews, continue-where-you-left-off

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/progress`

Progress across courses and skills

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/activities/{id}/attempts`

Start an attempt

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | `LESSON_LOCKED`, `ACTIVITY_ALREADY_COMPLETED` |

### `POST /api/v1/attempts/{id}/submit`

Submit a response for grading

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `ATTEMPT_EXPIRED`, `VALIDATION_FAILED` |
| Notes | Requires an `Idempotency-Key`. Returns 202 when the grader is asynchronous. |

### `GET /api/v1/attempts/{id}`

Attempt state and result

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/sessions`

Start a study session

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | standard set |

### `POST /api/v1/me/sessions/{id}/complete`

End a session

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
| `ALREADY_ENROLLED` | 409 | Duplicate enrolment |
| `LESSON_LOCKED` | 403 | Prerequisites not met |
| `ACTIVITY_ALREADY_COMPLETED` | 409 | Re-submission not allowed for this activity kind |
| `ATTEMPT_EXPIRED` | 409 | Time limit exceeded |
| `ATTEMPT_NOT_FOUND` | 404 | No such attempt |
| `FORBIDDEN` | 403 | The attempt belongs to another learner — an idempotency key is not an authorisation token |
| `ALREADY_GRADED` | 409 | The attempt has left `in_progress` and a fresh submission cannot claim it |
| `IDEMPOTENCY_CONFLICT` | 409 | A different `Idempotency-Key` was presented for an attempt already claimed — a conflict, not a retry |
| `INVALID_IDEMPOTENCY_KEY` | 422 | `Idempotency-Key` header missing or not a UUID |
| `UNSUPPORTED_ACTIVITY_KIND` | 422 | No grader is registered for the activity kind. The kinds a deployment declares are validated at boot, so reaching this means the data asked for a kind that was never declared — one request fails, not the process |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
