---
module: vocabulary
tier: learning
group: modules
status: DONE
phase: 2
owner: "@learning-team"
schema: skill
tables: [words, word_senses, word_relations, decks, deck_items, user_word_state, vocab_uploads, vocab_upload_items]
depends_on: [content, srs, media, ai, search]
depended_on_by: [learning, reading, writing, grammar]
spec_version: 1.0.0
last_verified: 2026-08-25
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
- [ ] Audio URL resolution: `content.Reader` exposes no media-asset lookup, so `audio_url` is null. It needs a `c_content` method returning a playable link for a `content.media_assets` object key; do not format a URL in this module
- [ ] Seed audio: the 200 senses P11.1 authors carry IPA, definitions and examples but no audio, because there is nothing to point `audio_asset_id` at until the lookup above exists. P11.1's acceptance asks for audio that resolves, and it is the one part of that task still open
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

## Done in work order 8

- [x] **A topic on every learner-added word.** `vocab_verify` asks for one from a closed
      list, `normaliseTopic` refuses anything outside it, and the sense carries it in
      `skill.word_senses.domain` — a column that had existed since `1700000230` with nothing
      writing to it. The learner's own choice is their deck membership: `my-words-<topic>`
      rather than the single `my-words` deck every word used to land in.
- [x] **The upload list shows what was decided, not what was typed.** `VocabUploadItem` now
      carries the definition, the Vietnamese gloss and the examples, so the screen stops
      repeating the learner's own note back at them.
- [x] **`vocabulary_quiz` dropped from `GradedKinds()`.** It was declared and seeded by
      nothing, and the seed's bidirectional check compares the seed against the runner, so
      nothing could ever have caught it.

## Done in work order 9

- [x] **Vocabulary administration & withdrawal.** Added `GET /admin/vocabulary/words` (with source filter for seed vs learner upload) and `DELETE /admin/vocabulary/words/{id}` to withdraw words and senses from the shared dictionary.
- [x] **Learner words queue inspection & moderation.** Added `GET /admin/vocabulary/queue` displaying model verification verdicts, target decks, and sense metadata.
- [x] **Word sense editing & sense withdrawal.** Added `PATCH /admin/vocabulary/senses/{id}` to correct definitions, Vietnamese glosses, topics, or examples, and `DELETE /admin/vocabulary/senses/{id}`.
- [x] **Withdrawal decision.** Recorded in `DECISIONS.md`: withdrawal suspends every review card
  on the material first, across all learners, and only then deletes the sense. Archiving the
  content version was the first answer and BR-CONTENT-01 forbids it — a published
  `content_versions` row cannot be updated at all.

The Phase 2 boxes above are unticked because docgen renders every generated item that way —
words, senses, decks, dictionary lookup and the four graders all shipped in Phase 2. Read the
code for status, not the box (`phase-3-next-steps.md` §4).
