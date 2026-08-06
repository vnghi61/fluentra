---
module: lesson
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [courses, course_units, lessons, activities, lesson_prerequisites]
depends_on: [content, cache]
depended_on_by: [learning, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# lesson — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `lesson`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/courses` | `content.read.published` | Catalogue with level filters |
| `GET` | `/api/v1/courses/{slug}` | `content.read.published` | Course with units and lesson summaries |
| `GET` | `/api/v1/lessons/{id}` | `content.read.published` | Lesson with its activities and resolved content |
| `POST` | `/api/v1/admin/courses` | `content.create` | Create a course |
| `PUT` | `/api/v1/admin/lessons/{id}/activities` | `content.edit` | Reorder or replace the activity list |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/courses`

Catalogue with level filters

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/courses/{slug}`

Course with units and lesson summaries

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | standard set |


### `GET /api/v1/lessons/{id}`

Lesson with its activities and resolved content

| | |
|---|---|
| Permission | `content.read.published` |
| Success | 200 |
| Errors | `LESSON_LOCKED` |


### `POST /api/v1/admin/courses`

Create a course

| | |
|---|---|
| Permission | `content.create` |
| Success | 201 |
| Errors | standard set |


### `PUT /api/v1/admin/lessons/{id}/activities`

Reorder or replace the activity list

| | |
|---|---|
| Permission | `content.edit` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `LESSON_LOCKED` | 403 | Prerequisites not met |
| `PREREQUISITE_CYCLE` | 422 | The proposed graph contains a cycle |
| `ACTIVITY_CONTENT_UNPUBLISHED` | 409 | Cannot publish a lesson pointing at draft content |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
