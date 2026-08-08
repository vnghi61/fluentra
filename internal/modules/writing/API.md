---
module: writing
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [writing_tasks, writing_drafts, writing_submissions, writing_feedback, writing_revisions]
depends_on: [ai, job, content, learning, notification]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# writing — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `writing`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/writing/tasks` | `content.read.published` | Available tasks by level and type |
| `PUT` | `/api/v1/writing/tasks/{id}/draft` | `self` | Autosave a draft |
| `POST` | `/api/v1/writing/submissions` | `self` | Submit for grading |
| `GET` | `/api/v1/writing/submissions/{id}` | `self` | Submission with feedback when ready |
| `GET` | `/api/v1/writing/submissions/{id}/stream` | `self` | SSE stream of grading progress and partial feedback |
| `GET` | `/api/v1/writing/submissions` | `self` | History with band progression |
| `POST` | `/api/v1/writing/submissions/{id}/dispute` | `self` | Flag a grade for human review |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/writing/tasks`

Available tasks by level and type

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `PUT /api/v1/writing/tasks/{id}/draft`

Autosave a draft

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/writing/submissions`

Submit for grading

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | `SUBMISSION_TOO_SHORT`, `SUBMISSION_TOO_LONG`, `AI_QUOTA_EXCEEDED`, `AI_OPTED_OUT` |
| Notes | Requires an `Idempotency-Key`; returns a stream URL |

### `GET /api/v1/writing/submissions/{id}`

Submission with feedback when ready

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/writing/submissions/{id}/stream`

SSE stream of grading progress and partial feedback

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/writing/submissions`

History with band progression

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/writing/submissions/{id}/dispute`

Flag a grade for human review

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `SUBMISSION_TOO_SHORT` | 422 | Below the task's minimum word count |
| `SUBMISSION_TOO_LONG` | 422 | Above the maximum |
| `SUBMISSION_ALREADY_GRADED` | 409 | Immutable once graded |
| `GRADING_FAILED` | 500 | All grading attempts failed |
| `DISPUTE_ALREADY_OPEN` | 409 | One open dispute per submission |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
