---
module: rbac
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [roles, permissions, role_permissions, user_roles]
depends_on: [cache, audit]
depended_on_by: [auth, admin, content, questionbank, exam, user]
spec_version: 1.0.0
last_verified: 2026-08-10
---

# rbac — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] Roles, permissions, mappings seeded via migration
- [ ] `Require`/`Can` guard with cached resolution
- [ ] `/admin/*` route-group middleware
- [ ] `GET /me/permissions`
- [ ] Self-elevation and last-admin protections
- [ ] A CI check that every non-public OpenAPI operation declares `x-permission`
<!-- END GENERATED: todo -->

## Progress

The list above is generated from `tools/docgen/data/core.json`, so its checkboxes cannot be ticked
by hand. Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P1.3 | 2026-08-10 | Four tables, the seeded role and permission catalogue, `Authorizer.Require`/`Can` with cached resolution and eager invalidation, the `/admin/*` middleware, `GET /me/permissions`, the role catalogue and grant/revoke operations, self-elevation and last-admin protections |
| — | 2026-08-27 | `GrantBaselineRole`: the system grant of `user` at account creation. `AssignRole` needs an actor holding `rbac.assign` and is only reachable from the admin handler, so no account created by registration or by `make seed` ever held a role — invisible until P7.1 gave `user` its first permission, and every learner's catalogue a 403 from that commit on. Wired into `user` through a one-method interface the composition root satisfies. |

Still open in Phase 1: MFA enrolment on an `admin` grant (BR-RBAC-06, needs WP2), and publishing
`rbac.access_denied` to the audit trail (BR-RBAC-07, needs P1.4).

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Resource-scoped permissions if content ownership becomes a requirement
- Temporary elevated access with automatic expiry
<!-- END GENERATED: todo-future -->
