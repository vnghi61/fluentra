---
module: content
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: content
tables: [content_items, content_versions, media_assets, taxonomies, content_tags, content_reviews]
depends_on: [storage, search, audit, ai, media]
depended_on_by: [lesson, learning, vocabulary, grammar, reading, listening, speaking, writing, questionbank]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# content — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] Items, versions, taxonomy, tags and media references
- [ ] Authoring state machine with a review gate
- [ ] Body schema validation per kind
- [ ] Publish pipeline: cache invalidation, TTS, reindex via outbox
- [ ] Admin authoring UI
- [ ] AI level estimation as a suggestion
- [ ] Seed content: one course, eight lessons, two hundred words
<!-- END GENERATED: todo -->

## Progress

The list above is generated from `tools/docgen/data/learning.json`, so its checkboxes cannot be
ticked by hand — `make docs` rewrites the block and `make docs-check` fails until it matches.
Completed work is recorded here instead.

| Task | Done | What landed |
|---|---|---|
| P7.2 | 2026-08-21 | The six tables in schema `content`, with the authoring, review and media status enums; `content_versions` frozen after publish by `trg_content_versions_immutable`; every foreign key indexed and pointing only inside `content` or at `core.users` (DB4); sqlc queries including the batched `GetManyContentVersionsByIDs`; a reversible down; integration tests covering immutability, every check constraint, both unique constraints, FK index coverage and the down migration |
| P7.3 | 2026-08-22 | Authoring state machine with pure domain transitions (5x5 matrix test); service layer enforcing BR-CONTENT-03 (self-approval forbidden) and BR-CONTENT-04 (media readiness verification before publish); outbox event emission for content.published and content.archived; archive-mid-session retrieval preservation; batched Reader.GetManyVersions with query count assertion; HTTP learner routes (/content) and admin authoring routes (/admin/content); full integration test suite |

## Carried into P7.3

- [x] **`media_refs` cannot be a foreign key, so publish has to check it.**
      `content_versions.media_refs` is `text[]` and a Postgres array carries no referential
      constraint. Nothing stops a version being published while it points at a
      `media_assets` row still `pending` — the learner would open a lesson whose audio does
      not exist yet. The publish service must resolve every `object_key` in `media_refs` and
      refuse while any referenced asset is not `ready`, with a test that proves an
      unprocessed asset blocks publishing.
- [x] **Archiving is an item-level action, and the reads have to know that.**
      See `DECISIONS.md`: a published version's `status` never changes, so a query filtered
      only on `content_versions.status = 'published'` still returns archived material.
      Every learner-facing read joins `content_items` and filters there too.

## Carried into P7.4

- [ ] **BR-CONTENT-05 `ErrContentInUse` is dead code.** Declared in `domain/errors.go` but never returned. Archive does not check whether a published lesson still references the version. Either wire `lesson` contract `IsVersionInUse` into `Archive` (blocked → 409 `CONTENT_IN_USE`) or remove the error and document why archiving is unconditionally allowed in Phase 2.
- [ ] **BR-CONTENT-04 `media_refs` has no API surface.** DTOs `CreateContentItemRequest` and `UpdateDraftRequest` (and their service structs) carry no `media_refs`/`tags` that reaches `content_versions.media_refs`; every create/update passes `[]string{}`. `verifyMediaAssetsReady` is therefore unreachable except via direct DB insert. Decide in P7.4 whether authoring UI supplies media_refs (add field to DTOs, validate object_key shape, and add test that publish blocks on non-ready asset supplied via API) or defer with rationale.
- [ ] **Heavier — decide before P7.4 builds on it: `current_version_id` semantics.** `createNewDraftFromPublished` sets `current_version_id` to the new draft and `item.status` to `draft`. Consequences: Archive emits `content.archived{version_id: draft}`; an item that was published but has an open draft cannot be archived (`ValidateTransition(draft, archived)` → 409); consumer of `content.published` reads `current_version_id` assuming "live" version. Proposal: `current_version_id` tracks the live (published) version only; draft ID lives elsewhere, or item has `published_version_id` + `draft_version_id`.
- [ ] **Heavier — decide before P7.4: BR-CONTENT-03 wrong subject.** `Review` checks `item.OwnerID == reviewerID`, but the author of the version under review is not necessarily the item owner (version 2 may be written by someone else). Correct subject is the version's author — requires `created_by` on `content_versions` (or `content_reviews` history). Also blocks `changes_requested` by owner with 403, though self-requesting changes is harmless.
- [ ] **Heavier — decide before P7.4: `actorID` is discarded.** `_ = actorID` in `Publish`, `Archive`, `UpdateDraft`, `SubmitForReview`. No audit trail for who published/archived. Module declares `depends_on: [audit]`; P7.4 should thread actor through to `audit.Log` or outbox payload, or document why audit is intentionally not wired in Phase 2.

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Scheduled publishing
- Content localisation
- Structural diff view between versions
- Reusable content blocks across items
<!-- END GENERATED: todo-future -->
