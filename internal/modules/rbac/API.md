---
module: rbac
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [roles, permissions, role_permissions, user_roles]
depends_on: [cache, audit]
depended_on_by: [auth, admin, content, questionbank, exam, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# rbac — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `rbac`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/me/permissions` | `self` | The caller's effective permissions — the frontend uses this to hide unavailable actions |
| `GET` | `/api/v1/admin/roles` | `rbac.read` | List roles and their permissions |
| `POST` | `/api/v1/admin/users/{id}/roles` | `rbac.assign` | Grant a role |
| `DELETE` | `/api/v1/admin/users/{id}/roles/{role}` | `rbac.assign` | Revoke a role |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/me/permissions`

The caller's effective permissions — the frontend uses this to hide unavailable actions

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |
| Notes | Hiding a control is UX, not security; the server still enforces every call |

### `GET /api/v1/admin/roles`

List roles and their permissions

| | |
|---|---|
| Permission | `rbac.read` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/admin/users/{id}/roles`

Grant a role

| | |
|---|---|
| Permission | `rbac.assign` |
| Success | 200 |
| Errors | `PERMISSION_DENIED`, `SELF_ELEVATION_FORBIDDEN` |


### `DELETE /api/v1/admin/users/{id}/roles/{role}`

Revoke a role

| | |
|---|---|
| Permission | `rbac.assign` |
| Success | 204 |
| Errors | `LAST_ADMIN_PROTECTED` |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `PERMISSION_DENIED` | 403 | Authenticated but lacking the required permission |
| `SELF_ELEVATION_FORBIDDEN` | 403 | An actor tried to grant themselves a role |
| `LAST_ADMIN_PROTECTED` | 409 | Would leave the system with no administrator |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
