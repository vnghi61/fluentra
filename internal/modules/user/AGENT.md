---
module: user
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [users, profiles, user_preferences, learning_profiles, user_deletion_requests, user_exports]
depends_on: [storage, mailer, audit]
depended_on_by: [auth, admin, learning, notification, subscription, gamification]
spec_version: 1.0.0
last_verified: 2026-08-09
---

# user — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `core` |
| Path | `internal/modules/user` |
| Schema | `core` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Owns the user record and everything descriptive about a person: profile, preferences, locale, timezone, avatar, learning goals, and the account lifecycle including export and deletion. It is the canonical source of `core.users`, which every other module references by ID.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- The `core.users` record: identity, status, timestamps
- Profile: display name, avatar, country, timezone, locale, date of birth (for the age gate)
- Preferences: notification settings, daily goal, UI theme, AI processing opt-out
- Learning profile: self-declared level, goals, target exam, study reminders
- Account status transitions: active, suspended, pending deletion
- Data export (async) and erasure (30-day grace, then anonymisation)
- Avatar upload coordination through `platform/storage`

**This module does NOT own:**

- Passwords, sessions, tokens — that is `auth`
- Roles and permissions — that is `rbac`
- Learning progress and statistics — that is `learning`
- Subscription state — that is `subscription`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/user/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/user/contract/` | You are calling this module from another module |
| `internal/modules/user/service/` | You are changing behaviour |
| `db/migrations/user/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/user/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `user.Reader` | `GetByID`, `GetManyByIDs`, `Exists` — batched to avoid N+1 across modules |
| interface | `user.Creator` | `CreateUser` — used only by `auth` during registration |
| struct | `user.Summary` | `{ID, DisplayName, AvatarURL, Locale, Timezone, Status}` — the shape other modules render |
| event | `user.DeletionRequested` | Every module holding personal data reacts to this |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `user.profile_updated` | publishes | `{user_id, changed_fields}` |
| `user.deletion_requested` | publishes | `{user_id, execute_after}` |
| `user.deleted` | publishes | `{user_id}` — modules must purge or anonymise their data |
| `user.suspended` | publishes | `{user_id, reason, actor_id}` |
| `subscription.activated` | consumes | Cache the entitlement tier on the profile for fast reads |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `core` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/user/` · Queries: `db/queries/user/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `core.users` | Canonical user identity | `email` (citext, UNIQUE), `status`, `email_verified_at`, `created_at`. **The one table other schemas may FK to** (ADR-0004) |
| `core.profiles` | Descriptive profile data | 1:1 with users; display name, avatar asset ID, country, timezone |
| `core.user_preferences` | Settings | Locale, theme, daily goal, notification channels, quiet hours, `ai_processing_opt_out` |
| `core.learning_profiles` | Self-declared learning context | Declared level, target level, target exam, weekly minutes goal, motivations |
| `core.user_deletion_requests` | Erasure workflow | `requested_at`, `execute_after`, `cancelled_at`, `completed_at` |
| `core.user_exports` | Data export jobs and artefacts | `status`, `object_key`, `expires_at` |

<!-- END GENERATED: schema -->

### What exists today (P1.1, `db/migrations/user/1700000010_create_core_user_tables.sql`)

Four of the six tables above are real: `users`, `profiles`, `user_preferences`,
`learning_profiles`. `user_deletion_requests` and `user_exports` arrive with WP3 (P3.2, P3.3).

| Object | Notes |
|---|---|
| extension `citext` | Created by this migration. It is what makes BR-USER-01 a database guarantee instead of a convention every query has to remember |
| enum `core.user_status` | `active`, `suspended`, `pending_deletion`, `deleted` |
| enum `core.ui_theme` | `light`, `dark`, `system` |
| enum `core.cefr_level` | `a1`…`c2`, lower-case |
| enum `core.target_exam` | `ielts`, `toeic`, `none` |

`core.users` carries exactly six columns — `id`, `email`, `status`, `email_verified_at`,
`created_at`, `updated_at` — and an integration test fails if a seventh appears. Everything
descriptive belongs to `profiles`, every setting to `user_preferences`.

The three satellite tables are 1:1 with `users` via a `UNIQUE` constraint on `user_id`, which
is also the index the foreign key needs; they cascade on delete.

The enums are written as bare `CREATE TYPE` rather than guarded by a `DO` block, because sqlc
parses the migration as its schema source and cannot see inside a `DO` block — guarded, every
enum column generates `interface{}` instead of a typed Go constant.

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `user`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/me` | `self` | Full profile of the caller |
| `PATCH` | `/api/v1/me` | `self` | Update profile fields |
| `GET` | `/api/v1/me/preferences` | `self` | Read preferences |
| `PUT` | `/api/v1/me/preferences` | `self` | Replace preferences |
| `POST` | `/api/v1/me/avatar/upload-intent` | `self` | Get a presigned URL for an avatar upload |
| `PUT` | `/api/v1/me/avatar` | `self` | Confirm the uploaded avatar |
| `POST` | `/api/v1/me/export` | `self` | Request a data export |
| `DELETE` | `/api/v1/me` | `self` | Request account deletion (30-day grace) |
| `POST` | `/api/v1/me/deletion/cancel` | `self` | Cancel a pending deletion |
| `GET` | `/api/v1/admin/users` | `user.list` | Search and list users |
| `GET` | `/api/v1/admin/users/{id}` | `user.read` | Read one user |
| `POST` | `/api/v1/admin/users/{id}/suspend` | `user.suspend` | Suspend an account |
| `POST` | `/api/v1/admin/users/{id}/reinstate` | `user.suspend` | Reinstate a suspended account |
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
| [`storage`](../../platform/storage/AGENT.md) | → depends on | Avatar upload and export artefacts |
| [`mailer`](../../platform/mailer/AGENT.md) | → depends on | Deletion confirmation, export-ready notice |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | Record profile and status changes |
| [`auth`](../../modules/auth/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`notification`](../../modules/notification/AGENT.md) | ← used by | consumes this module's contract |
| [`subscription`](../../modules/subscription/AGENT.md) | ← used by | consumes this module's contract |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-USER-01** — `core.users.email` is unique, case-insensitive (citext), and immutable after verification — changing it is a separate verified flow, not a profile edit.
2. **BR-USER-02** — A display name is 1–50 characters, must not impersonate staff (a reserved-word list), and is moderated on report.
3. **BR-USER-03** — Timezone must be a valid IANA name; all stored timestamps remain UTC.
4. **BR-USER-04** — Date of birth is used only for the age gate; users under 16 require a guardian email and receive no marketing.
5. **BR-USER-05** — Deletion is a 30-day grace period: the account is unusable immediately, cancellable by the user, and irreversible afterwards.
6. **BR-USER-06** — On execution, PII is hard-deleted and learning statistics are anonymised to a synthetic ID so aggregate analytics remain valid.
7. **BR-USER-07** — An export is produced asynchronously and delivered as a signed link valid for 24 hours; the artefact is deleted after 7 days.
8. **BR-USER-08** — Suspending a user revokes their sessions immediately via `auth`'s contract.
9. **BR-USER-09** — Setting `ai_processing_opt_out` disables AI grading only; deterministic exercises continue to work.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a profile field

1. Decide which table it belongs to: identity (`users`), descriptive (`profiles`), settings (`user_preferences`), or learning context (`learning_profiles`). Do **not** add it to `users`.
2. Write the migration in `db/migrations/user/`.
3. Add it to the sqlc query and the domain struct.
4. Add it to `contract/dto.go` only if another module needs it.
5. Update `openapi.yaml` and the frontend types via `make gen`.
6. Add validation, and a test for the boundary values.
7. Update this AGENT.md §5 and the front-matter `tables:` list.

### Add a module that stores personal data

1. Subscribe to `user.deletion_requested` and `user.deleted` in that module.
2. Implement idempotent purge or anonymisation.
3. Add the module to the erasure completeness check.
4. Document what it holds in `docs/security/data-inventory.md`.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Email change is not implemented in Phase 1 — it requires a re-verification flow.
- Avatar moderation is manual and report-driven; there is no automated image classification.
- The export format is a ZIP of JSON and media; there is no interoperable standard we target.
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
| `fluentra:{env}:user:summary:{user_id}:v1` | 10 min | `user.profile_updated` |
| `fluentra:{env}:user:prefs:{user_id}:v1` | 10 min | Preference write |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `DISPLAY_NAME_NOT_ALLOWED` | 422 | Reserved or moderated name |
| `INVALID_STATE_TRANSITION` | 409 | e.g. cancelling a deletion that already executed |
| `EXPORT_ALREADY_PENDING` | 409 | One export at a time |
| `AGE_RESTRICTED` | 403 | Under-16 account without guardian consent |

### Security considerations

- Only the account owner and an admin with `user.read` may read a profile; every query filters by actor.
- Admin reads of personal data are audited with the acting admin's ID.
- Export artefacts live in a private bucket and are reachable only through a short-lived presigned URL.
- Avatars are size- and type-limited, stripped of EXIF, and re-encoded before being made public.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **85% service, 90% domain**

```bash
go test ./internal/modules/user/...                    # unit
go test -tags=integration ./internal/modules/user/...  # integration (testcontainers)
```

**Focus areas**

- Deletion grace period: cancellable before, irreversible after
- Anonymisation preserves aggregate statistics while removing PII
- Every other module's purge handler is idempotent
- Timezone and locale validation, including odd IANA names
- Age gate boundaries around the 16th birthday
- Admin cannot read a profile without `user.read`, and the read is audited
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not add columns to `core.users` — use `profiles`, `user_preferences`, or your own module's table.
- Do not read another module's tables to build a profile response; call their contract.
- Do not expose `email` in any response other than the owner's own `/me`.
- Do not implement deletion as a cascade of `DELETE` statements from this module — publish the event.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
