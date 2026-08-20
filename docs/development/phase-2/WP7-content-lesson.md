---
doc_type: work_package
phase: 2
work_package: WP7
title: "content and lesson — items, versions, authoring, courses"
tasks: 5
estimate: "~11 days"
blocked_by: nothing
status: ready
last_verified: 2026-08-20
---

# WP7 — `content` + `lesson`

Read [`/docs/development/phase-2-plan.md`](../phase-2-plan.md) §2 first.

Both modules are **PLANNED** and contain only documentation and a `doc.go`. Their
`AGENT.md` files already specify the schema, the contract and the endpoints — read them and
implement what is there. Do not invent a table, an endpoint or a config key that is not in
`AGENT.md`, `api/openapi/openapi.yaml`, a migration, or
`docs/deployment/configuration.md`.

| Task | Branch |
|---|---|
| P7.1 | `feat/content-lesson-contracts` |
| P7.2 | `feat/content-schema` |
| P7.3 | `feat/content-module` |
| P7.4 | `feat/lesson-module` |
| P7.5 | `feat/lesson-learner-endpoints` |

**Required reading, in this order:** `internal/modules/content/AGENT.md`,
`internal/modules/lesson/AGENT.md`, `docs/adr/ADR-0015-content-exercise-core.md`,
`/DATABASE_GUIDELINE.md`, `/API_GUIDELINE.md`.

---

## P7.1 — Contracts and OpenAPI, no implementation `S`

| | |
|---|---|
| **Depends on** | — |
| **Context** | The plan §2.2. This is a **release valve**: it unblocks P10.2 immediately. |
| **Files** | `internal/modules/content/contract/`, `internal/modules/lesson/contract/`, `api/openapi/openapi.yaml`, `api/openapi/components/` |
| **Do** | Write the Go contract interfaces and DTOs exactly as `AGENT.md` §4 specifies: `content.Reader` with `GetVersion`, `GetManyVersions`, `Browse` — batched, because lesson rendering resolves many versions at once and an N+1 here is felt on every lesson open. Then author the OpenAPI paths under tags `lesson` and `content`: `GET /courses`, `GET /courses/{slug}`, `GET /lessons/{id}`, and the two admin authoring paths. Schemas carry the authoring state enum (`draft`, `in_review`, `approved`, `published`, `archived`) and CEFR levels. |
| **Acceptance** | `make gen` produces Go server types and `pnpm gen:api` produces TS types, both clean. `go build ./...` passes with no handler yet. `make check` green. A frontend agent can write a typed MSW handler for `GET /courses/{slug}` from this commit alone. |
| **Trap** | A lesson references a **content version**, never a mutable item. If the DTO exposes `content_item_id` where a learner-facing response is concerned, the immutability guarantee is gone before the first row exists. Model the response around the version. |

## P7.2 — `content` schema `M`

| | |
|---|---|
| **Depends on** | P7.1 |
| **Context** | `content/AGENT.md` §5, `/DATABASE_GUIDELINE.md` |
| **Files** | `db/migrations/content/`, `db/queries/content/` |
| **Do** | Create schema `content` with `content_items`, `content_versions`, `media_assets`, `taxonomies`, `content_tags`, `content_reviews` — the six tables the module's frontmatter declares, no more. `content_versions` rows are **immutable after publish**: enforce it with a trigger or a check, not with a comment. Taxonomy covers topic, skill, CEFR level, exam relevance and free tags. |
| **Acceptance** | Migration is reversible. Every FK is indexed. An `UPDATE` on a published version row is rejected by the database, proven by an integration test. `sqlc generate` is clean and `make gen` is reproducible. |
| **Trap** | `learn` and `skill` schemas belong to other modules (rule DB1). Nothing in this migration touches them. |

## P7.3 — `content` module: versioning and the authoring workflow `L`

| | |
|---|---|
| **Depends on** | P7.2 |
| **Context** | `content/AGENT.md` §1–2, `/ERROR_HANDLING.md` |
| **Files** | `internal/modules/content/{domain,service,repository,transport/http}/`, `module.go` |
| **Do** | Implement the state machine draft → in_review → approved → published → archived, in `domain/` as pure transitions with an explicit table of legal moves. Publishing creates an immutable version and emits the event other modules listen for. Implement `content.Reader` including the batched `GetManyVersions`. Wire admin authoring endpoints behind the `content.create` / `content.edit` permissions that `rbac` already seeds. |
| **Acceptance** | Every illegal transition returns a documented `apperr` code, not a 500. Publishing twice is idempotent. `GetManyVersions` issues **one** query for N ids — assert the query count in a test, do not eyeball it. Coverage ≥ 85 %. |
| **Tests** | Table-driven over all 25 transition pairs. Integration test for the batched read. |
| **Trap** | **Decide the archive-mid-session question here** (review §F3). A learner who opened a lesson yesterday holds a version id. Archiving the item must not 404 that learner. The version stays readable; only *discovery* stops. Write the test that proves it, and record the decision in `content/DECISIONS.md`. |

## P7.4 — `lesson` schema and module `L`

| | |
|---|---|
| **Depends on** | P7.3 |
| **Context** | `lesson/AGENT.md`, ADR-0015 |
| **Files** | `db/migrations/lesson/`, `db/queries/lesson/`, `internal/modules/lesson/**` |
| **Do** | Schema `learn`, tables `courses`, `course_units`, `lessons`, `activities`, `lesson_prerequisites`. An `activity` names a **grader kind** and a **content version** — that pairing is the whole hinge of ADR-0015, and Phase 3 adds five more grader kinds against it without touching this table. Implement unlocking from `lesson_prerequisites` via `learning.UnlockChecker`, and make the locked reason a **string the API returns** ("finish Unit 2 first"), not a boolean the UI has to invent copy for. |
| **Acceptance** | A course with units, lessons and activities can be read in one round trip per lesson, with content versions resolved through the batched reader. A locked lesson returns its reason. Prerequisite cycles are rejected at write time. |
| **Trap** | `lesson` depends on `content` and `cache` only. It must not import `learning` — the arrow goes the other way, and `UnlockChecker` is `learning`'s interface that `lesson` calls, per `learning/AGENT.md` §4. `go-arch-lint` will catch it; do not spend an hour discovering why. |

## P7.5 — Learner-facing read endpoints `M`

| | |
|---|---|
| **Depends on** | P7.4 |
| **Context** | `lesson/AGENT.md` §6, `/API_GUIDELINE.md` |
| **Files** | `internal/modules/lesson/transport/http/`, `api/openapi/openapi.yaml` |
| **Do** | Implement `GET /api/v1/courses`, `GET /api/v1/courses/{slug}`, `GET /api/v1/lessons/{id}` against the spec written in P7.1. Permission `content.read.published` — a learner sees published versions only, and the query enforces it rather than the handler filtering after the fact. Cache the course tree; invalidate on publish. |
| **Acceptance** | The spec is unchanged by this commit — if a handler needs a field the spec lacks, the spec changes **first**, in its own commit. An unpublished item is invisible to a learner even with a direct id. p95 under 100 ms for a course with 8 lessons, measured against the `make dev` stack. |
| **Trap** | Filtering in Go after selecting everything is the version of this bug that passes tests and leaks in production. Put `status = 'published'` in the SQL. |

---

## Work-package gate

- An author drives a content item draft → published entirely through the API
- A published lesson resolves to an immutable version, and archiving does not break a
  learner who is mid-lesson
- A locked lesson returns a human-readable reason
- `make check` green, coverage ≥ 85 % on `content`
- `docs/development/phase-2/README.md` handoff note updated with anything the next WP needs
