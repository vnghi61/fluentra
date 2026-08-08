---
module: questionbank
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_options, question_sets, question_set_items, question_stats]
depends_on: [content, ai, audit, search]
depended_on_by: [exam, reading, listening, grammar, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## AI-assisted authoring

```mermaid
sequenceDiagram
    autonumber
    actor AD as Admin
    participant Q as questionbank
    participant AI as platform/ai
    participant R as Reviewer

    AD->>Q: POST /admin/questions/generate { topic, level, type, count }
    Q->>Q: check quota + budget
    Q->>AI: Run(questionbank.generate_items)
    AI-->>Q: structured draft items
    Q->>Q: validate schema; reject malformed; dedupe against existing stems
    Q->>Q: INSERT as drafts, marked ai_generated
    Q-->>AD: 202 → drafts appear in the review queue
    R->>Q: review, edit, approve or reject
    Note over R,Q: nothing AI-generated reaches a learner unreviewed
    Q->>Q: publish approved items; statistics begin accumulating
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
