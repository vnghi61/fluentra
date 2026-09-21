---
module: resource
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: resource
tables: [resources]
depends_on: [storage, job]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-09-21
---

# resource — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `resource`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `POST` | `/api/v1/me/resources/upload-intent` | `self` | Presigned upload into fluentra-uploads; creates the row at pending |
| `POST` | `/api/v1/me/resources` | `self` | Confirm an uploaded file or submit a URL; queues validation |
| `GET` | `/api/v1/me/resources` | `self` | The caller's resources, newest first, filterable by status and kind |
| `GET` | `/api/v1/me/resources/{id}` | `self` | One resource, with a presigned GET for a validated file |
| `DELETE` | `/api/v1/me/resources/{id}` | `self` | Delete the resource and its stored object |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `POST /api/v1/me/resources/upload-intent`

Presigned upload into fluentra-uploads; creates the row at pending

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `RESOURCE_TYPE_NOT_SUPPORTED`, `RESOURCE_QUOTA_EXCEEDED` |

### `POST /api/v1/me/resources`

Confirm an uploaded file or submit a URL; queues validation

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | `RESOURCE_NOT_FOUND`, `RESOURCE_NOT_UPLOADED`, `RESOURCE_URL_NOT_ALLOWED`, `RESOURCE_QUOTA_EXCEEDED`, `INVALID_STATUS_TRANSITION` |

### `GET /api/v1/me/resources`

The caller's resources, newest first, filterable by status and kind

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/resources/{id}`

One resource, with a presigned GET for a validated file

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `RESOURCE_NOT_FOUND` |

### `DELETE /api/v1/me/resources/{id}`

Delete the resource and its stored object

| | |
|---|---|
| Permission | `self` |
| Success | 204 |
| Errors | `RESOURCE_NOT_FOUND` |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `RESOURCE_NOT_FOUND` | 404 | No such resource for this caller, the same answer another user's id gets |
| `RESOURCE_QUOTA_EXCEEDED` | 429 | The caller is at 50 resources or 250 MB |
| `RESOURCE_TYPE_NOT_SUPPORTED` | 422 | The declared type is not in the allow-list |
| `RESOURCE_URL_NOT_ALLOWED` | 422 | A scheme other than http or https, or a host with a non-public address |
| `RESOURCE_NOT_UPLOADED` | 409 | Confirmation arrived for a key with no object behind it |
| `INVALID_STATUS_TRANSITION` | 400 | Confirmation of a resource that is not pending |
| `INVALID_REQUEST_BODY` | 400 | The body is not valid JSON |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
