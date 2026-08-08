---
module: questionbank
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_options, question_sets, question_set_items, question_stats]
depends_on: [content, ai, audit, search]
depended_on_by: [exam, reading, listening, grammar, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `questionbank`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/questions` | `questionbank.read` | Search and filter items |
| `POST` | `/api/v1/admin/questions` | `questionbank.create` | Create an item |
| `POST` | `/api/v1/admin/questions/{id}/review` | `questionbank.review` | Approve or reject |
| `POST` | `/api/v1/admin/questions/generate` | `questionbank.create` | AI-generate draft items for review |
| `GET` | `/api/v1/admin/questions/{id}/stats` | `questionbank.read` | Empirical difficulty and discrimination |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/questions`

Search and filter items

| | |
|---|---|
| Permission | `questionbank.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/questions`

Create an item

| | |
|---|---|
| Permission | `questionbank.create` |
| Success | 201 |
| Errors | standard set |

### `POST /api/v1/admin/questions/{id}/review`

Approve or reject

| | |
|---|---|
| Permission | `questionbank.review` |
| Success | 200 |
| Errors | `SELF_APPROVAL_FORBIDDEN` |

### `POST /api/v1/admin/questions/generate`

AI-generate draft items for review

| | |
|---|---|
| Permission | `questionbank.create` |
| Success | 202 |
| Errors | standard set |

### `GET /api/v1/admin/questions/{id}/stats`

Empirical difficulty and discrimination

| | |
|---|---|
| Permission | `questionbank.read` |
| Success | 200 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `SELF_APPROVAL_FORBIDDEN` | 403 | Author reviewing their own item |
| `INSUFFICIENT_ITEMS` | 409 | Not enough approved items matching the sampling criteria |
| `ITEM_IN_USE` | 409 | Cannot archive an item used by a published exam |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
