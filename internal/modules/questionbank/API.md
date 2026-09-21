---
module: questionbank
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_stats]
depends_on: [content, lesson, learning, rbac]
depended_on_by: [exam]
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
| `GET` | `/api/v1/admin/questions` | `questionbank.read` | Filter by exam part, kind, CEFR, spine node, status. Reachable by moderators |
| `POST` | `/api/v1/admin/questions/generate` | `questionbank.create` | Generate draft questions for a part and nodes; they wait in the review queue |
| `GET` | `/api/v1/admin/questions/{id}/stats` | `questionbank.read` | Empirical difficulty and discrimination. Reachable by moderators |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/questions`

Filter by exam part, kind, CEFR, spine node, status. Reachable by moderators

| | |
|---|---|
| Permission | `questionbank.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/questions/generate`

Generate draft questions for a part and nodes; they wait in the review queue

| | |
|---|---|
| Permission | `questionbank.create` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/questions/{id}/stats`

Empirical difficulty and discrimination. Reachable by moderators

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
| `QUESTION_NOT_FOUND` | 404 | No such question |
| `QUESTION_NOT_REVIEWED` | 409 | Its content version is not published yet; approve it in the review queue |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
