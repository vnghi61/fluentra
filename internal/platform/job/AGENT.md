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
last_verified: 2026-08-08
---

# job — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/job` |
| Schema | `ops` |
| Delivery phase | 1 |
| Status | **PLANNED** |
| Owner | @platform-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Background execution: the River queue wiring, the cron scheduler, job middleware (tracing, logging, panic recovery, timeouts), the dead-letter path, and the operational surface for inspecting and retrying work.
<!-- END GENERATED: overview -->

**Context.** Jobs are enqueued **inside the business transaction**, so a job can never exist for a row that was rolled back, and a row can never exist without its follow-up work. That single property is why River was chosen over a Redis queue (ADR-0010).

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- River client and worker wiring, queue registry, concurrency configuration
- Cron scheduling with distributed locking
- Job middleware: span, structured log, panic recovery, timeout, metrics
- Retry policy and dead-letter handling
- The outbox publisher loop
- Queue depth and age metrics
- Admin operations: inspect, retry, cancel

**This module does NOT own:**

- The business logic inside a job — that lives in the owning module's `job/` package
- Deciding retry semantics for a business failure — the module classifies its own errors
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/job/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/job/contract/` | You are calling this module from another module |
| `internal/platform/job/service/` | You are changing behaviour |
| `db/migrations/job/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

**What actually exists today** (the generated table above describes the target layout):

| File | Read it when |
|---|---|
| `client.go` | You are enqueueing — `Enqueuer`, `EnqueueTx`, the five queue names |
| `worker.go` | You are changing how jobs are consumed — `ParseQueues`, `NewWorker`, the dead-letter handler, `MigrateUp`, the queue-age query |
| `middleware.go` | You are changing what wraps every handler — span, log, panic recovery, timeout, metrics |
| `cron.go` | You are adding a scheduled task |

## 4. Public API (contract)

Other modules may import **only** `internal/platform/job/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `job.Enqueuer` | `Enqueue(ctx, tx, args)` — takes the transaction, which is the whole point |
| interface | `job.Scheduler` | `Register(spec, kind)` for cron entries |
| struct | `job.Meta` | `{TraceID, RequestID, UserID}` carried in every job payload for correlation |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `job.failed_permanently` | publishes | `{kind, job_id, reason}` |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `ops` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/job/` · Queries: `db/queries/job/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `ops.river_job` | River's queue table | Managed by River's own migrations; do not hand-edit. Applied by `job.MigrateUp` at worker start-up, **not** by goose — River versions this schema itself, so it is deliberately absent from `db/migrations/job/` |
| `ops.outbox_events` | Transactional event outbox | `aggregate`, `event`, `payload`, `published_at`, `attempts`. Polled with `FOR UPDATE SKIP LOCKED` |
| `ops.job_failures` | Dead-letter record | `kind`, `args`, `last_error`, `failed_at` — retained 30 days for triage |


<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `job`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/jobs` | `system.jobs` | List jobs by queue and state |
| `POST` | `/api/v1/admin/jobs/{id}/retry` | `system.jobs` | Retry a failed job |
| `POST` | `/api/v1/admin/jobs/{id}/cancel` | `system.jobs` | Cancel a pending job |
| `GET` | `/api/v1/admin/queues` | `system.jobs` | Depth, oldest pending age, throughput per queue |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`telemetry`](../../platform/telemetry/AGENT.md) | → depends on | Every job is a span; every queue is metered |
| [`auth`](../../modules/auth/AGENT.md) | ← used by | consumes this module's contract |
| [`user`](../../modules/user/AGENT.md) | ← used by | consumes this module's contract |
| [`audit`](../../modules/audit/AGENT.md) | ← used by | consumes this module's contract |
| [`notification`](../../modules/notification/AGENT.md) | ← used by | consumes this module's contract |
| [`mailer`](../../platform/mailer/AGENT.md) | ← used by | consumes this module's contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`media`](../../platform/media/AGENT.md) | ← used by | consumes this module's contract |
| [`ai`](../../platform/ai/AGENT.md) | ← used by | consumes this module's contract |
| [`srs`](../../modules/srs/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`payment`](../../modules/payment/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-JOB-01** — Enqueue inside the transaction that creates the data the job will read. Never after commit.
2. **BR-JOB-02** — Every job handler is idempotent — delivery is at-least-once and a retry must be safe.
3. **BR-JOB-03** — Job arguments carry IDs, never payloads. The handler re-reads current state; stale embedded data causes subtle bugs.
4. **BR-JOB-04** — Every job declares a timeout and a maximum attempt count.
5. **BR-JOB-05** — A business rejection is not retried; only transient failures are.
6. **BR-JOB-06** — Every job carries `trace_id` and `request_id` so its logs join the originating request's trace.
7. **BR-JOB-07** — Cron handlers must tolerate double execution — distributed locking reduces the chance but does not eliminate it.
8. **BR-JOB-08** — A permanently failed job writes to `job_failures` and emits an event; it never disappears silently.
9. **BR-JOB-09** — Queues are isolated by workload: `ai`, `media`, `notify`, `batch`, `default`. A slow media job must not starve notifications.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a job kind

1. Define the args struct in the owning module's `job/` package; include only IDs plus `job.Meta`.
2. Implement the worker; make it idempotent and re-read state.
3. Register it in `cmd/worker`.
4. Choose a queue, timeout and attempt count deliberately.
5. Enqueue it inside the business transaction.
6. Add a test that runs the handler twice and asserts the same end state.

### Add a scheduled task

1. Register the cron spec in `cmd/worker`.
2. Make the handler idempotent and safe if it runs twice or is skipped once.
3. Use an advisory lock if it must not run concurrently across replicas.
4. Add a metric for its last successful run, and an alert if it goes stale.
<!-- END GENERATED: tasks -->

### Queue concurrency

Concurrency per queue is read from `WORKER_QUEUES` (`name:concurrency,…`) and parsed by
`job.ParseQueues`. A malformed value fails the worker's boot rather than falling back to a
default — a silent fallback to hardcoded numbers is indistinguishable from the config working.
`DefaultQueues()` is the documented shape, not the runtime source.

### Metrics

| Metric | State |
|---|---|
| `job_duration_seconds` | Recorded by the middleware, labelled `queue`/`kind`/`result` |
| `job_attempts_total` | Recorded by the middleware, same labels |
| `job_oldest_pending_seconds` | Observable gauge; callback registered by `Worker.Start` |
| `job_queue_depth` | **Declared but never written** — see `TODO.md` |

`result` is a closed set: `success`, `error`, `panic`, `timeout`, `cancelled`. Panic and timeout are
separated from a plain error because they call for different responses — one means the handler is
broken, the other that it is too slow.

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Throughput is bounded by Postgres — comfortable to a few thousand jobs per second, far above our projection, but not a stream processor.
- Jobs add write load to the primary database.
- There is no priority within a queue beyond scheduled time; priority is expressed by using a different queue.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->


### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `JOB_NOT_RETRYABLE` | 409 | Admin attempted to retry a job that failed for a business reason |


## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/platform/job/...                    # unit
go test -tags=integration ./internal/platform/job/...  # integration (testcontainers)
```

**Focus areas**

- Rollback of the business transaction leaves no job
- Handler idempotency: running twice yields one effect
- Retry backoff and the maximum-attempt boundary
- Timeout cancels the handler's context
- Panic in a handler is recovered and recorded, not fatal to the worker
- Outbox publishes exactly once per event to each subscriber
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not enqueue after commit.
- Do not put payloads or personal data in job arguments.
- Do not write a job handler that is not safe to run twice.
- Do not use `go func()` in a handler instead of a job.
- Do not run a long job on the `default` queue and starve everything else.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
