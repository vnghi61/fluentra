---
module: mailer
tier: platform
group: platform
status: PLANNED
phase: 1
owner: "@platform-team"
schema: comm
tables: [email_log, email_suppressions]
depends_on: [job, telemetry, storage]
depended_on_by: [auth, user, notification, subscription, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# mailer — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Send with retry and suppression

```mermaid
flowchart TD
    A["Sender.Send(msg)"] --> B[enqueue email.send job]
    B --> C{suppressed?}
    C -->|yes, non-security| D[skip, log reason]
    C -->|no| E[render template + locale]
    E -->|template missing| F[fail loudly — this is a bug, not a runtime condition]
    E --> G[provider send]
    G -->|transient| H{attempts left?}
    H -->|yes| G
    H -->|no| I[log failed; alert if the category is critical]
    G -->|hard bounce| J[suppress address; publish email.bounced]
    G -->|accepted| K[log sent with provider message id]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
