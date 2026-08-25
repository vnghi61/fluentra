---
module: vocabulary
tier: learning
group: modules
status: ACTIVE
phase: 2
owner: "@learning-team"
schema: skill
tables: [words, word_senses, word_relations, decks, deck_items, user_word_state]
depends_on: [content, srs, media, ai, search]
depended_on_by: [learning, reading, writing, grammar]
spec_version: 1.0.0
last_verified: 2026-08-25
---

# vocabulary — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/vocabulary` |
| Schema | `skill` |
| Delivery phase | 2 |
| Status | **ACTIVE** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Words and how learners acquire them: senses, pronunciations, collocations, word families, learner decks, and the vocabulary graders. The first skill module built, and the reference implementation the other five follow.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Word entries: lemma, part of speech, senses, CEFR level, frequency rank
- Pronunciation (IPA plus TTS audio) and example sentences
- Collocations and word families
- Learner decks and per-learner word state
- Vocabulary exercise graders: recognition, recall, spelling, cloze, matching
- Word lookup and dictionary search

**This module does NOT own:**

- Review scheduling — that is `srs`
- Content versioning — that is `content`
- Generating example sentences — it asks `platform/ai`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/vocabulary/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/vocabulary/contract/` | You are calling this module from another module |
| `internal/modules/vocabulary/service/` | You are changing behaviour |
| `db/migrations/vocabulary/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/vocabulary/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `vocabulary.Grader` | Implements `learning.ExerciseGrader` for vocabulary activity kinds |
| interface | `vocabulary.Reader` | `LookupWord`, `GetSenses` — used by `reading` and `writing` for inline glossing |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `vocabulary.word_learned` | publishes | `{user_id, word_sense_id, cefr_level}` |
| `review.card_answered` | consumes | Update per-learner word state as cards mature |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `skill` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/vocabulary/` · Queries: `db/queries/vocabulary/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `skill.words` | Lemma-level entry | `lemma` + `pos` UNIQUE, `cefr_level`, `frequency_rank`, `ipa` |
| `skill.word_senses` | One meaning of a word | `word_id`, `definition`, `register`, `domain`, `examples` jsonb |
| `skill.word_relations` | Families, synonyms, collocations | `from_word_id`, `to_word_id`, `relation` |
| `skill.decks` | Learner or curated collection | `owner_id` (null = curated), `slug`, `name`, `is_public` |
| `skill.deck_items` | Word in a deck | Unique on (deck_id, word_sense_id) |
| `skill.user_word_state` | Per-learner status | `user_id`, `word_sense_id`, `status` (new/learning/known/ignored), `first_seen_at` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `vocabulary`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/vocabulary/words/{lemma}` | `content.read.published` | Dictionary lookup with senses, IPA, audio, examples |
| `GET` | `/api/v1/vocabulary/search` | `content.read.published` | Search the dictionary |
| `GET` | `/api/v1/vocabulary/decks` | `self` | The learner's decks plus curated ones |
| `POST` | `/api/v1/vocabulary/decks` | `self` | Create a deck |
| `GET` | `/api/v1/vocabulary/decks/{id}/words` | `self` | The word senses in a deck, with everything a flashcard renders |
| `POST` | `/api/v1/vocabulary/decks/{id}/words` | `self` | Add a word sense to a deck |
| `DELETE` | `/api/v1/vocabulary/decks/{id}/words/{sense_id}` | `self` | Remove |
| `POST` | `/api/v1/vocabulary/words/{sense_id}/state` | `self` | Mark known or ignored |
| `POST` | `/api/v1/admin/vocabulary/words` | `content.create` | Create a word entry |
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
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`content`](../../modules/content/AGENT.md) | → depends on | Word entries are content versions |
| [`srs`](../../modules/srs/AGENT.md) | → depends on | Words enter spaced repetition |
| [`media`](../../platform/media/AGENT.md) | → depends on | TTS pronunciation audio |
| [`ai`](../../platform/ai/AGENT.md) | → depends on | Example sentence generation |
| [`search`](../../platform/search/AGENT.md) | → depends on | Dictionary search |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`reading`](../../modules/reading/AGENT.md) | ← used by | consumes this module's contract |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`grammar`](../../modules/grammar/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-VOCABULARY-01** — A learner learns a **sense**, not a lemma. "Bank" as a financial institution and as a riverbank are different cards.
2. **BR-VOCABULARY-02** — A word cannot appear twice in the same deck (unique on deck and sense).
3. **BR-VOCABULARY-03** — Deck count and size limits come from the learner's entitlement; the free tier is limited.
4. **BR-VOCABULARY-04** — Grading recall accepts minor spelling variation via edit distance, configured per activity; grading spelling does not.
5. **BR-VOCABULARY-05** — British and American spellings are both accepted unless the activity explicitly tests one variant.
6. **BR-VOCABULARY-06** — Example sentences are AI-generated but reviewed before publication — a wrong example teaches a wrong usage.
7. **BR-VOCABULARY-07** — Pronunciation audio is pre-generated at publish time and cached; it is never synthesised during a request.
8. **BR-VOCABULARY-08** — A word marked `ignored` never appears in a session again unless the learner reverses it.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a vocabulary exercise type

1. Define the activity kind and its `config` schema in `lesson`.
2. Define the content `body` schema in `content` if the item shape is new.
3. Implement the grading branch in `service/grader.go`.
4. Decide whether it produces review items, and with what initial grade.
5. Add the React renderer.
6. Add golden-file tests covering correct, near-miss and wrong answers.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Sense disambiguation in reading glossing is heuristic (frequency plus context window), not a model.
- Collocation data is manually curated; there is no corpus-derived pipeline.
- No morphological analyser — inflected forms are mapped by a lookup table.
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
| `fluentra:{env}:vocabulary:word:{lemma}:v1` | 30 d | Version bump on republish |
| `fluentra:{env}:vocabulary:deck:{deck_id}:v1` | 10 min | Deck write |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `DECK_LIMIT_REACHED` | 409 | Plan limit on deck count |
| `DECK_SIZE_LIMIT_REACHED` | 409 | Plan limit on words per deck |
| `WORD_ALREADY_IN_DECK` | 409 | Duplicate sense in the deck |
| `WORD_NOT_FOUND` | 404 | No entry for that lemma |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/vocabulary/...                    # unit
go test -tags=integration ./internal/modules/vocabulary/...  # integration (testcontainers)
```

**Focus areas**

- Sense-level uniqueness in decks
- Edit-distance tolerance boundaries per activity kind
- British/American spelling acceptance
- Deck and size limits by entitlement
- Review items produced with the correct initial grade
- Lookup performance on the dictionary index
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not schedule reviews here — return review items and let `srs` schedule.
- Do not treat a lemma as the learnable unit.
- Do not synthesise audio during a request.
- Do not publish an AI-generated example without review.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
