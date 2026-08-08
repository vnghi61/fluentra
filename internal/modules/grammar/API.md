---
module: grammar
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [grammar_points, grammar_rules, grammar_exercises, error_tags, user_grammar_state]
depends_on: [content, srs, ai, learning]
depended_on_by: [writing, speaking, learning, exam]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# grammar — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `grammar`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/grammar/points` | `content.read.published` | Browse the taxonomy by level |
| `GET` | `/api/v1/grammar/points/{code}` | `content.read.published` | Rule with examples and common errors |
| `GET` | `/api/v1/me/grammar/weaknesses` | `self` | Ranked weak points with drill suggestions |
| `POST` | `/api/v1/grammar/explain` | `self` | Explain a tagged error, citing a rule |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/grammar/points`

Browse the taxonomy by level

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/grammar/points/{code}`

Rule with examples and common errors

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/grammar/weaknesses`

Ranked weak points with drill suggestions

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/grammar/explain`

Explain a tagged error, citing a rule

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `AI_QUOTA_EXCEEDED` |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `GRAMMAR_POINT_NOT_FOUND` | 404 | Unknown taxonomy code |
| `EXPLANATION_UNGROUNDED` | 500 | The model could not cite a valid rule |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
