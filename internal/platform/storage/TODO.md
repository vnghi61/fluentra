---
module: storage
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry, job]
depended_on_by: [user, content, media, speaking, writing, analytics, audit]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# storage — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] Bucket definitions, policies and lifecycle rules
- [ ] Presign PUT/GET with pinned constraints
- [ ] Post-upload verification helper
- [ ] Deterministic key builder
- [ ] Orphan GC job
- [ ] Per-bucket metrics and a disk alert
- [ ] Purge handler for `user.deleted`
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- CDN in front of public buckets
- Multi-node MinIO with erasure coding
- Virus scanning for publicly visible uploads
<!-- END GENERATED: todo-future -->
