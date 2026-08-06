---
module: subscription
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@backend-team"
schema: billing
tables: [plans, entitlements, subscriptions, subscription_events]
depends_on: [payment, user, notification, cache, job, audit]
depended_on_by: [writing, speaking, vocabulary, exam, ai, admin, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# subscription — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Entitlement check on an AI feature



```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant W as writing
    participant S as subscription
    participant C as Redis
    participant AI as platform/ai

    U->>W: POST /writing/submissions
    W->>S: Entitlement(user, "ai.writing.daily")
    S->>C: cached entitlements?
    alt miss
        S->>S: resolve from subscription + plan
        S->>C: cache 60s
    end
    S-->>W: { value: 8 }
    W->>AI: check today's usage against 8
    alt over the limit
        W-->>U: 429 AI_QUOTA_EXCEEDED with an upgrade prompt
    else within
        W->>W: proceed with submission
    end
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
## State machine

Grace exists so that a card expiring does not immediately take away a paying learner's access. Every transition writes a `subscription_events` row.

```mermaid
stateDiagram-v2
    [*] --> Free
    Free --> Trialing: start a trial
    Trialing --> Active: converts with consent
    Trialing --> Free: trial ends without conversion
    Free --> Active: subscribe
    Active --> Active: renewal succeeds
    Active --> Grace: renewal payment fails
    Grace --> Active: payment recovered
    Grace --> Free: grace expires
    Active --> PendingCancel: cancel at period end
    PendingCancel --> Active: reactivate
    PendingCancel --> Free: period ends
    Active --> Free: refunded in full
```

<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
