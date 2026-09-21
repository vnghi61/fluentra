---
module: content
tier: learning
group: modules
status: DONE
phase: 2
owner: "@learning-team"
schema: content
tables: [content_items, content_versions, media_assets, taxonomies, content_tags, content_reviews, item_reports, tts_cache, taxonomy_prerequisites]
depends_on: [storage, search, audit, ai, media]
depended_on_by: [lesson, learning, vocabulary, grammar, reading, listening, speaking, writing, questionbank]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# content — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `content`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/content/{slug}` | `content.read.published` | Fetch a published content version |
| `GET` | `/api/v1/content` | `content.read.published` | Browse published content with taxonomy filters |
| `POST` | `/api/v1/content/versions/{id}/reports` | `self` | Report an issue with a content version |
| `POST` | `/api/v1/admin/content` | `content.create` | Create a draft item |
| `PUT` | `/api/v1/admin/content/{id}/draft` | `content.edit` | Update the working draft |
| `POST` | `/api/v1/admin/content/{id}/submit` | `content.edit` | Submit for review |
| `POST` | `/api/v1/admin/content/{id}/review` | `content.review` | Approve or request changes |
| `POST` | `/api/v1/admin/content/{id}/publish` | `content.publish` | Publish the approved version |
| `POST` | `/api/v1/admin/content/{id}/archive` | `content.publish` | Archive |
| `GET` | `/api/v1/foundation/topics` | `content.read.published` | Browse foundation topics with taxonomy filters |
| `GET` | `/api/v1/foundation/topics/{code}` | `content.read.published` | Get a foundation topic by code |
| `GET` | `/api/v1/foundation/path` | `content.read.published` | Get topologically sorted foundation learning path |
| `POST` | `/api/v1/admin/foundation/topics` | `content.create` | Create a new foundation taxonomy topic |
| `PATCH` | `/api/v1/admin/foundation/topics/{code}` | `content.edit` | Update foundation topic metadata |
| `PUT` | `/api/v1/admin/foundation/topics/{code}/prerequisites` | `content.edit` | Replace prerequisites for a foundation topic |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/content/{slug}`

Fetch a published content version

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | `CONTENT_NOT_PUBLISHED` |

### `GET /api/v1/content`

Browse published content with taxonomy filters

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/content/versions/{id}/reports`

Report an issue with a content version

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | `VALIDATION_FAILED` |

### `POST /api/v1/admin/content`

Create a draft item

| | |
|---|---|
| Permission | `content.create` |
| Success | 201 |
| Errors | standard set |

### `PUT /api/v1/admin/content/{id}/draft`

Update the working draft

| | |
|---|---|
| Permission | `content.edit` |
| Success | 200 |
| Errors | `INVALID_STATE_TRANSITION` |

### `POST /api/v1/admin/content/{id}/submit`

Submit for review

| | |
|---|---|
| Permission | `content.edit` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/content/{id}/review`

Approve or request changes

| | |
|---|---|
| Permission | `content.review` |
| Success | 200 |
| Errors | `SELF_APPROVAL_FORBIDDEN` |

### `POST /api/v1/admin/content/{id}/publish`

Publish the approved version

| | |
|---|---|
| Permission | `content.publish` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/content/{id}/archive`

Archive

| | |
|---|---|
| Permission | `content.publish` |
| Success | 200 |
| Errors | `CONTENT_IN_USE` |

### `GET /api/v1/foundation/topics`

Browse foundation topics with taxonomy filters

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/foundation/topics/{code}`

Get a foundation topic by code

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | `NOT_FOUND` |

### `GET /api/v1/foundation/path`

Get topologically sorted foundation learning path

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/foundation/topics`

Create a new foundation taxonomy topic

| | |
|---|---|
| Permission | `content.create` |
| Success | 201 |
| Errors | `TAXONOMY_CODE_IMMUTABLE`, `VALIDATION_FAILED` |

### `PATCH /api/v1/admin/foundation/topics/{code}`

Update foundation topic metadata

| | |
|---|---|
| Permission | `content.edit` |
| Success | 200 |
| Errors | `TAXONOMY_CODE_IMMUTABLE`, `NOT_FOUND` |

### `PUT /api/v1/admin/foundation/topics/{code}/prerequisites`

Replace prerequisites for a foundation topic

| | |
|---|---|
| Permission | `content.edit` |
| Success | 200 |
| Errors | `TAXONOMY_CYCLE`, `CROSS_NAMESPACE_PREREQUISITE` |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `CONTENT_NOT_PUBLISHED` | 404 | Draft or archived content requested by a learner |
| `INVALID_STATE_TRANSITION` | 409 | e.g. publishing something not approved |
| `SELF_APPROVAL_FORBIDDEN` | 403 | The author tried to review their own version |
| `CONTENT_IN_USE` | 409 | Referenced by published material |
| `MEDIA_NOT_READY` | 409 | Referenced assets are still processing |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
