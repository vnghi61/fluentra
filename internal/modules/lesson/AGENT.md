---
module: lesson
tier: learning
group: modules
status: READY
phase: 2
owner: "@learning-team"
schema: learn
tables: [courses, course_units, lessons, activities, lesson_prerequisites]
depends_on: [content, cache]
depended_on_by: [learning, admin, search]
spec_version: 1.0.0
last_verified: 2026-08-23
---

# lesson — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/lesson` |
| Schema | `learn` |
| Delivery phase | 2 |
| Status | **READY** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Structure: courses contain units, units contain lessons, lessons contain activities, and activities point at content versions. Owns prerequisites, ordering and unlocking rules.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Course, unit, lesson and activity structure and ordering
- Activity → content version binding
- Prerequisites and unlocking rules
- Estimated duration and skill/level metadata
- Course catalogue and lesson detail queries
- Draft/published state for structure, mirroring content

**This module does NOT own:**

- Learner progress through the structure — that is `learning`
- The material itself — that is `content`
- Grading — that is the exercise engine plus the skill graders
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/lesson/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/lesson/contract/` | You are calling this module from another module |
| `internal/modules/lesson/service/` | You are changing behaviour |
| `db/migrations/lesson/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/lesson/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `lesson.Reader` | `GetLesson`, `ListLessons`, `NextLesson` — used heavily by `learning` |
| struct | `lesson.Activity` | `{ID, Kind, ContentVersionID, Config, Weight}` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `lesson.published` | publishes | `{lesson_id, course_id, skill_focus}` |
| `content.archived` | consumes | Flag lessons whose content was archived |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `learn` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/lesson/` · Queries: `db/queries/lesson/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `learn.courses` | Top-level container | `slug` UNIQUE, `cefr_from`, `cefr_to`, `status`, `estimated_hours` |
| `learn.course_units` | Group of lessons | `course_id`, `position`, `title` |
| `learn.lessons` | Schedulable learning unit | `unit_id`, `position`, `skill_focus`, `estimated_minutes`, `status` |
| `learn.activities` | One thing the learner does | `lesson_id`, `position`, `kind`, `content_version_id`, `config` jsonb, `weight` |
| `learn.lesson_prerequisites` | Unlocking graph | `lesson_id`, `requires_lesson_id`, `min_score` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `lesson`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/courses` | `content.read.published` | Catalogue with level filters |
| `GET` | `/api/v1/courses/{slug}` | `content.read.published` | Course with units and lesson summaries |
| `GET` | `/api/v1/lessons/{id}` | `content.read.published` | Lesson with its activities and resolved content |
| `POST` | `/api/v1/admin/courses` | `content.create` | Create a course |
| `PUT` | `/api/v1/admin/lessons/{id}/activities` | `content.edit` | Reorder or replace the activity list |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`content`](../../modules/content/AGENT.md) | → depends on | see its contract |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
| [`search`](../../platform/search/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-LESSON-01** — An activity binds to a **content version**, not a content item — republishing content does not silently change a lesson a learner is halfway through.
2. **BR-LESSON-02** — A lesson cannot be published while any activity points at unpublished content.
3. **BR-LESSON-03** — Prerequisites form a directed acyclic graph; a cycle is rejected at write time.
4. **BR-LESSON-04** — Reordering activities does not affect in-flight attempts; those reference the activity ID.
5. **BR-LESSON-05** — A published lesson's activity list is versioned — changing it creates a new lesson version rather than mutating the live one.
6. **BR-LESSON-06** — Estimated duration is the sum of activity estimates and is recalculated on change, because it drives the learner's daily plan.
7. **BR-LESSON-07** — Unlocking is evaluated by `learning`, using rules this module defines — the rule lives here, the learner state lives there.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add an activity kind

1. Register the kind and its `config` schema.
2. Ensure the corresponding content kind exists in `content`.
3. Implement the grader in the owning skill module (it implements `learning.ExerciseGrader`).
4. Add the React renderer in `web/src/features/lesson/activities/`.
5. Add a fixture lesson containing the new activity and an E2E path through it.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Structure is linear with prerequisites; there is no branching or conditional path in v1.
- No per-learner lesson variants.
- Duration estimates are static rather than learned from real completion times.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:lesson:detail:{lesson_id}:v1` | 1 h | `lesson.published`, structure edit |
| `fluentra:{env}:lesson:catalogue:{filter_hash}:v1` | 15 min | Any publish |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `LESSON_LOCKED` | 403 | Prerequisites not met |
| `PREREQUISITE_CYCLE` | 422 | The proposed graph contains a cycle |
| `ACTIVITY_CONTENT_UNPUBLISHED` | 409 | Cannot publish a lesson pointing at draft content |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/lesson/...                    # unit
go test -tags=integration ./internal/modules/lesson/...  # integration (testcontainers)
```

**Focus areas**

- Prerequisite cycle detection
- Lesson publish blocked on draft content
- Activity content resolution is batched (assert the query count)
- Reordering does not disturb in-flight attempts
- Locked lesson returns 403 naming the missing prerequisite
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not bind an activity to a content item.
- Do not resolve content one activity at a time.
- Do not store learner progress here.
- Do not publish a lesson whose content is not published.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
