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
