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

## Progress

The list above is generated from `tools/docgen/data/learning.json`, so its checkboxes cannot be
ticked by hand — `make docs` rewrites the block and `make docs-check` fails until it matches.
Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P7.4 | 2026-08-23 | Schema `learn` with the five tables, `activities.content_version_id` a bare uuid because DB4 forbids the cross-schema foreign key, and an integration test that asserts DB4 rather than assuming it; prerequisite DAG with cycle detection over all seven graph shapes; `UnlockChecker` declared in `service` so `lesson` does not depend on `learning`, nil-safe to unlocked until WP8; lock reason a sentence naming every prerequisite, carried to the client on the 403 through `Problem.meta`; activity content resolved through one `content.Reader.GetManyVersions` call, proven by a counting reader that also asserts `GetVersion` was never called; every learner query published-only in SQL, with the lesson read joining up to the course; `GET /courses` implements the `level` filter the spec declares and its page size bounded in the spec and clamped in the domain. Coverage **86.1%**, measured with `-coverpkg` against postgres 17 |

## Carried into P7.5 / P11.1

- [ ] **Units and lessons have no creation endpoint, and no lesson publish endpoint.**
      The spec gives Phase 2 two admin routes for this module — `POST /admin/courses` and
      `PUT /admin/lessons/{id}/activities` — so `repository.CreateUnit`, `CreateLesson` and
      `UpdateLesson` are reachable only from the integration test, and `service.PublishLesson`
      has no caller at all. A course can therefore be created through the API but not filled
      in. P11.1's seed needs exactly these; either it calls the repository directly, or the
      spec gains the routes first and the service grows the methods. Decide before the seed
      is written, not during.
- [ ] **`IsUnlocked` is called once per lesson.** `GetCourseDetail` evaluates the lock for
      every lesson in the course, so a forty-lesson course is forty calls into `learning`.
      The interface is single-lesson because `learning/AGENT.md` §4 documents it that way;
      P8.1 owns that contract and should decide whether it takes a batch of lesson ids, the
      way `content.Reader.GetManyVersions` does. Changing it here would have meant editing
      another module's documented interface from this side.
- [ ] **Caching.** `AGENT.md` §12 lists `lesson:detail:{id}` and `lesson:catalogue:{hash}`
      with their invalidation events. P7.5 owns both, along with invalidation on publish.

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
