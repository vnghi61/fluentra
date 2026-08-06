---
module: exam
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [exams, exam_sections, exam_attempts, attempt_answers, score_reports, integrity_events]
depends_on: [questionbank, job, ai, writing, speaking, learning]
depended_on_by: [learning, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# exam — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `exam`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/exams` | `content.read.published` | Available mock exams |
| `POST` | `/api/v1/exams/{id}/attempts` | `self` | Start a sitting |
| `GET` | `/api/v1/exam-attempts/{id}` | `self` | Current state with server time remaining |
| `PUT` | `/api/v1/exam-attempts/{id}/answers` | `self` | Save answers (autosave) |
| `POST` | `/api/v1/exam-attempts/{id}/sections/{n}/complete` | `self` | Finish a section |
| `POST` | `/api/v1/exam-attempts/{id}/submit` | `self` | Submit the whole exam |
| `GET` | `/api/v1/exam-attempts/{id}/report` | `self` | Score report when ready |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/exams`

Available mock exams

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/exams/{id}/attempts`

Start a sitting

| | |
|---|---|
| Permission | `self` |
| Success | 201 |
| Errors | `EXAM_WINDOW_CLOSED`, `ATTEMPT_IN_PROGRESS`, `INSUFFICIENT_ITEMS` |


### `GET /api/v1/exam-attempts/{id}`

Current state with server time remaining

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `PUT /api/v1/exam-attempts/{id}/answers`

Save answers (autosave)

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `ATTEMPT_EXPIRED` |


### `POST /api/v1/exam-attempts/{id}/sections/{n}/complete`

Finish a section

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/exam-attempts/{id}/submit`

Submit the whole exam

| | |
|---|---|
| Permission | `self` |
| Success | 202 |
| Errors | standard set |
| Notes | 202 because writing and speaking sections grade asynchronously |

### `GET /api/v1/exam-attempts/{id}/report`

Score report when ready

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `ATTEMPT_IN_PROGRESS` | 409 | Finish or abandon the existing attempt first |
| `EXAM_ALREADY_SUBMITTED` | 409 | Terminal state |
| `EXAM_WINDOW_CLOSED` | 403 | Outside the availability window |
| `SECTION_TIME_EXPIRED` | 409 | Section time is over |
| `INSUFFICIENT_ITEMS` | 409 | Not enough approved items to build the exam |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
