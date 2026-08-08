---
module: mailer
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: comm
tables: [email_log, email_suppressions]
depends_on: [job, telemetry, storage]
depended_on_by: [auth, user, notification, subscription, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# mailer — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `mailer`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/emails` | `system.email` | Delivery log search |
| `POST` | `/api/v1/admin/emails/{id}/resend` | `system.email` | Resend a failed message |
| `POST` | `/api/v1/webhooks/email` | `public` | Provider bounce and complaint webhook |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/admin/emails`

Delivery log search

| | |
|---|---|
| Permission | `system.email` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/admin/emails/{id}/resend`

Resend a failed message

| | |
|---|---|
| Permission | `system.email` |
| Success | 202 |
| Errors | standard set |

### `POST /api/v1/webhooks/email`

Provider bounce and complaint webhook

| | |
|---|---|
| Permission | `public` |
| Success | 200 |
| Errors | standard set |
| Notes | Signature-verified on the raw body |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `EMAIL_SUPPRESSED` | 409 | Address is on the suppression list |
| `TEMPLATE_NOT_FOUND` | 500 | Template missing for the requested locale |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
