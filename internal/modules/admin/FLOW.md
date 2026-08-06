---
module: admin
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: core
tables: [feature_flags, admin_notes, moderation_items]
depends_on: [user, rbac, audit, auth, content, cache]
depended_on_by: []
spec_version: 1.0.0
last_verified: 2026-08-06
---

# admin — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Impersonation



```mermaid
sequenceDiagram
    autonumber
    actor AD as Admin
    participant A as admin
    participant AU as auth
    participant AD2 as audit

    AD->>A: POST /admin/impersonate/{user_id} { reason }
    A->>A: require permission user.impersonate
    A->>A: reject if target is an admin
    A->>AU: IssueImpersonationToken(admin_id, user_id, 30m)
    AU-->>A: token with act claim
    A->>AD2: audit(impersonation_started, reason)
    A-->>AD: token + banner flag
    Note over AD: every subsequent request records both identities
    Note over AD: payment, deletion and role changes are refused
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
