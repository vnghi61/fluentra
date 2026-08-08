---
module: analytics
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@data-team"
schema: analytics
tables: [analytics_events, daily_rollups, cohorts, funnel_steps, learner_outcomes]
depends_on: [job, cache]
depended_on_by: [admin, notification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# analytics — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `analytics`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/analytics/kpis` | `analytics.read` | Headline KPIs with period comparison |
| `GET` | `/api/v1/admin/analytics/funnel` | `analytics.read` | Funnel conversion by cohort |
| `GET` | `/api/v1/admin/analytics/retention` | `analytics.read` | Retention curve by cohort |
| `GET` | `/api/v1/admin/analytics/outcomes` | `analytics.read` | Learning outcome metrics |
| `POST` | `/api/v1/admin/analytics/export` | `analytics.export` | Async CSV export |
| `GET` | `/api/v1/me/insights` | `self` | The learner's own progress insights |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/analytics/kpis`

Headline KPIs with period comparison

| | |
|---|---|
| Permission | `analytics.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/analytics/funnel`

Funnel conversion by cohort

| | |
|---|---|
| Permission | `analytics.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/analytics/retention`

Retention curve by cohort

| | |
|---|---|
| Permission | `analytics.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/analytics/outcomes`

Learning outcome metrics

| | |
|---|---|
| Permission | `analytics.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/analytics/export`

Async CSV export

| | |
|---|---|
| Permission | `analytics.export` |
| Success | 202 |
| Errors | standard set |

### `GET /api/v1/me/insights`

The learner's own progress insights

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
| `PERIOD_TOO_LARGE` | 422 | Requested range exceeds the query limit |
| `ROLLUP_NOT_READY` | 409 | The requested day has not been rolled up yet |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
