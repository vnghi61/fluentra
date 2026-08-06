---
module: user
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [users, profiles, user_preferences, learning_profiles, user_deletion_requests, user_exports]
depends_on: [storage, mailer, audit]
depended_on_by: [auth, admin, learning, notification, subscription, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# user — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `user`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/me` | `self` | Full profile of the caller |
| `PATCH` | `/api/v1/me` | `self` | Update profile fields |
| `GET` | `/api/v1/me/preferences` | `self` | Read preferences |
| `PUT` | `/api/v1/me/preferences` | `self` | Replace preferences |
| `POST` | `/api/v1/me/avatar/upload-intent` | `self` | Get a presigned URL for an avatar upload |
| `PUT` | `/api/v1/me/avatar` | `self` | Confirm the uploaded avatar |
| `POST` | `/api/v1/me/export` | `self` | Request a data export |
| `DELETE` | `/api/v1/me` | `self` | Request account deletion (30-day grace) |
| `POST` | `/api/v1/me/deletion/cancel` | `self` | Cancel a pending deletion |
| `GET` | `/api/v1/admin/users` | `user.list` | Search and list users |
| `GET` | `/api/v1/admin/users/{id}` | `user.read` | Read one user |
| `POST` | `/api/v1/admin/users/{id}/suspend` | `user.suspend` | Suspend an account |
| `POST` | `/api/v1/admin/users/{id}/reinstate` | `user.suspend` | Reinstate a suspended account |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/me`

Full profile of the caller

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `PATCH /api/v1/me`

Update profile fields

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `VALIDATION_FAILED` |


### `GET /api/v1/me/preferences`

Read preferences

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `PUT /api/v1/me/preferences`

Replace preferences

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/me/avatar/upload-intent`

Get a presigned URL for an avatar upload

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `UNSUPPORTED_MEDIA_TYPE`, `TOO_LARGE` |


### `PUT /api/v1/me/avatar`

Confirm the uploaded avatar

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/me/export`

Request a data export

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | standard set |


### `DELETE /api/v1/me`

Request account deletion (30-day grace)

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | standard set |


### `POST /api/v1/me/deletion/cancel`

Cancel a pending deletion

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `INVALID_STATE_TRANSITION` |


### `GET /api/v1/admin/users`

Search and list users

| | |
|---|---|
| Permission | `user.list` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/admin/users/{id}`

Read one user

| | |
|---|---|
| Permission | `user.read` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/admin/users/{id}/suspend`

Suspend an account

| | |
|---|---|
| Permission | `user.suspend` |
| Success | 200 |
| Errors | `INVALID_STATE_TRANSITION` |


### `POST /api/v1/admin/users/{id}/reinstate`

Reinstate a suspended account

| | |
|---|---|
| Permission | `user.suspend` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `DISPLAY_NAME_NOT_ALLOWED` | 422 | Reserved or moderated name |
| `INVALID_STATE_TRANSITION` | 409 | e.g. cancelling a deletion that already executed |
| `EXPORT_ALREADY_PENDING` | 409 | One export at a time |
| `AGE_RESTRICTED` | 403 | Under-16 account without guardian consent |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
