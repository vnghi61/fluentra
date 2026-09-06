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

# job — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| River, Asynq, or a broker? | River | Transactional enqueue is worth more to us than raw throughput; see the comparison in ARCHITECTURE §11.1 and ADR-0010 |
| Job args: IDs or full payloads? | IDs | A payload captured at enqueue time is stale by the time the job runs; re-reading is correct and keeps the queue table small |
| One queue or several? | Five, by workload shape | A three-minute transcode must not delay a notification; separate queues give independent concurrency and independent alerting |
| Run cron jobs once at startup? | Yes, before the ticker | A fresh deployment otherwise waits out a whole interval doing nothing, which is an hour for upload verification. The advisory lock still stops two instances colliding, and the orchestrator's restart backoff bounds how often a crash-loop can repeat the call |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0010](../../../docs/adr/ADR-0010-job-queue-river.md) — River (Postgres) for background jobs
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
