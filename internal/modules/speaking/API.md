---
module: speaking
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_tasks, speaking_attempts, pronunciation_scores, speaking_feedback]
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
| `GET` | `/api/v1/speaking/tasks` | `content.read.published` | Available tasks |
| `POST` | `/api/v1/speaking/upload-intent` | `self` | Presigned URL for the recording |
| `POST` | `/api/v1/speaking/attempts` | `self` | Create the attempt after upload |
| `GET` | `/api/v1/speaking/attempts/{id}` | `self` | Attempt with scores and feedback when ready |
| `GET` | `/api/v1/speaking/attempts` | `self` | History with score progression |
| `DELETE` | `/api/v1/speaking/attempts/{id}/recording` | `self` | Delete the audio while keeping the scores |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/speaking/tasks`

Available tasks

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/speaking/upload-intent`

Presigned URL for the recording

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `UNSUPPORTED_AUDIO_FORMAT`, `AUDIO_TOO_LONG` |


### `POST /api/v1/speaking/attempts`

Create the attempt after upload

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | `UPLOAD_VERIFICATION_FAILED`, `AI_QUOTA_EXCEEDED` |


### `GET /api/v1/speaking/attempts/{id}`

Attempt with scores and feedback when ready

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/speaking/attempts`

History with score progression

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `DELETE /api/v1/speaking/attempts/{id}/recording`

Delete the audio while keeping the scores

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
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
