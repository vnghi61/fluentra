---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [feature_flags, admin_notes, moderation_items]
depends_on: [user, rbac, audit, auth, content, cache]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-08-06
---

# admin — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `admin`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/dashboard` | `admin.dashboard` | Composed KPI summary |
| `GET` | `/api/v1/admin/moderation` | `moderation.read` | Moderation queue |
| `POST` | `/api/v1/admin/moderation/{id}/resolve` | `moderation.act` | Resolve a queue item |
| `GET` | `/api/v1/admin/feature-flags` | `system.flags` | List flags |
| `PUT` | `/api/v1/admin/feature-flags/{key}` | `system.flags` | Update a flag |
| `POST` | `/api/v1/admin/jobs/{id}/retry` | `system.jobs` | Retry a failed job |
| `POST` | `/api/v1/admin/impersonate/{user_id}` | `user.impersonate` | Start a time-boxed impersonation session |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/dashboard`

Composed KPI summary

| | |
|---|---|
| Permission | `admin.dashboard` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/moderation`

Moderation queue

| | |
|---|---|
| Permission | `moderation.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/moderation/{id}/resolve`

Resolve a queue item

| | |
|---|---|
| Permission | `moderation.act` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/feature-flags`

List flags

| | |
|---|---|
| Permission | `system.flags` |
| Success | 200 |
| Errors | standard set |

### `PUT /api/v1/admin/feature-flags/{key}`

Update a flag

| | |
|---|---|
| Permission | `system.flags` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/jobs/{id}/retry`

Retry a failed job

| | |
|---|---|
| Permission | `system.jobs` |
| Success | 202 |
| Errors | standard set |

### `POST /api/v1/admin/impersonate/{user_id}`

Start a time-boxed impersonation session

| | |
|---|---|
| Permission | `user.impersonate` |
| Success | 200 |
| Errors | standard set |
| Notes | Audited; blocked for payment actions; a banner is shown throughout |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `IMPERSONATION_FORBIDDEN_ACTION` | 403 | The attempted action is not permitted while impersonating |
| `SELF_ADMIN_ACTION_FORBIDDEN` | 403 | An admin may not administer their own account |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
