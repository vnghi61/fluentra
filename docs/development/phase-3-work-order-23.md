---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-23
---

# Phase 3 — work order 23: 10,000 words and thirteen Foundation courses, seeded offline

**Purpose.** The development seed still carries the Phase 2 dataset: 200 hand-written word senses and five
skill courses. The owner wants the catalogue a learner actually needs:

1. **10,000 vocabulary words**, each with IPA, a Vietnamese meaning, examples and a recorded pronunciation
   **stored as a link** in the seed data — not looked up and republished after the fact.
2. **Thirteen Foundation courses** in place of the five: Grammar Foundations, English Tenses, Sentence
   Structure, Clauses, Sentence Patterns, Vocabulary Foundations, Word Formation, Phrasal Verbs &
   Expressions, Pronunciation Foundations, Reading, Listening, Speaking and Writing Foundations.

Both are built the way WO 22 builds mock tests: **generate once, check once, freeze into fixtures**, so
`make seed` is fast, offline and gives every machine the same data.

**Read first.** WO 19 §D (Foundation topics, `cmd/foundation`); WO 21 Stage G and D21-6 to D21-8 (recorded
pronunciation as a link, credit); WO 22 D22-3 (fixtures); `vocabulary`, `content`, `lesson` `AGENT.md`;
the owner's brief of 2026-09-20 §1 (Foundation coverage, "CEFR is metadata, not the course structure")
and §11 (sources and licences).

---

## 1. What exists today

Verified 2026-09-23.

| Fact | Where |
|---|---|
| 200 word senses, written by hand, one curated public deck | `cmd/seed/content_data.go` (`seedWordSense`) |
| `skill.words` already has `frequency_rank` | `seedVocabularyWords` |
| Recorded pronunciation arrives by `seed -audio`, which looks each lemma up **after** the seed and publishes a **second version** of every sense to hold the link | `cmd/seed/audio.go` |
| Five courses: Everyday English A2–B1 (8 lessons), Reading, Writing, Speaking, Listening (6 each) | `cmd/seed/*_data.go` |
| 70 spine nodes: grammar 37, pattern 13, vocabulary 8, pronunciation 8, skill 4 | `db/migrations/content/*seed*` |
| **No spine nodes** for word formation, idioms or fixed expressions; each skill has one node and no progression | same |
| `cmd/foundation -all` drafts a topic, three exercises, a quiz and a review item per node; drafts need approval (BR-CONTENT-10) | WO 19 §D; fixed in `302c025`, `6ec61f8` |
| The lesson runner has **no kind for a Foundation topic's explanation**: a topic renders only on `/foundation/topics/{code}` | `web/src/routes/LessonPage.tsx` |

### What a request for 10,000 lookups costs

At the dictionary's courtesy pace (four a second) 10,000 lookups take about 45 minutes, and the service
was down for a day in September (522s, then timeouts). Done inside `make seed` that is a 45-minute seed
that fails whenever someone else's server does. Done once and written into a fixture, it costs nothing
afterwards. That is the whole argument for D23-4.

---

## 2. Decisions

| Question | Default |
|---|---|
| D23-1. What "10,000 words" counts | **10,000 headwords (lemmas)**, one primary sense each, ranked by frequency. Inflections fold into their lemma ("went" is "go"). More senses per word are later work |
| D23-2. Where the list comes from | **An openly licensed frequency list**, licence verified and recorded in the fixture header (source, URL, licence, date). Candidates to check: the `wordfreq` data, Wiktionary frequency lists. **Not** Oxford 3000/5000 or English Vocabulary Profile: both are proprietary |
| D23-3. Who writes meanings and examples | **The model, in batches of 50 lemmas**: part of speech, CEFR estimate, a simple English definition, the Vietnamese meaning, two example sentences with Vietnamese translations. Original text, so no dictionary's definitions are copied and no share-alike obligation attaches to them |
| D23-4. Pronunciation | **The link is looked up once by the build tool and written into the fixture**: `audio_url`, `audio_attribution`, `audio_licence`, US recording first (D21-6). `make seed` writes version 1 with the link; no second version. `seed -audio` stays only as a repair for words that had none |
| D23-5. How the words are checked | **Automatic checks on all 10,000, a person on a sample.** Schema, the lemma appears in both examples, Vietnamese present, CEFR in range, no duplicate lemma. A reviewer reads a random 2 % plus every word the checks flagged. Vocabulary cards are practice content, which BR-CONTENT-10 allows to publish without per-item review |
| D23-6. Decks | **One public deck per CEFR level** (A1 … C1), plus "Top 1,000" by frequency replacing the current curated deck. Nothing enrols a learner in 10,000 cards: they add a deck |
| D23-7. The thirteen courses | **Built from spine nodes**: each course is a set of nodes, each node one lesson, and every node belongs to exactly one course (table in Stage C). A course's lessons are that node's published Foundation content |
| D23-8. The five old courses | **Removed from the seed; their content kept.** Reading passages, listening scripts, writing prompts and speaking tasks become lessons of the four skill Foundation courses; Everyday English's flashcards go to Vocabulary Foundations. On a database with real learners they are unpublished, never deleted |
| D23-9. How Foundation content reaches a fresh database | **The WO 22 pattern**: `cmd/foundation` generates, a person approves in the review queue, an export freezes the published content into `db/fixtures/foundation/*.json`, and `make seed` loads it |

---

## 3. Order

```text
A  word list        build tool: list, meanings, IPA, audio link → fixture      cmd/vocabgen
B  load words       seed reads the fixture; decks by level                     cmd/seed
C  spine            the missing nodes; the course ↔ node map                   migration
D  topic step       a runner kind that shows a topic's explanation             spec → lesson → web
E  foundation       generate, review, export, load                             cmd/foundation, cmd/seed
F  courses          the thirteen courses; the five retired                     cmd/seed, lesson
```

A–B and C–F are independent: words can ship before courses. One commit per stage:
`feat(vocabulary): wo23 stage X — …`.

## 4. Numbers reserved

| | Range |
|---|---|
| Migrations | `1700000960`–`1700000979` |
| Advisory lock IDs | `1_700_000_960`–`0979` |

---

## Stage A — the word list, built once

**`cmd/vocabgen`**, a command of its own with its own declared config sections (WO 21 found
`cmd/foundation` reading none of its AI keys; do not repeat it). Four steps, each resumable from a cache
file in the scratch directory so a crash at word 7,000 does not start over:

1. **List.** Read the source list (D23-2), lemmatise, drop proper nouns, abbreviations, profanity and
   non-words, keep the first 10,000 by rank.
2. **Meanings.** Batches of 50 to the model (D23-3). Anything that fails the checks of D23-5 is retried
   once, then written to a `rejected` list for a person.
3. **Pronunciation.** IPA and the recording link from the dictionary, then Wikimedia Commons by file name
   — the same two sources and order as `cmd/seed/audio.go`, reusing its code rather than copying it. A
   word with no recording has no link; the browser falls back as it does today (D21-8).
4. **Write** `db/fixtures/vocabulary/words-{a1,a2,b1,b2,c1}.json`, each with the header naming every
   source, licence and date.

**Traps.** (1) A recording without its credit page is not written (Trap 2 of WO 21 Stage G). (2) Model
CEFR estimates drift; clamp to A1–C1 and flag disagreements with the source list's rank band for the
reviewer rather than trusting either. (3) Keep each fixture file under a few MB; ten thousand entries
with two examples each is about 5 MB of JSON in total.

**Gate.** The fixtures hold 10,000 unique lemmas; at least 80 % carry an audio link with a credit; the
reviewer's sample has been read and its corrections applied.

## Stage B — loading 10,000 words

- `seedVocabularyWords` reads the fixtures instead of `content_data.go`, and writes `audio_url`,
  `audio_attribution` and `audio_licence` into version 1 of each sense body.
- **Batches.** `pgx.Batch` in chunks of a few hundred rows. The Supabase pooler is a network hop per
  statement; row by row, 10,000 words × five statements is tens of minutes.
- **Decks** per D23-6, idempotent on slug.
- `make seed` drops the `seed -audio` line, or keeps it only as the repair D23-4 describes.

**Gate.** A fresh `make seed` loads 10,000 words in minutes with no network call; a flashcard of any
seeded word plays its recording and shows the credit.

## Stage C — the spine the courses need

New nodes (migration, `ON CONFLICT (namespace, code)` like `1700000781`), each with a label, description,
CEFR and prerequisite:

- **Word formation** (vocabulary): prefixes, suffixes, word families, conversion, compound words.
- **Expressions** (vocabulary): idioms, fixed expressions, common spoken expressions.
- **Skill progressions** (skill): Listening — words, phrases, sentences, conversations. Speaking — words,
  phrases, sentences, paragraphs. Reading — sentences, paragraphs, vocabulary in context, main idea and
  detail. Writing — sentences, paragraphs, linking words.

**Course map** — every node in exactly one course:

| Course | Nodes |
|---|---|
| Grammar Foundations | parts of speech, nouns, articles, pronouns, adjectives, adverbs, prepositions, conjunctions, modal verbs, comparatives and superlatives, passive voice, conditionals, reported speech, gerunds and infinitives, participles, common grammar mistakes |
| English Tenses | tenses and the twelve tense nodes |
| Sentence Structure | sentence structure, subject–verb–object, questions and negatives |
| Clauses | clauses, relative clauses, noun clauses, adverbial clauses |
| Sentence Patterns | the thirteen pattern nodes |
| Vocabulary Foundations | essential everyday, common verbs, high-frequency words, topic vocabulary, collocations, synonyms and antonyms, academic, workplace |
| Word Formation | the five new word-formation nodes |
| Phrasal Verbs & Expressions | phrasal verbs (grammar namespace) and the three expression nodes |
| Pronunciation Foundations | the eight pronunciation nodes |
| Reading, Listening, Speaking, Writing Foundations | each skill's node and its progression |

The map lives in one place — a table the seed reads — so a node added later is placed by one edit.

**Gate.** Every current spine node appears in exactly one course; a test fails if one is in none or two.

## Stage D — a lesson step that teaches

A Foundation lesson opens with the topic's explanation, then its exercises and quiz. The runner cannot
show the first part today.

- Spec first: a `foundation_topic` activity kind, ungraded, weight 0 like `lesson_material` (WO 20), whose
  config is the topic's published body: objective, explanation, examples.
- `lesson` registers the kind; the web renders it with the component `/foundation/topics/{code}` already
  uses, not a second copy.

**Gate.** A lesson whose first activity is a topic shows the explanation and moves on to the exercises
with "Tiếp tục".

## Stage E — Foundation content for all 93 nodes

1. `go run ./cmd/foundation -all` (WO 21 fixes applied) for the 70 existing nodes and the Stage C
   additions.
2. Review in the queue. WO 22 Stage F's batch approval applies: one batch per node.
3. **`cmd/foundation -export`** writes the published topics, exercises and quizzes to
   `db/fixtures/foundation/*.json`.
4. **`cmd/seed -foundation`** loads them with the approval they already had.

**Trap.** A node whose topic was rejected has no lesson. The export reports every node without
published content, so a course does not silently lose a lesson.

**Gate.** After a database reset, `make seed` publishes Foundation content for every node without a model
call.

## Stage F — thirteen courses, five retired

- `cmd/seed` builds each course through `lesson`'s `Author` contract (`EnsureCourse`, `EnsureUnit`,
  `EnsureLesson`, `SyncActivities`): one unit per group of related nodes, one lesson per node in
  prerequisite order, activities = topic step (Stage D), exercises, quiz.
- A course's CEFR range is the span of its nodes; CEFR is metadata on the card, not the grouping (brief §1).
- The five old courses leave the seed; their content moves as D23-8 says. Slugs of retired courses are
  not reused (BR-CONTENT-09 spirit: links do not change meaning).
- Course cards and titles in Vietnamese and English.

**Traps.** (1) `lesson.Author` is the only path; this seed must not insert into `learn.*` directly the
way the old course seed did. (2) A learner's placement opens lessons below their level
(BR-LEARNING-11); check that placement still finds lessons when every course spans several levels.

**Gate.** The catalogue shows the thirteen courses in both locales; each opens, and a lesson runs from
the explanation through the quiz.

---

## 5. Final gate

WO 19 §2 in full; the catalogue and a Foundation lesson at 320 and 390 px in both locales; a fresh
`make migrate-up && make seed` on an empty database finishes offline and in minutes; `make gen-check`
and `make gen-check-web` after committing.

## 6. What to cut, in order

1. D23-6's per-level decks (one "all words" deck works).
2. Stage C's skill progressions (one lesson per skill to start).
3. Stage D — link the topic page from the lesson instead of rendering it inline.

The 10,000 words with links (A–B) and the thirteen courses (F) are the owner's ask and are not on this
list.
