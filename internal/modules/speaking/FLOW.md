---
module: speaking
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_tasks, speaking_attempts, pronunciation_scores, speaking_feedback]
depends_on: [media, ai, storage, job, content, learning]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# speaking — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Record to score

```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant S as speaking
    participant ST as storage
    participant M as media
    participant AI as platform/ai

    U->>S: POST /speaking/upload-intent
    S->>S: consent check, quota check
    S->>ST: PresignPut (5 min, type + size pinned)
    S-->>U: { upload_url, asset_id }
    U->>ST: PUT audio directly
    U->>S: POST /speaking/attempts { asset_id, task_id }
    S->>ST: Stat — verify size and type
    S->>S: BEGIN; INSERT attempt(status=processing); enqueue job; COMMIT
    S-->>U: 202 { attempt_id }

    M->>M: transcode → 16 kHz mono
    M->>M: ASR → transcript + word timings
    alt low confidence
        M->>S: fail with TRANSCRIPTION_LOW_CONFIDENCE
        S-->>U: ask to re-record; no quota charged
    else
        M->>M: pronunciation assessment
        M->>S: media.transcribed + scores
        S->>AI: Run(speaking.feedback, { transcript, scores, task, level })
        AI-->>S: structured coaching
        S->>S: clamp + persist; publish speaking.scored
        S-->>U: attempt ready (poll or SSE)
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
