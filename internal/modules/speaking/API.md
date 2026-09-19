---
module: speaking
tier: learning
group: modules
status: DONE
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_feedback, speaking_consents]
depends_on: [media, ai, storage, job, content, learning]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# speaking — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `speaking`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/speaking/upload-intent` | `self` | Presigned PUT URL for recording upload to storage |
| `DELETE` | `/api/v1/speaking/attempts/{id}/recording` | `self` | Purges the recording object while keeping scores and feedback |
| `GET` | `/api/v1/speaking/attempts/{id}/feedback` | `self` | Read feedback on a graded speaking attempt, with a presigned audio_url while the recording lives |
| `GET` | `/api/v1/speaking/submissions` | `self` | Paginated history of the learner's graded speaking attempts |
| `GET` | `/api/v1/speaking/consent` | `self` | Whether the learner has consented to voice recording, and when |
| `POST` | `/api/v1/speaking/consent` | `self` | Record the learner's consent to voice recording (BR-SPEAKING-03) |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/speaking/upload-intent`

Presigned PUT URL for recording upload to storage

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `UNSUPPORTED_AUDIO_FORMAT`, `SPEECH_DAILY_LIMIT_REACHED` |

### `DELETE /api/v1/speaking/attempts/{id}/recording`

Purges the recording object while keeping scores and feedback

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |

### `GET /api/v1/speaking/attempts/{id}/feedback`

Read feedback on a graded speaking attempt, with a presigned audio_url while the recording lives

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `FEEDBACK_NOT_FOUND` |

### `GET /api/v1/speaking/submissions`

Paginated history of the learner's graded speaking attempts

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/speaking/consent`

Whether the learner has consented to voice recording, and when

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/speaking/consent`

Record the learner's consent to voice recording (BR-SPEAKING-03)

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
| `RECORDING_CONSENT_REQUIRED` | 403 | Voice consent not yet given |
| `AUDIO_TOO_LONG` | 422 | Exceeds the task maximum |
| `AUDIO_TOO_QUIET` | 422 | Signal level too low to assess |
| `TRANSCRIPTION_LOW_CONFIDENCE` | 422 | Ask the learner to re-record |
| `SCORING_FAILED` | 500 | Assessment pipeline failed after retries |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
