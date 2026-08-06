---
module: vocabulary
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: skill
tables: [words, word_senses, word_relations, decks, deck_items, user_word_state]
depends_on: [content, srs, media, ai, search]
depended_on_by: [learning, reading, writing, grammar]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# vocabulary — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Learning a word end to end



```mermaid
flowchart LR
    A[Learner meets a word<br/>in a reading passage] --> B[taps for a gloss]
    B --> C["vocabulary.LookupWord"]
    C --> D[add to deck]
    D --> E[user_word_state = learning]
    E --> F[appears in the next lesson's<br/>vocabulary activity]
    F --> G[graded by vocabulary.Grader]
    G --> H["GradeResult.ReviewItems → srs"]
    H --> I[review card created]
    I --> J[due tomorrow, then in 3 days, then 8…]
    J --> K[status becomes known<br/>once stability passes the threshold]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
