---
module: listening
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [audio_items, transcripts, listening_attempts]
depends_on: [content, media, questionbank, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# listening — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Exam-mode listening

```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant L as listening
    participant ST as storage

    U->>L: POST /listening/attempts { item_id }
    L->>L: INSERT attempt (plays_used = 0)
    L->>ST: PresignGet (attempt-scoped, 15 min)
    L-->>U: { attempt_id, audio_url, plays_allowed: 1 }
    U->>L: POST /attempts/{id}/play
    L->>L: plays_used = 1
    L-->>U: { plays_remaining: 0 }
    U->>L: POST /attempts/{id}/play
    L-->>U: 403 PLAY_LIMIT_REACHED
    U->>L: POST /attempts/{id}/submit
    L->>L: grade; unlock the transcript
    L-->>U: score + transcript
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
