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
last_verified: 2026-08-06
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

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Resource-scoped permissions if content ownership becomes a requirement
- Temporary elevated access with automatic expiry
<!-- END GENERATED: todo-future -->
