---
module: cache
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: none
tables: []
depends_on: [telemetry]
depended_on_by: [auth, rbac, user, content, lesson, learning, srs, gamification, ai, notification, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# cache — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Cache-aside with single-flight and degradation



```mermaid
flowchart TD
    A["GetOrLoad(key, loader)"] --> B{Redis reachable?}
    B -->|no| L[call loader directly<br/>warn + cache_unavailable_total]
    B -->|yes| C[GET key]
    C -->|hit| D[deserialise → return<br/>cache_requests_total result=hit]
    C -->|miss| E[single-flight per key]
    E --> F[loader]
    F -->|error| G[return error, cache nothing]
    F -->|value| H[SET with jittered TTL]
    H --> I[return value<br/>result=miss]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
