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

# vocabulary — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Lemma-level or sense-level learning? | Sense-level | Conflating senses teaches the wrong thing and makes scheduling meaningless for polysemous words, which are precisely the difficult ones |
| Generate example sentences at request time? | No — generate at authoring time and review them | Latency, cost, and above all correctness: an unreviewed example can teach a wrong collocation to thousands of learners |
| What does a grader do when the activity content carries no answer key? | Return an error | The alternative that shipped first was to mark the learner correct. That inflates progress and schedules a review card for a word they may not know, silently and for everyone the broken content reaches. Unreadable content is a deployment fault and should look like one |
| Where does a sense audio URL come from? | Nowhere yet — `audio_url` is null | content.media_assets stores an object key, not a link, and turning one into something a browser can play needs a presigned URL reached through c_content. content.Reader exposes no media lookup, so the honest answer is null. The first version formatted `/api/v1/content/assets/{id}`, a path in no OpenAPI document, which would 404 in every flashcard |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
