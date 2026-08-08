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

- [x] River client and worker wiring with the five queues
- [x] Job middleware: span, log, recover, timeout, metrics
- [x] Outbox table and publisher loop with `SKIP LOCKED`
- [x] Cron scheduler with advisory locking
- [~] Dead-letter recording and the `job.failed_permanently` event
- [ ] Admin inspect/retry/cancel endpoints
- [~] Queue depth and oldest-pending-age metrics with alerts
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

### Carried over from P0.R6 — the two `[~]` items above are half done

- **`job.failed_permanently` event.** `ops.job_failures` is written on the attempt that exhausts
  the budget (BR-JOB-08), verified by integration test. The *event* half is not published: doing it
  from River's `ErrorHandler` would need an outbox write outside the job's transaction, which is the
  one place the outbox contract does not fit. Decide the shape when the first consumer exists.
- **`job_queue_depth`.** `job_oldest_pending_seconds` now has a callback and reports real values.
  `job_queue_depth` is still declared in `telemetry.Instruments` with nothing writing to it. It is
  an `Int64UpDownCounter`, which is the wrong instrument for a value read from a table — it wants
  to be an observable gauge alongside the age one. Changing its type is a telemetry change, not a
  job change, so it was left alone rather than half-wired here.
- **Alerts.** The metrics exist; no alert rule consumes them yet. That belongs with the Grafana
  provisioning in P0.R15.

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Per-queue autoscaling hints
- Job dependency graphs for multi-stage pipelines
- Broker-backed queue after a module is extracted
<!-- END GENERATED: todo-future -->
