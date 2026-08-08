---
module: ai
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@ai-team"
schema: ai
tables: [ai_requests, ai_usage, prompt_versions, ai_cache_entries, ai_budgets]
depends_on: [cache, telemetry, job]
depended_on_by: [writing, speaking, grammar, questionbank, content, reading, media, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# ai — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `ai`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/ai/usage` | `ai.read` | Spend and volume by task, provider and day |
| `GET` | `/api/v1/admin/ai/requests/{id}` | `ai.read` | Inspect one call for debugging |
| `GET` | `/api/v1/admin/ai/prompts` | `ai.read` | Deployed prompt versions and their status |
| `POST` | `/api/v1/admin/ai/prompts/{task}/activate` | `ai.manage` | Promote a prompt version |
| `GET` | `/api/v1/admin/ai/budget` | `ai.read` | Budget consumption today |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/ai/usage`

Spend and volume by task, provider and day

| | |
|---|---|
| Permission | `ai.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/ai/requests/{id}`

Inspect one call for debugging

| | |
|---|---|
| Permission | `ai.read` |
| Success | 200 |
| Errors | standard set |
| Notes | Bodies are redacted; full content requires `ai.read.sensitive` and is itself audited |

### `GET /api/v1/admin/ai/prompts`

Deployed prompt versions and their status

| | |
|---|---|
| Permission | `ai.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/ai/prompts/{task}/activate`

Promote a prompt version

| | |
|---|---|
| Permission | `ai.manage` |
| Success | 200 |
| Errors | `EVAL_THRESHOLD_NOT_MET` |

### `GET /api/v1/admin/ai/budget`

Budget consumption today

| | |
|---|---|
| Permission | `ai.read` |
| Success | 200 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `AI_QUOTA_EXCEEDED` | 429 | Per-user daily limit reached |
| `AI_BUDGET_EXCEEDED` | 503 | Global daily spend cap reached; non-critical tasks are shed first |
| `AI_UNAVAILABLE` | 503 | Every provider in the chain failed |
| `AI_TIMEOUT` | 504 | Provider exceeded the task timeout |
| `AI_OUTPUT_INVALID` | 500 | Output failed schema validation after the repair attempt |
| `AI_CONTENT_FLAGGED` | 422 | Moderation blocked the input |
| `AI_OPTED_OUT` | 403 | The user disabled AI processing |
| `EVAL_THRESHOLD_NOT_MET` | 409 | Prompt promotion blocked by its eval suite |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
