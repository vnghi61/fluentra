---
module: telemetry
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: []
depended_on_by: [ai, cache, storage, job, media, search, mailer]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# telemetry — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Correlation across a request and its background work



```mermaid
flowchart LR
    A[Browser click<br/>OTel Web SDK] -->|traceparent + X-Request-Id| B[API middleware]
    B --> C[root span]
    C --> D[slog records carry trace_id]
    C --> E[service span]
    E --> F[pgx span]
    E --> G[job enqueued with job.Meta<br/>trace_id + request_id]
    G --> H[worker span links to the same trace]
    H --> I[ai span<br/>provider, model, cost]
    D & F & I --> J[Grafana: log → trace → span → exemplar]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
