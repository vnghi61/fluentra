---
module: search
tier: platform
group: platform
status: PLANNED
phase: 4
owner: "@platform-team"
schema: none
tables: []
depends_on: [cache, job]
depended_on_by: [content, vocabulary, questionbank, lesson, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# search — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/search` |
| Schema | `none` |
| Delivery phase | 4 |
| Status | **PLANNED** |
| Owner | @platform-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
A thin abstraction over full-text search. Backed by PostgreSQL `tsvector` in v1, with an interface shaped so a dedicated engine can replace it without touching callers.
<!-- END GENERATED: overview -->

**Context.** Deliberately minimal. Introducing Elasticsearch for a corpus of tens of thousands of items would add an operational dependency and a synchronisation problem to solve a query that Postgres answers in single-digit milliseconds.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Index definition and maintenance per searchable entity
- Query building: tokenisation, prefix matching, ranking, highlighting
- Reindex jobs, full and incremental
- Language configuration for English content
- Search latency and result-quality metrics

**This module does NOT own:**

- Owning the searchable data — each module owns its own rows and its own index
- Semantic or vector search — that is `platform/ai` with pgvector
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/search/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/search/contract/` | You are calling this module from another module |
| `internal/platform/search/service/` | You are changing behaviour |
| `db/migrations/search/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/platform/search/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `search.Indexer` | `Index(ctx, doc)`, `Remove(ctx, id)` — called by the owning module on write |
| interface | `search.Searcher` | `Search(ctx, Query) (Results, error)` with filters, ranking and highlighting |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `content.published` | consumes | Index the published version |
| `content.archived` | consumes | Remove from the index |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
This module owns no tables.
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `search`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
_None yet._
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
| [`cache`](../../platform/cache/AGENT.md) | → depends on | see its contract |
| [`job`](../../platform/job/AGENT.md) | → depends on | see its contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`vocabulary`](../../modules/vocabulary/AGENT.md) | ← used by | consumes this module's contract |
| [`questionbank`](../../modules/questionbank/AGENT.md) | ← used by | consumes this module's contract |
| [`lesson`](../../modules/lesson/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-SEARCH-01** — The search index is derived data. It can always be rebuilt from the owning tables, and a rebuild must be routine, not an emergency.
2. **BR-SEARCH-02** — Indexing happens in the same transaction as the write when it is a generated column, and via an event when it is a separate table.
3. **BR-SEARCH-03** — Only published content is searchable by learners; admins may search drafts.
4. **BR-SEARCH-04** — Every query is bounded by a `LIMIT` and returns a cursor.
5. **BR-SEARCH-05** — Search must degrade to a simple `ILIKE` prefix match if the index is unavailable — a degraded search beats an error page.
6. **BR-SEARCH-06** — Query latency above 300 ms p95 is the trigger to reconsider the backend, and is monitored for exactly that reason.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Make an entity searchable

1. Add a `search_vector tsvector` generated column, or a dedicated search table if the document spans rows.
2. Add a GIN index.
3. Populate on write and on the relevant domain event.
4. Add a reindex job for backfill.
5. Add the entity to the search endpoint's type filter.
6. Measure p95 latency on realistic data volumes before shipping.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Postgres FTS has no built-in fuzzy matching beyond trigram similarity, no synonym management, and limited relevance tuning.
- No cross-entity ranking — searching lessons and words together requires a union with hand-tuned weights.
- Stemming is English-only; Vietnamese UI text is not full-text searchable.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/platform/search/...                    # unit
go test -tags=integration ./internal/platform/search/...  # integration (testcontainers)
```

**Focus areas**

- Ranking puts exact title matches first
- Draft content is invisible to learners and visible to admins
- Reindex produces the same results as incremental indexing
- Degraded path when the index is missing
- Query latency on a realistically sized dataset
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not add Elasticsearch without the latency trigger firing and an ADR.
- Do not make the index a source of truth.
- Do not run an unbounded search query.
- Do not expose draft content to learners through search.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
