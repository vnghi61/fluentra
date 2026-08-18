---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [admin_actions, feature_flags, admin_notes, moderation_items]
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
| `GET` | `/api/v1/admin/users` | `user.list` | Search accounts, cursor-paginated |
| `GET` | `/api/v1/admin/users/{id}` | `user.read` | One account in full |
| `POST` | `/api/v1/admin/users/{id}/suspend` | `user.suspend` | Suspend an account and end its sessions |
| `POST` | `/api/v1/admin/users/{id}/reinstate` | `user.reinstate` | Return a suspended account to active |
| `POST` | `/api/v1/admin/users/{id}/sessions/revoke` | `user.manage_sessions` | Sign a user out everywhere |
| `GET` | `/api/v1/admin/flags` | `system.flags` | List every feature flag |
| `POST` | `/api/v1/admin/flags` | `system.flags` | Create a feature flag |
| `PUT` | `/api/v1/admin/flags/{key}` | `system.flags` | Update a feature flag |
| `DELETE` | `/api/v1/admin/flags/{key}` | `system.flags` | Delete a feature flag |
| `GET` | `/api/v1/admin/dashboard` | `admin.dashboard` | Composed KPI summary |
| `GET` | `/api/v1/admin/moderation` | `moderation.read` | Moderation queue |
| `POST` | `/api/v1/admin/moderation/{id}/resolve` | `moderation.act` | Resolve a queue item |
| `POST` | `/api/v1/admin/jobs/{id}/retry` | `system.jobs` | Retry a failed job |
| `POST` | `/api/v1/admin/impersonate/{user_id}` | `user.impersonate` | Start a time-boxed impersonation session |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/users`

Search accounts, cursor-paginated

| | |
|---|---|
| Permission | `user.list` |
| Success | 200 |
| Errors | standard set |
| Notes | The search itself is not audited; opening one result is |

### `GET /api/v1/admin/users/{id}`

One account in full

| | |
|---|---|
| Permission | `user.read` |
| Success | 200 |
| Errors | standard set |
| Notes | Recorded as `admin.user_viewed` |

### `POST /api/v1/admin/users/{id}/suspend`

Suspend an account and end its sessions

| | |
|---|---|
| Permission | `user.suspend` |
| Success | 200 |
| Errors | standard set |
| Notes | Reason required; refused on the caller's own account |

### `POST /api/v1/admin/users/{id}/reinstate`

Return a suspended account to active

| | |
|---|---|
| Permission | `user.reinstate` |
| Success | 200 |
| Errors | standard set |
| Notes | Reason required; sessions are not restored |

### `POST /api/v1/admin/users/{id}/sessions/revoke`

Sign a user out everywhere

| | |
|---|---|
| Permission | `user.manage_sessions` |
| Success | 200 |
| Errors | standard set |
| Notes | Reason required; status is unchanged |

### `GET /api/v1/admin/flags`

List every feature flag

| | |
|---|---|
| Permission | `system.flags` |
| Success | 200 |
| Errors | standard set |
| Notes | Unpaginated — the set is small by design |

### `POST /api/v1/admin/flags`

Create a feature flag

| | |
|---|---|
| Permission | `system.flags` |
| Success | 201 |
| Errors | standard set |
| Notes | `owner` and a future `expires_on` are required |

### `PUT /api/v1/admin/flags/{key}`

Update a feature flag

| | |
|---|---|
| Permission | `system.flags` |
| Success | 200 |
| Errors | standard set |
| Notes | Visible within one 30 s cache generation |

### `DELETE /api/v1/admin/flags/{key}`

Delete a feature flag

| | |
|---|---|
| Permission | `system.flags` |
| Success | 204 |
| Errors | standard set |
| Notes | Evaluations of the key then return false |

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
| `REASON_REQUIRED` | 422 | A state-changing admin action was sent without a reason of at least 10 characters |
| `USER_NOT_FOUND` | 404 | The target user account does not exist |
| `FEATURE_FLAG_NOT_FOUND` | 404 | The requested feature flag does not exist |
| `FEATURE_FLAG_ALREADY_EXISTS` | 409 | A feature flag with that key already exists |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
