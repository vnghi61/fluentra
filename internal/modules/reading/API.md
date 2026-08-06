---
module: reading
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [passages, passage_questions, reading_attempts]
depends_on: [content, questionbank, vocabulary, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# reading — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `reading`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/reading/passages/{id}` | `content.read.published` | Passage with glossing hints |
| `POST` | `/api/v1/reading/attempts` | `self` | Start a timed reading attempt |
| `POST` | `/api/v1/reading/attempts/{id}/submit` | `self` | Submit answers |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/reading/passages/{id}`

Passage with glossing hints

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/reading/attempts`

Start a timed reading attempt

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | standard set |


### `POST /api/v1/reading/attempts/{id}/submit`

Submit answers

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `ATTEMPT_EXPIRED` |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `PASSAGE_NOT_FOUND` | 404 | Unknown or unpublished passage |
| `QUESTIONS_LOCKED` | 403 | Questions requested before reading was marked complete |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
