---
module: telemetry
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: []
depended_on_by: [ai, cache, storage, job, media, search, mailer]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# telemetry — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `telemetry`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/health` | `public` | Liveness — the process is running |
| `GET` | `/ready` | `public` | Readiness — dependencies reachable and migrations current |
| `GET` | `/version` | `public` | Build version and commit |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /health`

Liveness — the process is running

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |


### `GET /ready`

Readiness — dependencies reachable and migrations current

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | `DEPENDENCY_UNAVAILABLE` |


### `GET /version`

Build version and commit

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |


<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
_None yet._
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
