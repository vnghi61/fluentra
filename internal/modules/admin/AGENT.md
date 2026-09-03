---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [admin_actions, feature_flags, admin_notes, moderation_items]
depends_on: [user, rbac, audit, auth, content, cache]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-08-17
---

# admin — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `core` |
| Path | `internal/modules/admin` |
| Schema | `core` |
| Delivery phase | 2 |
| Status | **PLANNED** |
| Owner | @backend-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
The back-office surface. It owns almost no data of its own; it composes other modules' contracts into the screens an administrator needs: dashboards, user management, content moderation, feature flags, and operational tooling.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Admin dashboard aggregation across modules
- User management screens (list, search, inspect, suspend, reinstate, impersonate)
- Moderation queues (flagged content, reported names, flagged submissions)
- Feature flag management
- Operational actions: retry a job, replay a webhook, invalidate a cache key
- System health summary for non-engineers

**This module does NOT own:**

- Any business rule — those live in the owning module; admin only calls contracts
- Content authoring logic — that is `content`
- Its own copies of other modules' data
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/admin/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/admin/contract/` | You are calling this module from another module |
| `internal/modules/admin/service/` | You are changing behaviour |
| `db/migrations/admin/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/admin/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `admin.FlagReader` | `IsEnabled(ctx, key, userID) bool` — used by every module that gates a feature |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `admin.impersonation_started` | publishes | `{admin_id, user_id, expires_at}` |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `core` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/admin/` · Queries: `db/queries/admin/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `core.admin_actions` | Audit log of admin operations on accounts | `actor_id`, `target_id`, `action` (suspend/reinstate/revoke_sessions), `reason`, `occurred_at`. CHECK actor_id != target_id |
| `core.feature_flags` | Runtime feature toggles | `key`, `enabled`, `rollout_percent`, `owner`, `expires_on`. Cached 30 s in-process. |
| `core.admin_notes` | Free-text notes attached to a user or content item | Visible to admins only; audited on create |
| `core.moderation_items` | The moderation queue | `kind`, `target_type`, `target_id`, `reason`, `status`, `assignee_id` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `admin`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/users` | `user.list` | Search accounts, cursor-paginated |
| `GET` | `/api/v1/admin/users/{id}` | `user.read` | One account in full |
| `POST` | `/api/v1/admin/users/{id}/suspend` | `user.suspend` | Suspend an account and end its sessions |
| `POST` | `/api/v1/admin/users/{id}/reinstate` | `user.reinstate` | Return a suspended account to active |
| `POST` | `/api/v1/admin/users/{id}/sessions/revoke` | `user.manage_sessions` | Sign a user out everywhere |
| `GET` | `/api/v1/admin/flags` | `system.flags` | List every feature flag |
| `GET` | `/api/v1/admin/ai/usage` | `admin.dashboard` | Read today's AI usage and budget headroom per provider |
| `POST` | `/api/v1/admin/flags` | `system.flags` | Create a feature flag |
| `PUT` | `/api/v1/admin/flags/{key}` | `system.flags` | Update a feature flag |
| `DELETE` | `/api/v1/admin/flags/{key}` | `system.flags` | Delete a feature flag |
| `GET` | `/api/v1/admin/dashboard` | `admin.dashboard` | Composed KPI summary |
| `GET` | `/api/v1/admin/moderation` | `moderation.read` | Moderation queue |
| `POST` | `/api/v1/admin/moderation/{id}/resolve` | `moderation.act` | Resolve a queue item |
| `POST` | `/api/v1/admin/jobs/{id}/retry` | `system.jobs` | Retry a failed job |
| `POST` | `/api/v1/admin/impersonate/{user_id}` | `user.impersonate` | Start a time-boxed impersonation session |
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
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`user`](../../modules/user/AGENT.md) | → depends on | User management screens |
| [`rbac`](../../modules/rbac/AGENT.md) | → depends on | Role assignment |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | Every admin action is recorded |
| [`auth`](../../modules/auth/AGENT.md) | → depends on | Session revocation and impersonation token issuance |
| [`content`](../../modules/content/AGENT.md) | → depends on | Moderation and publishing actions |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | Flag caching and operational cache invalidation |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-ADMIN-01** — Admin composes, it does not compute. Any rule an admin screen needs belongs in the owning module's service.
2. **BR-ADMIN-02** — Every admin write action is audited with the acting admin's ID and, where the UI requires it, a stated reason.
3. **BR-ADMIN-03** — Impersonation is time-boxed to 30 minutes, produces a visually distinct session, is fully audited, and is blocked for payment and deletion actions.
4. **BR-ADMIN-04** — A feature flag defaults to disabled and carries an owner and an expiry date; expired flags are reported weekly.
5. **BR-ADMIN-05** — Operational actions are idempotent — retrying a retry must not double-execute.
6. **BR-ADMIN-06** — An admin cannot act on their own account through admin screens (suspension, role change) — that path goes through another admin.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add an admin screen

1. Identify which module owns the data and which contract methods you need. If a method does not exist, add it to that module's contract — do not query its tables.
2. Add the endpoint under `/api/v1/admin/` in `openapi.yaml` with an `x-permission`.
3. Implement a thin handler that composes contract calls.
4. Add the permission to `rbac` and map it to the admin role.
5. Audit any write.
6. Add the React page under `web/src/features/admin/`.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- The dashboard aggregates synchronously across several modules; above a few thousand rows it will need a materialised read model.
- Moderation is manual — there is no automated classification in Phase 2.
- Feature flags are boolean plus a percentage; there is no targeting by attribute.
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
| `IMPERSONATION_FORBIDDEN_ACTION` | 403 | The attempted action is not permitted while impersonating |
| `SELF_ADMIN_ACTION_FORBIDDEN` | 403 | An admin may not administer their own account |

### Security considerations

- All `/admin/*` routes require the `admin` role at the route group and a specific permission at the service.
- Admin accounts require MFA.
- Reads of personal data are audited, not just writes.
- Impersonation tokens carry an `act` claim so downstream audit entries record both identities.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/admin/...                    # unit
go test -tags=integration ./internal/modules/admin/...  # integration (testcontainers)
```

**Focus areas**

- A non-admin receives 403 on every `/admin/*` route
- Impersonation cannot perform forbidden actions and expires on time
- Self-administration is refused
- Every write produces an audit entry with the correct actor
- Flag changes propagate within the cache TTL
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not query another module's tables from admin — call its contract.
- Do not implement business rules here.
- Do not add an admin action without a permission and an audit entry.
- Do not allow impersonation of another admin.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
