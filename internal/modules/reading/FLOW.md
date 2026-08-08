---
module: reading
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [passages, passage_questions, reading_attempts]
depends_on: [content, questionbank, vocabulary, learning]
depended_on_by: [learning, exam, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# reading — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Timed reading attempt

```mermaid
flowchart LR
    A[Start attempt] --> B[render passage, start timer]
    B --> C{learner taps a word}
    C -->|yes| D[gloss via vocabulary<br/>record the lookup]
    D --> B
    C -->|no| E[learner marks reading complete]
    E --> F[stop timer → wpm]
    F --> G[reveal questions]
    G --> H[submit answers]
    H --> I[grade per question type]
    I --> J[comprehension score + wpm → progress]
    J --> K[looked-up words suggested for the deck]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
