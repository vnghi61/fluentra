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

# content

The canonical model for every piece of learning material, whatever skill it belongs to: content items, immutable versions, media links, taxonomy, CEFR levelling, and the authoring workflow that takes a draft to publication.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Content is the product. Producing it is the real bottleneck, not writing code — so the authoring workflow, review gate and publishing pipeline are first-class features, not admin afterthoughts.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- `content_items` — the identity of a piece of material, independent of its revisions
- `content_versions` — immutable snapshots; a published lesson references a version, never a mutable item
- The authoring state machine: draft → in_review → approved → published → archived
- Taxonomy: topic, skill, CEFR level, exam relevance, tags
- Media asset references and TTS pre-generation on publish
- Content-level cache invalidation and search reindexing on publish
- Level estimation assistance (AI-suggested, human-approved)
<!-- END GENERATED: readme-resp -->

## Where things are

<!-- BEGIN GENERATED: readme-folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: readme-folders -->

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**PLANNED** — planned for delivery phase 2. See [/ROADMAP.md](../../../ROADMAP.md).
