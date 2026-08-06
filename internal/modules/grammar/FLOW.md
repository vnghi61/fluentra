---
module: grammar
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [grammar_points, grammar_rules, grammar_exercises, error_tags, user_grammar_state]
depends_on: [content, srs, ai, learning]
depended_on_by: [writing, speaking, learning, exam]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# grammar — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## From an essay error to a targeted drill



```mermaid
flowchart LR
    A[writing.graded<br/>with annotations] --> B["grammar.TagErrors"]
    B --> C[map each annotation to a taxonomy point]
    C --> D[INSERT error_tags]
    D --> E[update user_grammar_state error rate]
    E --> F[weakness profile reranked]
    F --> G[drills recommended, respecting prerequisites]
    G --> H[learner completes a drill]
    H --> I[grammar point enters srs]
    I --> J[error rate falls → point drops down the ranking]
```

<!-- END GENERATED: flows -->

<!-- BEGIN GENERATED: states -->
_This module has no explicit state machine._
<!-- END GENERATED: states -->

## Failure paths

<!-- BEGIN GENERATED: failures -->
_None yet._
<!-- END GENERATED: failures -->
