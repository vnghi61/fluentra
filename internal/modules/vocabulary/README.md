---
module: vocabulary
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: skill
tables: [words, word_senses, word_relations, decks, deck_items, user_word_state]
depends_on: [content, srs, media, ai, search]
depended_on_by: [learning, reading, writing, grammar]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# vocabulary

Words and how learners acquire them: senses, pronunciations, collocations, word families, learner decks, and the vocabulary graders. The first skill module built, and the reference implementation the other five follow.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Words and how learners acquire them: senses, pronunciations, collocations, word families, learner decks, and the vocabulary graders. The first skill module built, and the reference implementation the other five follow.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Word entries: lemma, part of speech, senses, CEFR level, frequency rank
- Pronunciation (IPA plus TTS audio) and example sentences
- Collocations and word families
- Learner decks and per-learner word state
- Vocabulary exercise graders: recognition, recall, spelling, cloze, matching
- Word lookup and dictionary search
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
