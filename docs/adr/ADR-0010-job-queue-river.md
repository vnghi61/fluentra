---
adr: 0010
title: "River (Postgres-backed) for background jobs"
status: Accepted
date: 2026-08-06
tags: [backend]
---

# ADR-0010: River (Postgres-backed) for background jobs

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | backend |

## Context

AI grading, media processing, emails and scheduled work must run outside the request path, reliably, with retries — and must never exist for data that was rolled back.

## Decision

Use `riverqueue/river`, which stores jobs in PostgreSQL, so a job can be enqueued inside the business transaction. Five queues by workload shape: `ai`, `media`, `notify`, `batch`, `default`.

## Alternatives considered

### A. Asynq (Redis)

| | |
|---|---|
| **Pros** | Mature, fast, good tooling |
| **Cons** | Enqueue cannot participate in the database transaction — you must choose between orphan jobs and lost jobs, or build an outbox for jobs too |
| **Why rejected** | The transactional property is the whole point. |

### B. NATS JetStream

| | |
|---|---|
| **Pros** | High throughput; ready for a distributed future |
| **Cons** | New infrastructure; same transactional gap |
| **Why rejected** | Deferred until extraction. |

### C. Temporal

| | |
|---|---|
| **Pros** | Excellent for long multi-step workflows |
| **Cons** | A cluster to operate; a large programming-model shift |
| **Why rejected** | Our workflows are short and simple; the cost is not repaid. |

## Consequences

### Positive

- Transactional enqueue: no orphaned jobs, no lost work
- No additional infrastructure
- Unique jobs, periodic jobs, retries and a web UI included
- Job state is queryable with SQL, which makes operational tooling trivial

### Negative — accepted knowingly

- Throughput bounded by Postgres (thousands/second — far above our need)
- Adds write load to the primary database
- A younger project than the Redis alternatives

## Compliance

Enqueueing after commit fails review. Every job handler has a test that runs it twice and asserts one effect.

## Revisit when

If sustained job throughput approaches the low thousands per second, or when a module is extracted and needs cross-service work distribution.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
