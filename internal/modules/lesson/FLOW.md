---
module: lesson
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [courses, course_units, lessons, activities, lesson_prerequisites]
depends_on: [content, cache]
depended_on_by: [learning, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# lesson — Flows

Sequence diagrams, state machines and business processes owned by this module.

<!-- BEGIN GENERATED: flows -->
## Rendering a lesson

```mermaid
sequenceDiagram
    autonumber
    actor U as Learner
    participant L as lesson
    participant LR as learning
    participant C as content
    participant CA as cache

    U->>L: GET /lessons/{id}
    L->>LR: IsUnlocked(user, lesson)
    alt locked
        L-->>U: 403 LESSON_LOCKED + which prerequisite
    else unlocked
        L->>CA: lesson detail cached?
        alt miss
            L->>L: load lesson + activities
            L->>C: GetManyVersions(activity content ids)  // batched, never N+1
            L->>CA: store
        end
        L->>LR: progress for this lesson
        L-->>U: lesson + activities + resume point
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
