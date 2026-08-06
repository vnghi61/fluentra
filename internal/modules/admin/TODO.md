---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [feature_flags, admin_notes, moderation_items]
depends_on: [user, rbac, audit, auth, content, cache]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-08-06
---

# admin — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] Dashboard composing user, learning and AI-cost summaries
- [ ] User management screens with audit on every action
- [ ] Feature flag CRUD with in-process caching
- [ ] Moderation queue with assignment and resolution
- [ ] Job retry and webhook replay tooling
- [ ] Impersonation with all its guards
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Materialised dashboard read model
- Bulk content operations
- Saved admin searches
- Automated moderation triage
<!-- END GENERATED: todo-future -->
