---
module: user
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [users, profiles, user_preferences, learning_profiles, user_deletions, user_exports]
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

| P1.1 | 2026-08-09 | `core.users`, `core.profiles`, `core.user_preferences`, `core.learning_profiles`, the four `core` enums and the `citext` extension; the sqlc query set in `db/queries/user/`; schema and query integration tests |
| P1.2 | 2026-08-09 | The vertical slice: `contract` (`Reader` with batched `GetManyByIDs`, `Creator`, `Summary`, events), `domain`, `repository`, `service`, `transport/http`. `GET`/`PATCH /me` and `GET`/`PUT /me/preferences` |
| P3.1 | 2026-08-15 | Avatar upload lifecycle: `POST /api/v1/me/avatar/upload-intent` presigned upload, `PUT /api/v1/me/avatar` confirmation, magic bytes sniffing, pure-Go JPEG image resizing to 3 sizes (64x64, 128x128, 256x256) with EXIF GPS stripping, outbox event publishing, and safe old-avatar cleanup |
| P3.2 | 2026-08-16 | GDPR User Data Export: `POST /api/v1/me/export` & `GET /api/v1/me/export/{id}`, River background export worker collecting data across modules (`user`, `auth`, `rbac`, `audit`), ZIP packaging with `metadata.json`, MinIO upload, 24h presigned URL email delivery, 7-day retention cleanup cron job |
| P3.3 | 2026-08-16 | GDPR User Account Deletion: `DELETE /api/v1/me`, `POST /api/v1/me/deletion/cancel`, `GET /api/v1/me/deletion/{id}` with 30-day grace period, immediate session revocation (`user.deletion_requested`), daily `DeletionExecutor` cron job (`1_700_000_050`), user/profile anonymisation preserving aggregate stats, preferences/learning profile deletion, and event-driven data purge (`user.deleted`) across `auth` and `rbac`. |

That closes the first three generated items, export (P3.2), deletion (P3.3), and the `user.deleted` fan-out. Still open in Phase 1: the
admin user group (P4.1).

Not started, and deliberately: `core.learning_profiles` has a table and queries but no service or
endpoint. It gets one when onboarding needs it.

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
