---
module: cache
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry]
depended_on_by: [auth, rbac, user, content, lesson, learning, srs, gamification, ai, notification, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# cache — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `cache`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `DELETE` | `/api/v1/admin/cache/{pattern}` | `system.cache` | Operational invalidation by key prefix |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `DELETE /api/v1/admin/cache/{pattern}`

Operational invalidation by key prefix

| | |
|---|---|
| Permission | `system.cache` |
| Success | 204 |
| Errors | standard set |
| Notes | Prefix-scoped and audited; never a bare `KEYS *` |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `RATE_LIMITED` | 429 | Rate limiter rejected the request |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
