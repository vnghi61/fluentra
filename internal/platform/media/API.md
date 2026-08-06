---
module: media
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@platform-team"
schema: content
tables: [media_derivatives, transcripts, tts_cache]
depends_on: [storage, job, ai, telemetry]
depended_on_by: [speaking, listening, content, vocabulary, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# media — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `media`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/media/{asset_id}/derivatives` | `content.read` | Inspect the pipeline output for one asset |
| `POST` | `/api/v1/admin/media/{asset_id}/reprocess` | `content.manage` | Re-run the pipeline |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/media/{asset_id}/derivatives`

Inspect the pipeline output for one asset

| | |
|---|---|
| Permission | `content.read` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/admin/media/{asset_id}/reprocess`

Re-run the pipeline

| | |
|---|---|
| Permission | `content.manage` |
| Success | 202 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `UNSUPPORTED_AUDIO_FORMAT` | 415 | Magic bytes do not match a supported container |
| `AUDIO_TOO_LONG` | 422 | Exceeds the configured maximum duration |
| `MEDIA_PROCESSING_FAILED` | 500 | A pipeline stage failed after retries |
| `TRANSCRIPTION_LOW_CONFIDENCE` | 422 | Audio too noisy or too quiet to score fairly |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
