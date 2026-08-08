---
module: notification
tier: core
group: modules
status: PLANNED
phase: 2
owner: "@backend-team"
schema: comm
tables: [notifications, notification_preferences, devices, notification_dedupe]
depends_on: [mailer, job, cache, user]
depended_on_by: [auth, writing, speaking, exam, gamification, subscription, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# notification — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Event to delivered notification

```mermaid
sequenceDiagram
    autonumber
    participant W as writing
    participant OB as outbox
    participant N as notification
    participant C as Redis
    participant J as job
    participant M as mailer
    actor U as User

    W->>OB: writing.graded (in the grading transaction)
    OB->>N: deliver event
    N->>N: load preferences + timezone
    alt category disabled on every channel
        N->>N: drop, record reason
    else
        N->>C: check dedupe key
        alt duplicate
            N->>N: suppress
        else
            N->>N: INSERT notifications (in-app is always written)
            N->>C: bust unread count
            alt inside quiet hours
                N->>J: schedule for the next allowed slot
            else
                N->>J: enqueue push + email now
                J->>M: render + send
                M-->>U: email
            end
        end
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
