---
module: gamification
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: learn
tables: [xp_events, streaks, badges, badges_earned, quests, user_quests, leaderboard_snapshots]
depends_on: [learning, srs, cache, job, notification]
depended_on_by: [notification, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# gamification — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Streak evaluation at the day boundary



```mermaid
flowchart TD
    A[Nightly job, per timezone bucket] --> B{daily goal met yesterday?}
    B -->|yes| C[current += 1; longest = max]
    B -->|no| D{freeze available?}
    D -->|yes| E[consume a freeze; streak preserved<br/>notify gently]
    D -->|no| F[streak broken → 0<br/>publish streak_broken]
    C --> G[check badge conditions]
    E --> G
    F --> G
    G --> H[award any newly earned badges, idempotently]
    H --> I[bust the summary cache]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
