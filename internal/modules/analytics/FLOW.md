---
module: analytics
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@data-team"
schema: analytics
tables: [analytics_events, daily_rollups, cohorts, funnel_steps, learner_outcomes]
depends_on: [job, cache]
depended_on_by: [admin, notification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# analytics — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Ingestion to dashboard

```mermaid
flowchart LR
    A[Domain events via outbox] --> B[ingest job<br/>idempotent on event_id]
    B --> C[(analytics_events<br/>partitioned, append-only)]
    C --> D[nightly rollup job]
    D --> E[(daily_rollups)]
    E --> F[KPI / funnel / retention queries]
    F --> G[cache 15 min]
    G --> H[admin dashboard]
    E --> I[learner insights]
    E --> J[weekly progress email]
    K[late or replayed event] --> L[mark the day dirty] --> D
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
