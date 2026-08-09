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

# user — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] `users`, `profiles`, `user_preferences` tables and CRUD
- [ ] `GET/PATCH /me` and preferences endpoints
- [ ] Avatar upload via presigned URL, with EXIF stripping
- [ ] Admin user list, search, suspend, reinstate — all audited
- [ ] Deletion request with 30-day grace and cancellation
- [ ] Export job producing a signed link
- [ ] `user.deleted` fan-out with an erasure completeness check
<!-- END GENERATED: todo -->

## Progress

The list above is generated from `tools/docgen/data/core.json`, so its checkboxes cannot be
ticked by hand — `make docs` would put them back. Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P1.1 | 2026-08-09 | `core.users`, `core.profiles`, `core.user_preferences`, `core.learning_profiles`, the four `core` enums and the `citext` extension; the sqlc query set in `db/queries/user/`; schema and query integration tests |

That leaves the first generated item — "`users`, `profiles`, `user_preferences` tables and
CRUD" — half done: the tables and the SQL exist, the module that calls them does not. **P1.2**
adds `contract`, `domain`, `service`, `repository` and `transport/http`.

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Verified email change
- Profile completeness prompts driving onboarding
- Per-user data-retention preferences
<!-- END GENERATED: todo-future -->
