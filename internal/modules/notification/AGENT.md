---
module: notification
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: comm
tables: [notifications, notification_preferences, devices, notification_dedupe]
depends_on: [mailer, job, cache, user]
depended_on_by: [auth, writing, speaking, exam, gamification, subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# notification — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `core` |
| Path | `internal/modules/notification` |
| Schema | `comm` |
| Delivery phase | 2 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Turns domain events into messages a learner actually sees: in-app notifications, push notifications and emails. Owns delivery preferences, quiet hours, digesting, and deduplication.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Notification templates and rendering per channel and locale
- Delivery preferences per category and channel
- Quiet hours in the learner's own timezone
- Digesting: collapsing many events into one message
- Deduplication and rate limiting per user
- In-app inbox: list, mark read, unread count
- Push device registration and token lifecycle

**This module does NOT own:**

- Sending the email — `platform/mailer` does the transport
- Deciding that something happened — modules publish events
- Marketing campaigns
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/notification/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/notification/contract/` | You are calling this module from another module |
| `internal/modules/notification/service/` | You are changing behaviour |
| `db/migrations/notification/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/notification/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `notification.Notifier` | `Notify(ctx, userID, category, payload)` — modules may call it directly for immediate, non-event-driven notices |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `notification.sent` | publishes | `{user_id, category, channel}` — feeds engagement analytics |
| `writing.graded` | consumes | Tell the learner their feedback is ready |
| `speaking.scored` | consumes | Same, for speaking |
| `review.due_soon` | consumes | Daily review reminder |
| `gamification.streak_at_risk` | consumes | Streak reminder, timed to the learner's habit |
| `exam.attempt_finished` | consumes | Score report ready |
| `subscription.expiring` | consumes | Renewal notice |
| `auth.password_changed` | consumes | Security notice |
| `user.deleted` | consumes | Purge inbox and devices |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `comm` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/notification/` · Queries: `db/queries/notification/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `comm.notifications` | In-app inbox | `user_id`, `category`, `payload` jsonb, `read_at`, `created_at`. Partitioned monthly. |
| `comm.notification_preferences` | Per-category channel settings | `category`, `in_app`, `push`, `email`, plus quiet-hours window |
| `comm.devices` | Push endpoints | `token`, `platform`, `last_seen_at`; stale tokens pruned |
| `comm.notification_dedupe` | Suppression keys | `dedupe_key` UNIQUE, `expires_at` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `notification`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/notifications` | `self` | Inbox, newest first |
| `GET` | `/api/v1/notifications/unread-count` | `self` | Badge count (cached) |
| `POST` | `/api/v1/notifications/{id}/read` | `self` | Mark one read |
| `POST` | `/api/v1/notifications/read-all` | `self` | Mark all read |
| `GET` | `/api/v1/me/notification-preferences` | `self` | Read preferences |
| `PUT` | `/api/v1/me/notification-preferences` | `self` | Update preferences |
| `POST` | `/api/v1/me/devices` | `self` | Register a push device |
| `DELETE` | `/api/v1/me/devices/{id}` | `self` | Unregister |
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
| [`mailer`](../../platform/mailer/AGENT.md) | → depends on | Email transport and template rendering |
| [`job`](../../platform/job/AGENT.md) | → depends on | Asynchronous dispatch and digest scheduling |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | Unread counts, dedupe keys, per-user rate limits |
| [`user`](../../modules/user/AGENT.md) | → depends on | Locale, timezone and preferences |
| [`auth`](../../modules/auth/AGENT.md) | ← used by | consumes this module's contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
| [`subscription`](../../modules/subscription/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-NOTIFICATION-01** — A notification is never sent for a category the user has disabled, on a channel they have disabled.
2. **BR-NOTIFICATION-02** — Quiet hours are evaluated in the learner's own timezone; a message that would land inside them is deferred to the next allowed slot, not dropped — unless it is a security notice, which always sends.
3. **BR-NOTIFICATION-03** — Security notices (password changed, new device, session revoked) ignore preferences and quiet hours.
4. **BR-NOTIFICATION-04** — Deduplication: the same `dedupe_key` within its window produces exactly one message.
5. **BR-NOTIFICATION-05** — No more than one push and three emails per user per day, excluding security notices.
6. **BR-NOTIFICATION-06** — Related events within a digest window collapse into one message ("3 essays graded") rather than three.
7. **BR-NOTIFICATION-07** — Delivery is at-least-once from the outbox; the dispatcher is idempotent on `event_id`.
8. **BR-NOTIFICATION-08** — A push token that fails twice with a permanent error is deleted.
9. **BR-NOTIFICATION-09** — Every message includes a working unsubscribe or preferences link, except security notices.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a notification category

1. Add the category constant and its default channel settings.
2. Write templates for every channel and every supported locale.
3. Subscribe to the triggering event, or call `Notify` directly for a non-event trigger.
4. Choose a dedupe key and window, and decide whether it participates in a digest.
5. Add it to the preferences UI.
6. Test: disabled preference suppresses it; quiet hours defer it; duplicates collapse.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Web push only — no native mobile push until there is a mobile app.
- Digest windows are fixed per category rather than learned from behaviour.
- Send-time optimisation is a fixed heuristic (the learner's usual study hour), not a model.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:notification:unread:{user_id}:v1` | 5 min | New notification, mark-read |
| `fluentra:{env}:notification:dedupe:{key}:v1` | per category | Natural expiry |
| `fluentra:{env}:notification:ratelimit:{user_id}:{channel}:v1` | 24 h | Natural expiry |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `DEVICE_LIMIT_REACHED` | 409 | Too many registered push devices |
| `INVALID_QUIET_HOURS` | 422 | Window is malformed or spans more than 12 hours |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/notification/...                    # unit
go test -tags=integration ./internal/modules/notification/...  # integration (testcontainers)
```

**Focus areas**

- Preference matrix: every category × channel combination behaves
- Quiet hours across timezone boundaries and DST transitions
- Deduplication under concurrent event delivery
- Digest collapsing
- Security notices bypass preferences and quiet hours
- Idempotency on redelivered events
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not send a notification synchronously inside a request — enqueue it.
- Do not bypass preference checks for anything except a genuine security notice.
- Do not put personal content (essay text, scores) in a push payload — link to the app.
- Do not add a category without templates for every supported locale.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
