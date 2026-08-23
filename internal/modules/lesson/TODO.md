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
| P7.5 | 2026-08-23 | Lesson publish endpoint `POST /admin/lessons/{id}/publish` guarded by `content.publish`, enforcing BR-LESSON-02 (409 when an activity points at a draft or archived content version) and a non-empty activity list (422), idempotent on an already published lesson, and emitting the `lesson.published` outbox event. Duration recalculation stays where it was, on the activities write. Typed caching for course tree (`GetCourseDetail`), lesson detail (`GetLessonDetail`), and catalogue (`ListCourses`) using `platform/cache.Cache[T]` with singleflight, generation counter for catalogue keys avoiding wildcards (Trap 1), synchronous post-commit invalidation with a `lesson.published` worker consumer as the backstop (Trap 2), a cache outage degrading to the loader rather than to a 500 (Trap 3), and reverse-lookup detail invalidation on the `content.archived` worker event (Part C). Proved against a real Redis and a query-counting `pgx` tracer in `cache_integration_test.go`; `latency_integration_test.go` measures the learner reads on an eight-lesson course (opt-in, `LESSON_LATENCY=1`). `activities` is bounded in the spec and in the domain (at most 100 activities, weight 0..100), which is what makes the int32 `estimated_minutes` safe. Coverage **90.3%** on `internal/modules/lesson/...`. |

## Carried into P8.1 / P11.1

- [ ] **`IsUnlocked` is called once per lesson.** `GetCourseDetail` evaluates the lock for
      every lesson in the course, so a forty-lesson course is forty calls into `learning`.
      The interface is single-lesson because `learning/AGENT.md` §4 documents it that way;
      P8.1 owns that contract and should decide whether it takes a batch of lesson ids, the
      way `content.Reader.GetManyVersions` does. Changing it here would have meant editing
      another module's documented interface from this side.

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
