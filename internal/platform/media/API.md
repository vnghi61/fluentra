---
module: media
tier: platform
group: platform
status: IMPLEMENTED
phase: 3
owner: "@platform-team"
schema: content
tables: [tts_cache]
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
_None yet._
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
_This module exposes no HTTP endpoints. It is consumed through its `contract` package._
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
