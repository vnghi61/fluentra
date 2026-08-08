---
module: mailer
tier: platform
group: platform
status: READY
phase: 1
owner: "@platform-team"
schema: comm
tables: [email_log, email_suppressions]
depends_on: [job, telemetry, storage]
depended_on_by: [auth, user, notification, subscription, analytics]
spec_version: 1.0.0
last_verified: 2026-08-07
---

# mailer — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/mailer` |
| Schema | `comm` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @platform-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Email transport: template rendering, localisation, sending through SMTP or a provider API, bounce and complaint handling, suppression lists, and a delivery log.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- MJML/HTML template rendering with a plain-text alternative
- Localisation of subject and body
- Sending via SMTP or a provider API, behind one interface
- Retry with backoff on transient failures
- Bounce and complaint handling; suppression list
- Delivery log for support and debugging
- A development mailbox (Mailpit) so no real email leaves a developer machine

**This module does NOT own:**

- Deciding whether to send — that is `notification`
- Marketing campaigns or list management
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/mailer/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/mailer/contract/` | You are calling this module from another module |
| `internal/platform/mailer/service/` | You are changing behaviour |
| `db/migrations/mailer/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/platform/mailer/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `mailer.Sender` | `Send(ctx, Message)` — always enqueues, never blocks the caller |
| struct | `mailer.Message` | `{To, Template, Locale, Data, Category}` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `email.bounced` | publishes | `{email_hash, reason}` |
| `user.deleted` | consumes | Purge the delivery log entries for that address |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `comm` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/mailer/` · Queries: `db/queries/mailer/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `comm.email_log` | Delivery record | `to_hash`, `template`, `locale`, `status`, `provider_message_id`, `error`. Partitioned monthly. |
| `comm.email_suppressions` | Addresses we must not email | `email_hash` UNIQUE, `reason` (hard_bounce/complaint/unsubscribe) |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `mailer`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/emails` | `system.email` | Delivery log search |
| `POST` | `/api/v1/admin/emails/{id}/resend` | `system.email` | Resend a failed message |
| `POST` | `/api/v1/webhooks/email` | `public` | Provider bounce and complaint webhook |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`job`](../../platform/job/AGENT.md) | → depends on | All sending is asynchronous |
| [`telemetry`](../../platform/telemetry/AGENT.md) | → depends on | Delivery metrics |
| [`storage`](../../platform/storage/AGENT.md) | → depends on | Attachments such as data exports |
| [`auth`](../../modules/auth/AGENT.md) | ← used by | consumes this module's contract |
| [`user`](../../modules/user/AGENT.md) | ← used by | consumes this module's contract |
| [`notification`](../../modules/notification/AGENT.md) | ← used by | consumes this module's contract |
| [`subscription`](../../modules/subscription/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-MAILER-01** — Sending is always asynchronous — a mail provider outage must never fail a user request.
2. **BR-MAILER-02** — An address on the suppression list is never emailed, except for account-security messages, which bypass suppression only when the address is not a hard bounce.
3. **BR-MAILER-03** — Every template has a plain-text alternative and exists in every supported locale before it can be used.
4. **BR-MAILER-04** — Templates are rendered with escaping; user-supplied values are never interpolated as HTML.
5. **BR-MAILER-05** — Email addresses are logged as hashes; the delivery log stores `to_hash`, not the address.
6. **BR-MAILER-06** — Every non-security email includes a working unsubscribe link.
7. **BR-MAILER-07** — A hard bounce suppresses the address immediately; a soft bounce retries up to three times.
8. **BR-MAILER-08** — In development and CI, all mail is captured by Mailpit; a real send from a non-production environment is a configuration bug.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add an email template

1. Create the MJML source and the plain-text alternative.
2. Add every supported locale — a missing locale is a startup validation failure, not a runtime fallback.
3. Add a preview fixture so the template renders in the dev mailbox with realistic data.
4. Test rendering with hostile input (HTML in a display name) to confirm escaping.
5. Add it to the notification category mapping if it is user-triggered.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- No A/B testing or send-time optimisation for email; that logic, if it ever exists, belongs in `notification`.
- Attachment size is capped at 10 MB; larger artefacts are delivered by presigned link instead.
- Deliverability monitoring (SPF/DKIM/DMARC reporting) is manual.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `EMAIL_SUPPRESSED` | 409 | Address is on the suppression list |
| `TEMPLATE_NOT_FOUND` | 500 | Template missing for the requested locale |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/platform/mailer/...                    # unit
go test -tags=integration ./internal/platform/mailer/...  # integration (testcontainers)
```

**Focus areas**

- Suppression honoured for non-security categories and bypassed correctly for security ones
- Retry classification: transient retries, hard bounce suppresses
- Template rendering escapes hostile input
- Every template exists in every locale (a startup validation test)
- Webhook signature verification rejects a forged payload
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not send synchronously from a request handler.
- Do not log a raw email address.
- Do not add a template without every locale and a plain-text alternative.
- Do not interpolate user input into HTML without escaping.
- Do not send real email from development or CI.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
