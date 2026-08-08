---
module: search
tier: platform
group: platform
status: PLANNED
phase: 4
owner: "@platform-team"
schema: none
tables: []
depends_on: [cache, job]
depended_on_by: [content, vocabulary, questionbank, lesson, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# search — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Index maintenance

```mermaid
flowchart LR
    A[content.published] --> B[search job]
    B --> C[build tsvector from title, body, tags]
    C --> D[UPSERT into the module's search table]
    D --> E[GIN index]
    F[Learner query] --> G[normalise + tokenise]
    G --> H["websearch_to_tsquery + ts_rank"]
    H --> I[filter by level, skill, status]
    I --> J[highlight + paginate]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
