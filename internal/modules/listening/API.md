---
module: listening
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [audio_items, transcripts, listening_attempts]
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
| `GET` | `/api/v1/listening/items/{id}` | `content.read.published` | Item with a presigned audio URL and the play policy |
| `POST` | `/api/v1/listening/attempts` | `self` | Start an attempt |
| `POST` | `/api/v1/listening/attempts/{id}/play` | `self` | Record a play (server-side counter) |
| `POST` | `/api/v1/listening/attempts/{id}/submit` | `self` | Submit answers |
| `GET` | `/api/v1/listening/attempts/{id}/transcript` | `self` | Transcript after submission |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/listening/items/{id}`

Item with a presigned audio URL and the play policy

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/listening/attempts`

Start an attempt

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | standard set |

### `POST /api/v1/listening/attempts/{id}/play`

Record a play (server-side counter)

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `PLAY_LIMIT_REACHED` |

### `POST /api/v1/listening/attempts/{id}/submit`

Submit answers

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/listening/attempts/{id}/transcript`

Transcript after submission

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
