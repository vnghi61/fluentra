---
module: rbac
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [roles, permissions, role_permissions, user_roles]
depends_on: [cache, audit]
depended_on_by: [auth, admin, content, questionbank, exam, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# rbac — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Three-layer authorization

Each layer catches a different mistake. Middleware catches a route wired to the wrong group; the guard catches a service method called from an unexpected path; the ownership filter catches the IDOR that the other two cannot see.

```mermaid
flowchart TD
    A[Request] --> B[auth middleware<br/>validates token → Actor in ctx]
    B --> C{Route group}
    C -->|/admin/*| D[require role=admin]
    C -->|/api/v1/*| E[authenticated]
    D --> F[Handler]
    E --> F
    F --> G[Service method]
    G --> H["rbac.Require(ctx, 'content.publish')"]
    H -->|denied| X[403 PERMISSION_DENIED<br/>+ audit event]
    H -->|allowed| I[Repository query]
    I --> J["WHERE user_id = actor.UserID<br/>(ownership — the real IDOR defence)"]
    J --> K[Result]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
