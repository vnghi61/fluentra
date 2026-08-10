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
last_verified: 2026-08-10
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

## What is actually served (P1.4)

Three of the four above are in `openapi.yaml` and mounted. `GET /admin/audit-logs/export` is
not: an async export needs `platform/storage` for the signed URL, which is not a dependency of
this module. See [`TODO.md`](TODO.md).

### Search parameters

`GET /admin/audit-logs` and `GET /admin/security-events` share the window, cursor and limit
parameters and differ only in what they filter on.

| Parameter | Both | `audit-logs` | `security-events` |
|---|---|---|---|
| `from`, `to` | RFC 3339. `to` defaults to now, `from` to 90 days before it | | |
| `cursor` | Opaque, from the previous page's `next_cursor` | | |
| `limit` | 1–100, default 20 | | |
| filters | | `actor_id`, `action`, `target_type`, `target_id` | `kind`, `severity`, `resolved`, `user_id` |

**The window is the important one.** Both tables are partitioned monthly on `created_at`. A
query with no bound on it reads every partition ever created, so the server supplies a default
rather than letting a caller omit one, and trims any span over 400 days from the far end. A
client that wants everything gets the most recent 400 days of it.

Ordering is `created_at DESC, id DESC` — the primary key of both tables — so the keyset cursor
pages deep at the same cost as shallow. `next_cursor` is present only when another page exists.

### Two things the responses do not carry

- **`ip_hash`.** The column exists; no response exposes it. It is a pseudonymous identifier that
  correlates one person's activity across sessions, and reading the trail does not need it.
- **Values, usually.** `changed_fields` names what moved; `before`/`after` are null unless the
  emitting module sent values, and anything on the PII deny-list is `[redacted]` when it did
  (BR-AUDIT-04). An audit log holding every old display name would be a second store of personal
  data with a longer retention period than the first.

## Error codes

<!-- BEGIN GENERATED: api-errors -->
_None yet._
<!-- END GENERATED: api-errors -->

| Code | Status | When |
|---|---|---|
| `UNAUTHENTICATED` | 401 | No actor on the request |
| `PERMISSION_DENIED` | 403 | The caller lacks `audit.read` or `audit.manage` |
| `NOT_FOUND` | 404 | No such security event — also returned for a malformed id, so probing the surface reveals nothing about its shape |
| `SECURITY_EVENT_ALREADY_RESOLVED` | 409 | A second resolution. The first administrator's explanation describes what was investigated; the second would replace it with a guess |
| `BAD_REQUEST` | 400 | The cursor did not decode. It is a token the server issued, not a field a person typed |
| `VALIDATION_FAILED` | 422 | A malformed filter, an out-of-range `limit`, `from` after `to`, or a missing/blank/overlong `note` |

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
