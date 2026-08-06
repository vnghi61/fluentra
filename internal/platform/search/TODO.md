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

# search — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 4

- [ ] Indexer and Searcher interfaces
- [ ] tsvector generated columns and GIN indexes for content, vocabulary and question bank
- [ ] Query builder with ranking, highlighting and filters
- [ ] Reindex jobs
- [ ] Degraded fallback path
- [ ] Latency metrics with the 300 ms p95 trigger alert
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Trigram fuzzy matching for misspelled words
- Synonym dictionary for vocabulary search
- Semantic search over lessons via pgvector
- A dedicated engine if the latency trigger fires
<!-- END GENERATED: todo-future -->
