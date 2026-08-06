---
module: job
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: ops
tables: [river_job, outbox_events, job_failures]
depends_on: [telemetry]
depended_on_by: [auth, user, audit, notification, mailer, content, writing, speaking, media, ai, srs, analytics, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# job — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 1

- [ ] River client and worker wiring with the five queues
- [ ] Job middleware: span, log, recover, timeout, metrics
- [ ] Outbox table and publisher loop with `SKIP LOCKED`
- [ ] Cron scheduler with advisory locking
- [ ] Dead-letter recording and the `job.failed_permanently` event
- [ ] Admin inspect/retry/cancel endpoints
- [ ] Queue depth and oldest-pending-age metrics with alerts
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Per-queue autoscaling hints
- Job dependency graphs for multi-stage pipelines
- Broker-backed queue after a module is extracted
<!-- END GENERATED: todo-future -->
