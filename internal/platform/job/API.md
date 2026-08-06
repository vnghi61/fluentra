---
module: job
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: ops
tables: [river_job, outbox_events, job_failures]
depends_on: [telemetry]
depended_on_by: [auth, user, audit, notification, mailer, content, writing, speaking, media, ai, srs, analytics, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# job — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `job`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/jobs` | `system.jobs` | List jobs by queue and state |
| `POST` | `/api/v1/admin/jobs/{id}/retry` | `system.jobs` | Retry a failed job |
| `POST` | `/api/v1/admin/jobs/{id}/cancel` | `system.jobs` | Cancel a pending job |
| `GET` | `/api/v1/admin/queues` | `system.jobs` | Depth, oldest pending age, throughput per queue |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/jobs`

List jobs by queue and state

| | |
|---|---|
| Permission | `system.jobs` |
| Success | 200 |
| Errors | standard set |


### `POST /api/v1/admin/jobs/{id}/retry`

Retry a failed job

| | |
|---|---|
| Permission | `system.jobs` |
| Success | 202 |
| Errors | standard set |


### `POST /api/v1/admin/jobs/{id}/cancel`

Cancel a pending job

| | |
|---|---|
| Permission | `system.jobs` |
| Success | 204 |
| Errors | standard set |


### `GET /api/v1/admin/queues`

Depth, oldest pending age, throughput per queue

| | |
|---|---|
| Permission | `system.jobs` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `JOB_NOT_RETRYABLE` | 409 | Admin attempted to retry a job that failed for a business reason |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
