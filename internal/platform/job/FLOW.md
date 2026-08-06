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

# job — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Transactional enqueue and the outbox



```mermaid
sequenceDiagram
    autonumber
    participant S as Service
    participant DB as PostgreSQL
    participant OB as Outbox publisher
    participant W as Worker

    S->>DB: BEGIN
    S->>DB: INSERT business row
    S->>DB: INSERT river_job (same tx)
    S->>DB: INSERT outbox_events (same tx)
    S->>DB: COMMIT
    Note over S,DB: all three commit atomically — no orphans, no lost work
    W->>DB: dequeue job (FOR UPDATE SKIP LOCKED)
    W->>W: run handler inside span + timeout
    alt transient failure
        W->>DB: reschedule with exponential backoff
    else business failure
        W->>DB: INSERT job_failures; publish job.failed_permanently
    else success
        W->>DB: mark completed
    end
    OB->>DB: poll unpublished outbox rows
    OB->>OB: dispatch to in-process subscribers
    OB->>DB: mark published
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
