---
module: audit
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: audit
tables: [audit_logs, security_events]
depends_on: [job]
depended_on_by: [auth, user, rbac, admin, content, questionbank, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# audit — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `audit`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/audit-logs` | `audit.read` | Search the audit trail |
| `GET` | `/api/v1/admin/audit-logs/export` | `audit.export` | Async CSV export |
| `GET` | `/api/v1/admin/security-events` | `audit.read` | Security event feed |
| `POST` | `/api/v1/admin/security-events/{id}/resolve` | `audit.manage` | Mark an event triaged |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/audit-logs`

Search the audit trail

| | |
|---|---|
| Permission | `audit.read` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/admin/audit-logs/export`

Async CSV export

| | |
|---|---|
| Permission | `audit.export` |
| Success | 202 |
| Errors | standard set |

### `GET /api/v1/admin/security-events`

Security event feed

| | |
|---|---|
| Permission | `audit.read` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/security-events/{id}/resolve`

Mark an event triaged

| | |
|---|---|
| Permission | `audit.manage` |
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
