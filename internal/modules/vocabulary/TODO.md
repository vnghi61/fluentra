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

# vocabulary — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] Words, senses, relations, decks and learner state
- [ ] Dictionary lookup and search
- [ ] Deck CRUD with entitlement limits
- [ ] Recognition, recall, spelling and cloze graders
- [ ] Review item production feeding `srs`
- [ ] TTS pronunciation pre-generation
- [ ] Seed dictionary of 2,000 A1–B1 senses
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Corpus-derived collocations
- Personalised word recommendation from reading history
- Morphological analyser
- Image mnemonics
<!-- END GENERATED: todo-future -->
