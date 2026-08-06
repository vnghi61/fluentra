---
module: audit
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: audit
tables: [audit_logs, security_events]
depends_on: [job]
depended_on_by: [auth, user, rbac, admin, content, questionbank, exam, payment]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# audit — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Recording an admin action



```mermaid
sequenceDiagram
    autonumber
    participant AD as admin handler
    participant SV as content.Service
    participant DB as PostgreSQL
    participant OB as outbox publisher
    participant AU as audit

    AD->>SV: PublishContent(ctx, id)
    SV->>DB: BEGIN
    SV->>DB: UPDATE content_versions SET status='published'
    SV->>DB: INSERT outbox(content.published, before, after)
    SV->>DB: COMMIT
    OB->>AU: deliver event
    AU->>DB: INSERT audit.audit_logs (actor, action, diff, trace_id)
    Note over AU,DB: at-least-once; the handler dedupes on event_id
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
