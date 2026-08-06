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

# lesson — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] Course/unit/lesson/activity model and admin CRUD
- [ ] Prerequisite graph with cycle detection
- [ ] Batched content resolution
- [ ] Catalogue and lesson detail endpoints with caching
- [ ] Publish validation against content state
- [ ] Lesson player shell in the web app
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Branching paths by performance
- Learned duration estimates
- Per-learner lesson variants
- Course cloning for A/B curriculum tests
<!-- END GENERATED: todo-future -->
