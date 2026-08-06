---
module: storage
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry, job]
depended_on_by: [user, content, media, speaking, writing, analytics, audit]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# storage — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `storage`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/storage/stats` | `system.storage` | Object counts and sizes per bucket |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/storage/stats`

Object counts and sizes per bucket

| | |
|---|---|
| Permission | `system.storage` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `UPLOAD_VERIFICATION_FAILED` | 422 | The uploaded object does not match the intent |
| `STORAGE_UNAVAILABLE` | 503 | Object store unreachable |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
