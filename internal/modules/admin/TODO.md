---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [admin_actions, feature_flags, admin_notes, moderation_items, banners]
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
- [ ] Moderation queue with assignment and resolution
- [ ] Job retry and webhook replay tooling
- [ ] Impersonation with all its guards
<!-- END GENERATED: todo -->

## Done (hand-written)

- [x] Dashboard announcement banners (2026-10-08): `core.banners`, `GET /banners/active`,
  `/admin/banners` CRUD under `system.flags`, audited writes, 30 s in-process cache of the
  active list, admin screen and dashboard carousel. A write invalidates the cache only on the
  instance that served it; other instances catch up within the TTL. Repository SQL has no
  integration test yet (add one to `admin_integration_test.go` beside the flag cases).
- [x] Banner images (2026-10-08, migration 952): presigned upload to `banners/raw/<admin>/`,
  server-side sniff, 40 MP guard, fit 1600x900, JPEG re-encode (metadata dropped, alpha on
  white), unique `image_key` per banner, old image deleted on replace/clear/delete, URLs signed
  when the active cache fills. Migration constraints checked by hand against dev Postgres in a
  rolled-back transaction; still no integration test (same gap as above).
- [x] Banner kinds (2026-10-09, migration 953): `announcement` (dialog once per visit; a
  dismissible one stays closed on the device, a non-dismissible one returns next session)
  and `slide` (image-only carousel, 6 s autoplay with pause, prev/next, dots and swipe,
  never dismissed). Per-kind limits 3 and 6. Constraints checked against dev Postgres in
  a rolled-back transaction.
- [ ] Sweep orphaned banner images: an image processed but never saved on a banner (the admin
  closed the form) stays under `banners/images/`. Done when a periodic job deletes objects
  older than a day that no `core.banners.image_key` names. Raw uploads that were never
  processed need a bucket lifecycle rule on `banners/raw/`.

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
