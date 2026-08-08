---
module: srs
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [review_cards, review_logs, srs_params, review_daily_stats]
depends_on: [cache, job, content]
depended_on_by: [learning, vocabulary, grammar, gamification, notification, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# srs — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Review session

```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant S as srs
    participant C as Redis
    participant DB as PostgreSQL
    participant CO as content

    U->>S: GET /reviews/session?limit=20
    S->>DB: SELECT due cards ORDER BY due_at, id LIMIT 20
    S->>S: apply daily limits, interleave skills, avoid family clustering
    S->>CO: GetManyVersions (batched)
    S-->>U: cards with content and prefetch hints

    loop each card
        U->>S: POST /reviews/{card}/answer { grade, elapsed_ms }
        S->>S: FSRS: update stability, difficulty, next due
        S->>DB: BEGIN; UPDATE review_cards; INSERT review_logs
        S->>DB: INSERT outbox(review.card_answered); COMMIT
        S->>C: DEL due_count
        S-->>U: { next_due_at, interval_days }
    end

    U->>S: POST /reviews/session/complete
    S->>DB: INSERT outbox(review.session_completed)
    Note over DB: gamification awards XP and updates the streak
    S-->>U: { reviewed, accuracy, next_session_at }
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
## State machine

FSRS card states. `Relearning` is the path back after a lapse — it is not the same as `Learning`, because prior stability is partially retained.

```mermaid
stateDiagram-v2
    [*] --> New: card created by a grader
    New --> Learning: first answer
    Learning --> Review: graduates
    Learning --> Learning: again/hard within the learning steps
    Review --> Review: good/easy → longer interval
    Review --> Relearning: again (lapse)
    Relearning --> Review: graduates again
    Review --> Suspended: learner suspends
    Relearning --> Suspended: learner suspends
    Suspended --> Review: unsuspend
    Review --> New: reset
```

<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
