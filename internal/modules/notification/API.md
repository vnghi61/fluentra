---
module: notification
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: comm
tables: [notifications, notification_preferences, devices, notification_dedupe]
depends_on: [mailer, job, cache, user]
depended_on_by: [auth, writing, speaking, exam, gamification, subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# notification — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `notification`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/notifications` | `self` | Inbox, newest first |
| `GET` | `/api/v1/notifications/unread-count` | `self` | Badge count (cached) |
| `POST` | `/api/v1/notifications/{id}/read` | `self` | Mark one read |
| `POST` | `/api/v1/notifications/read-all` | `self` | Mark all read |
| `GET` | `/api/v1/me/notification-preferences` | `self` | Read preferences |
| `PUT` | `/api/v1/me/notification-preferences` | `self` | Update preferences |
| `POST` | `/api/v1/me/devices` | `self` | Register a push device |
| `DELETE` | `/api/v1/me/devices/{id}` | `self` | Unregister |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/notifications`

Inbox, newest first

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/notifications/unread-count`

Badge count (cached)

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/notifications/{id}/read`

Mark one read

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |

### `POST /api/v1/notifications/read-all`

Mark all read

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | standard set |

### `GET /api/v1/me/notification-preferences`

Read preferences

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `PUT /api/v1/me/notification-preferences`

Update preferences

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/devices`

Register a push device

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | standard set |

### `DELETE /api/v1/me/devices/{id}`

Unregister

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
| `DEVICE_LIMIT_REACHED` | 409 | Too many registered push devices |
| `INVALID_QUIET_HOURS` | 422 | Window is malformed or spans more than 12 hours |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
