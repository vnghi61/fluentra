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
last_verified: 2026-08-21
---

# content — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/content` |
| Schema | `content` |
| Delivery phase | 2 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
The canonical model for every piece of learning material, whatever skill it belongs to: content items, immutable versions, media links, taxonomy, CEFR levelling, and the authoring workflow that takes a draft to publication.
<!-- END GENERATED: overview -->

**Context.** This module exists so that vocabulary, grammar, reading, listening, speaking and writing do **not** each reinvent items, versions, media and tagging. Getting it right in Phase 2 makes Phase 3 six thin modules; getting it wrong makes Phase 3 six copies of Phase 2 (ADR-0015).

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- `content_items` — the identity of a piece of material, independent of its revisions
- `content_versions` — immutable snapshots; a published lesson references a version, never a mutable item
- The authoring state machine: draft → in_review → approved → published → archived
- Taxonomy: topic, skill, CEFR level, exam relevance, tags
- Media asset references and TTS pre-generation on publish
- Content-level cache invalidation and search reindexing on publish
- Level estimation assistance (AI-suggested, human-approved)

**This module does NOT own:**

- Skill-specific item shape and grading — that belongs to each skill module
- Sequencing content into a course — that is `lesson`
- Storing files — that is `platform/storage`
- Deciding what a learner sees next — that is `learning`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/content/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/content/contract/` | You are calling this module from another module |
| `internal/modules/content/service/` | You are changing behaviour |
| `db/migrations/content/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/content/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `content.Reader` | `GetVersion`, `GetManyVersions`, `Browse` — batched to avoid N+1 from lesson rendering |
| struct | `content.Version` | `{ItemID, Version, Kind, Body, CEFRLevel, MediaRefs, Tags}` |
| event | `content.Published` | Triggers TTS generation, cache invalidation and reindexing |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `content.published` | publishes | `{item_id, version_id, kind, cefr_level}` |
| `content.archived` | publishes | `{item_id, version_id}` |
| `media.processed` | consumes | Mark an asset ready; block publishing until its media is processed |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `content` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/content/` · Queries: `db/queries/content/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `content.content_items` | Stable identity for a piece of material | `kind`, `slug` UNIQUE, `current_version_id`, `status`, `owner_id` |
| `content.content_versions` | Immutable revision | `item_id`, `version`, `body` jsonb (skill-specific payload), `cefr_level`, `status`, `published_at`. Never updated after publication. |
| `content.media_assets` | Reference to an object in storage | `object_key`, `kind`, `duration_ms`, `checksum`, `status` |
| `content.taxonomies` | Controlled vocabularies | `namespace` (topic/skill/exam), `code`, `label`, `parent_id` |
| `content.content_tags` | Item ↔ taxonomy mapping | Composite PK; indexed for filtered browsing |
| `content.content_reviews` | Review workflow record | `version_id`, `reviewer_id`, `decision`, `comments` |

**Indexes of note**

- `idx_content_versions_item_version` — unique on (item_id, version)
- `idx_content_items_status_kind` — partial `WHERE status = 'published'`, the learner-facing browse path
- `idx_content_tags_taxonomy` — filtered browsing by level and topic
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `content`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/content/{slug}` | `content.read.published` | Fetch a published content version |
| `GET` | `/api/v1/content` | `content.read.published` | Browse published content with taxonomy filters |
| `POST` | `/api/v1/admin/content` | `content.create` | Create a draft item |
| `PUT` | `/api/v1/admin/content/{id}/draft` | `content.edit` | Update the working draft |
| `POST` | `/api/v1/admin/content/{id}/submit` | `content.edit` | Submit for review |
| `POST` | `/api/v1/admin/content/{id}/review` | `content.review` | Approve or request changes |
| `POST` | `/api/v1/admin/content/{id}/publish` | `content.publish` | Publish the approved version |
| `POST` | `/api/v1/admin/content/{id}/archive` | `content.publish` | Archive |
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
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`storage`](../../platform/storage/AGENT.md) | → depends on | Media asset objects |
| [`search`](../../platform/search/AGENT.md) | → depends on | Reindex on publish |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | Authoring actions are audited |
| [`ai`](../../platform/ai/AGENT.md) | → depends on | Level estimation and authoring assistance |
| [`media`](../../platform/media/AGENT.md) | → depends on | TTS pre-generation, asset readiness |
| [`lesson`](../../modules/lesson/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`vocabulary`](../../modules/vocabulary/AGENT.md) | ← used by | consumes this module's contract |
| [`grammar`](../../modules/grammar/AGENT.md) | ← used by | consumes this module's contract |
| [`reading`](../../modules/reading/AGENT.md) | ← used by | consumes this module's contract |
| [`listening`](../../modules/listening/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`questionbank`](../../modules/questionbank/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-CONTENT-01** — A published version is immutable. Editing published material creates a new version; the old one stays live until the new one is published.
2. **BR-CONTENT-02** — A learner never sees a draft. Every learner-facing query filters `status = 'published'`.
3. **BR-CONTENT-03** — An author cannot approve their own version — review requires a different admin.
4. **BR-CONTENT-04** — Publishing is blocked until every referenced media asset is processed and verified.
5. **BR-CONTENT-05** — Archiving is blocked while a published lesson still references the version (`CONTENT_IN_USE`).
6. **BR-CONTENT-06** — CEFR level is required before review; an AI suggestion is a starting point, never the final value.
7. **BR-CONTENT-07** — `body` is validated against a JSON schema chosen by `kind`, so a skill module's payload cannot be malformed.
8. **BR-CONTENT-08** — Publishing invalidates the content cache, enqueues TTS generation, and triggers reindexing — all through the outbox so a failure in one does not roll back the publish.
9. **BR-CONTENT-09** — Slugs are immutable after first publication; a changed slug would break external links and bookmarks.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a content kind

1. Define the JSON schema for its `body` in `api/events/../content-kinds/`.
2. Register the kind and its schema in the validator.
3. Decide whether it needs media, and whether TTS applies on publish.
4. Add the authoring UI in `web/src/features/admin/content/`.
5. Add the skill module that consumes it, and its grader.
6. Add a fixture and a round-trip test through the whole workflow.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- No collaborative editing — one draft per item, last write wins with an ETag check.
- No scheduled publishing in Phase 2.
- Localisation of content itself (as opposed to UI) is not modelled; English-only material in v1.
- Version history has no diff view for `body` beyond raw JSON.
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
| `fluentra:{env}:content:version:{version_id}:v1` | 24 h | `content.published` / `content.archived` |
| `fluentra:{env}:content:browse:{filter_hash}:v1` | 10 min | Any publish |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `CONTENT_NOT_PUBLISHED` | 404 | Draft or archived content requested by a learner |
| `INVALID_STATE_TRANSITION` | 409 | e.g. publishing something not approved |
| `SELF_APPROVAL_FORBIDDEN` | 403 | The author tried to review their own version |
| `CONTENT_IN_USE` | 409 | Referenced by published material |
| `MEDIA_NOT_READY` | 409 | Referenced assets are still processing |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/content/...                    # unit
go test -tags=integration ./internal/modules/content/...  # integration (testcontainers)
```

**Focus areas**

- Published versions are genuinely immutable
- Self-approval is refused
- Archiving is blocked while referenced
- Body schema validation rejects a malformed payload per kind
- Publishing with unprocessed media is refused
- Cache invalidation and reindexing happen after commit, not inside the transaction
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not mutate a published version.
- Do not add skill-specific columns here — put them in `body`, validated by the kind's schema, or in the skill module.
- Do not let a learner-facing query omit the published filter.
- Do not publish inside a transaction that also does cache and search work — use the outbox.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
