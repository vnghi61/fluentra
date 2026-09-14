---
module: listening
tier: learning
group: modules
status: DONE
phase: 3
owner: "@learning-team"
schema: skill
tables: [listening_plays]
depends_on: [content, media, questionbank, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# listening — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `listening`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/listening/items/{versionId}/plays` | `self` | Checks the play policy, records the play, and returns a short-lived presigned audio GET URL |
| `GET` | `/api/v1/listening/items/{versionId}/transcript` | `self` | Returns script only for a graded attempt belonging to caller |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/listening/items/{versionId}/plays`

Checks the play policy, records the play, and returns a short-lived presigned audio GET URL

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `PLAY_LIMIT_REACHED` |

### `GET /api/v1/listening/items/{versionId}/transcript`

Returns script only for a graded attempt belonging to caller

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `TRANSCRIPT_LOCKED` |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `PLAY_LIMIT_REACHED` | 403 | No plays remaining for this attempt |
| `TRANSCRIPT_LOCKED` | 403 | Transcript not yet available |
| `AUDIO_NOT_READY` | 409 | Media still processing |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
