---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-23
---

# Phase 3 — work order 22: the content a learner needs — words, Foundation courses, mock tests

**Purpose.** One work order, handed over once, that turns the development dataset into the product the
owner described. It has three parts and one piece of shared machinery:

| Part | What the learner gets |
|---|---|
| **Shared** | Generated content is published when an **independent AI verifier** confirms it; a person sees only what it doubts |
| **I. Vocabulary** | **10,000 words** with IPA, Vietnamese meaning, examples and a recorded pronunciation stored as a link; and adding a word **checks the database first**, so a known word costs no dictionary or model call |
| **II. Foundation** | **Thirteen Foundation courses** in place of the five Phase 2 courses |
| **III. Exams** | Pick an **exam** — TOEIC, IELTS, VSTEP — then **Test 1 … Test N** or a random test, in exam or practice mode; **at least five tests each** from a fresh seed, **built to the officially published format** and checked against it, and **more every day** |

Everything a fresh database needs is built **once**, checked, and **frozen into fixtures** in the repo, so
`make seed` is fast, offline and gives every machine the same data.

**Owner's decisions, 2026-09-23** (quoted so no later reader has to reconstruct them):

1. "Tôi cho phép public duyệt nếu AI xác minh đúng" — a generated question or Foundation item is published
   without a person when AI verification confirms it (Stage A). This amends BR-QUESTIONBANK-04 and
   BR-CONTENT-10.
2. Adding vocabulary must check the database before any AI call, because most words already exist
   (Stage B).
3. The seed accounts are `nguyenvannghi1110@gmail.com` (admin) and `nghitienvl@gmail.com` (learner) —
   already done in `6ec61f8`.

**Read first.** WO 19 §§D, F–H; WO 21 Stages C, E, G and D21-6 to D21-8; `vocabulary`, `content`,
`lesson`, `learning`, `questionbank` and `exam` `AGENT.md`; the owner's content-system brief of 2026-09-20
§1 (Foundation coverage, "CEFR is metadata, not the course structure"), §§5–8 (exams, bank, mock tests,
quality) and §11 (verify exam formats from official sources; record every source and licence).

**How to run it.** One branch, stages in the order of §3, one commit per stage:
`feat(<module>): wo22 stage X — …`. A stage's gate passes before the next stage starts.

---

## 1. What exists today

Verified 2026-09-23 against the code, the migrations and the dev database.

### Shared

| Fact | Where |
|---|---|
| Generation already verifies each item before storing it — structure, provenance, CEFR and a **blind solve** — via `learning.VerifyItem`. The blind solve uses the **same provider chain that wrote the item**: a model grading its own work | `learning/service/generator.go`, `verifier.go` |
| `content.Author` offers `EnsurePublished` and `EnsureDraft` only; approval exists only as the reviewer's HTTP route. The review queue approves **one item at a time** | `content/contract/contract.go`; WO 21 Stage C |
| Learner reports on an item already have a table | `content.item_reports` (`1700000510`) |
| The worker resolves the admin that owns generated content with `RoleMembers().FirstHolderOf(RoleAdmin)` | `cmd/worker/main.go` |
| Config keys are listed in `.env.example`. `docs/deployment/configuration.md`, which CLAUDE.md names as the registry, does not exist. `config.Load` **drops any env section a command does not declare** — `cmd/foundation` read none of its AI keys for that reason (fixed in `302c025`) | repo root; `internal/shared/config` |

### I. Vocabulary

| Fact | Where |
|---|---|
| 200 word senses, written by hand, one curated public deck; `skill.words` has `frequency_rank`; `(lemma, pos)` is unique | `cmd/seed/content_data.go`; `1700000230` |
| Recorded pronunciation arrives by `seed -audio`, which looks each lemma up **after** the seed and publishes a **second version** of every sense to hold the link | `cmd/seed/audio.go` |
| **Adding words never checks the database first.** The search field above the box only *appends the lemma* to the text; pasted text and picked words are submitted identically to `POST /me/vocabulary/uploads` | `web/src/features/vocabulary/components/UploadForm.tsx` |
| For every term the job calls the **dictionary** (external), then the **model** (`TaskVerifyVocabulary`), and only then touches the database (`sharedWord`, `matchingSense`) | `vocabulary/service/upload.go` `verifySingleWord`, `judge`, `materialise` |
| `matchingSense` reuses an existing sense only when the English definition is **character-for-character equal** (case aside) or the learner's Vietnamese matches exactly. The model rewrites the definition each time, so a word already in the database gets a **new duplicate sense and content version** on nearly every upload | same |
| The learner's own meaning is kept on the upload item (`provided_meaning`) | `skill.vocab_upload_items` |

### II. Foundation

| Fact | Where |
|---|---|
| Five courses: Everyday English A2–B1 (8 lessons), Reading, Writing, Speaking, Listening (6 each) | `cmd/seed/*_data.go` |
| 70 spine nodes: grammar 37, pattern 13, vocabulary 8, pronunciation 8, skill 4 | `db/migrations/content/*seed*` |
| No nodes for word formation, idioms or fixed expressions; each skill has one node and no progression | same |
| `cmd/foundation -all` drafts a topic, three exercises, a quiz and a review item per node. Before `302c025` and `6ec61f8` it could not run at all (guard panic, no AI keys read, wrong admin query) | WO 19 §D |
| The lesson runner has **no kind for a topic's explanation**; a topic renders only on `/foundation/topics/{code}` | `web/src/routes/LessonPage.tsx` |

### III. Exams

| Fact | Where |
|---|---|
| Six exam versions: TOEIC LR, VSTEP 3-5, IELTS Academic, Cambridge B2 First, THPT 2026 (current), TOEFL iBT (not current) | `1700000860_create_exam_structures.sql` |
| Two blueprints: `toeic_default`, `vstep_default`. **IELTS has none**, so no IELTS test can be composed | same |
| IELTS parts are coarse: Listening is one 40-question part with group size 1, Reading likewise | same |
| **One fixed test per blueprint**: `mode=fixed` returns the stored row if one exists (`findFixedMockTest`) | `exam/service/composer.go` |
| No endpoint lists a version's fixed tests; `StartMockTestAttempt` takes no body, so a mock test is always sat in exam mode | `openapi.yaml`; `composer.go` |
| The exam hub lists `assess.exams`: three `mock-toeic-a2/b1/b2` rows renamed "Mixed-skill practice exam", plus `toeic-lr-2026` and `vstep-3-5` | `1700000860`, `1700000870` |
| The question bank holds **0 questions**; no job generates any | dev DB |
| TOEIC Part 1 needs photographs, IELTS Writing Task 1 needs charts; nothing produces either. TTS renders a script in **one voice** | `cmd/tts` |
| **The official format is stored but never enforced.** `assess.exam_parts` holds counts, minutes and a `constraints` column (option count, questions per group, word limits), and the TOEIC and VSTEP numbers match the published formats. But the question bank asks the generator for items **by kind and CEFR only** — no exam, no part, no constraints — so a "TOEIC Part 2" item may have four options instead of three and a "Part 3" conversation any number of questions | `questionbank/service/service.go` `Generate` |
| No production code ever fills `VerifyItemRequest.ExamConstraints`, so the structural check the verifier has for exam parts never runs. And the migration writes `options_count` and `sub_questions_per_group` while the Go struct reads `option_count` and `questions_per_group`: even read, they would be dropped | `learning/contract`; `1700000860` |
| Only multiple choice exists for exam listening and reading. IELTS is mostly **typed answers** — form, note, sentence and summary completion with a word limit — plus True/False/Not Given and matching | `learning` kinds |
| Exam mode already allows **one play** per clip and opens one section at a time; practice mode allows three. The play policy recognises only `listening_comprehension`, not TOEIC Parts 1–2 (`photo_description`, `question_response`) | `exam/service/service.go` `ListeningPlayPolicy` |
| VSTEP's version row cites Circular 01/2014/TT-BGDĐT, which is the six-level proficiency framework, not the VSTEP.3-5 test format | `1700000860` |

### The numbers

| | Count |
|---|---|
| Words | 10,000 headwords, one primary sense each |
| Foundation nodes | 70 today + 23 added in Stage E = **93**, each one lesson |
| Exam questions for five disjoint tests | TOEIC 1,000 (200 per test) · IELTS 425 · VSTEP 400 = **1,825**, ~2,200 generated with a 20 % margin |

At the dictionary's courtesy pace (four a second), 10,000 lookups take about 45 minutes, and the service
was down for a day in September. Inside `make seed` that is a 45-minute seed that fails whenever someone
else's server does; done once into a fixture it costs nothing afterwards.

---

## 2. Decisions

### Shared

| Question | Default |
|---|---|
| D22-1. Who approves generated content | **An independent AI verifier** (owner's decision 1). A draft is published without a person when a **different model from the one that wrote it** confirms it on every line of Stage A's checklist. Anything it doubts or cannot check goes to a person, who reviews by **batch**. The verifier can publish or escalate; it never rejects, so a false alarm costs a look, not an item |
| D22-2. When auto-publish is off | When only one provider model is configured (nothing independent to ask), when its switch is off (the default), or when the day's AI budget is spent. Then everything escalates — today's behaviour |
| D22-3. How auto-published items stay honest | **Marked, sampled, pullable.** The approval names the verifier; a daily random sample goes to a person; an item a learner reports stops being drawn until someone looks |
| D22-4. How content reaches a fresh database | **Generate once, verify once, freeze into fixtures** under `db/fixtures/`. `make seed` loads them offline and records the approval each item already had. No model call on `make seed` |

### I. Vocabulary

| Question | Default |
|---|---|
| D22-5. What "10,000 words" counts | **10,000 headwords (lemmas)**, one primary sense each, ranked by frequency. Inflections fold into their lemma ("went" is "go") |
| D22-6. Where the list comes from | **An openly licensed frequency list**, licence verified and recorded in the fixture header. Candidates to check: the `wordfreq` data, Wiktionary frequency lists. **Not** Oxford 3000/5000 or English Vocabulary Profile: both are proprietary |
| D22-7. Who writes meanings and examples | **The model, 50 lemmas per call**: part of speech, CEFR estimate, a simple English definition, the Vietnamese meaning, two examples with translations. Original text, so no dictionary's definitions are copied |
| D22-8. Pronunciation | **Looked up once by the build tool and written into the fixture** (`audio_url`, `audio_attribution`, `audio_licence`, US recording first per D21-6). Version 1 carries the link; no second version. `seed -audio` becomes a repair only |
| D22-9. How the words are checked | Automatic checks on all 10,000 (schema, lemma in both examples, Vietnamese present, CEFR in range, unique lemma) plus a person reading a random 2 % and every flagged word. Vocabulary is practice content, which BR-CONTENT-10 allows to publish directly |
| D22-10. Decks | One public deck per CEFR level (A1 … C1) plus "Top 1,000" replacing the curated deck. Nothing enrols a learner in 10,000 cards |
| D22-11. Adding a word the database already has | **Database first** (owner's decision 2). A known word is added from the database with no dictionary and no model call; a known word with a learner's meaning costs at most one model call choosing among its existing senses; only an unknown word takes today's path. The learner's own meaning never overwrites a shared sense |

### II. Foundation

| Question | Default |
|---|---|
| D22-12. The thirteen courses | **Built from spine nodes**: each course a set of nodes, each node one lesson, every node in exactly one course (map in Stage E). CEFR is metadata on the card, not the grouping |
| D22-13. What "verified" means for a topic | A topic has no key, so its check is a **review, not a solve**: a different model must find no error of fact or grammar, examples that illustrate the point, and a level within one band of the node's. The node's exercises and quiz pass the choice-item checklist. A node publishes only when its topic **and** all its items pass; otherwise its batch escalates whole |
| D22-14. The five old courses | **Removed from the seed; their content kept**: reading passages, listening scripts, writing prompts and speaking tasks become lessons of the four skill courses; Everyday English's flashcards go to Vocabulary Foundations. On a database with real learners they are unpublished, never deleted |

### III. Exams

| Question | Default |
|---|---|
| D22-15. What the learner chooses first | **The exam** — TOEIC, IELTS, VSTEP — as cards; level is shown on the card, never chosen. The three "Mixed-skill practice exam" rows are **unlisted, not deleted** (attempts point at them) |
| D22-16. Exams in scope | **TOEIC LR, IELTS Academic, VSTEP 3-5.** Cambridge B2 First, THPT and TOEFL keep their rows, unlisted |
| D22-17. Fixed tests | **Numbered and disjoint**: Test N draws only questions no earlier fixed test of the blueprint uses. A bank that cannot fill the next test does not get one |
| D22-18. What "random" means | **A freshly composed test** (existing `mode=random`), preferring questions the learner has not seen |
| D22-19. Practice mode | **Yes**, with the exam hub's settings: sections, duration, no time limit (BR-EXAM-17). Exam mode stays full-length and timed |
| D22-20. Daily generation | **One test's worth per day, rotating** TOEIC → IELTS → VSTEP, through the verifier. When published questions complete a disjoint test, the next numbered test is composed — with no person involved on a day the verifier confirms everything. Off by default |
| D22-21. TOEIC Part 1 photos | **Openly licensed photographs by link** (Wikimedia Commons or Openverse, CC0 or CC BY only) with a human-written description; the model writes the statements from the description, never the image |
| D22-22. IELTS Writing Task 1 charts | **Rendered by our code** from model-generated data to SVG in `fluentra-media`; the data stays in the body for the grader |
| D22-23. Voices | **Two voices** for conversations: the script carries speaker turns, TTS renders each with its speaker's voice |
| D22-24. How closely a test follows the official format | **Exactly, on everything published**: parts, question counts, groups and questions per group, options per question, question types and their mix, passage and recording shape, word and time limits, section order and timing, plays per recording, scoring scale. One **format specification per exam version**, each line citing the official source and the date it was checked; the generator is given it, the verifier checks against it, the composer and the runner obey it. What the official source does not publish is not invented |
| D22-25. IELTS typed answers | **A completion question kind**: typed answer, accepted variants, a word limit ("NO MORE THAN TWO WORDS"), graded by normalised exact match. Without it the tests are "IELTS-style (multiple choice)" and must be labelled so, never "IELTS" |
| D22-26. Official wording and scores | **Our own directions**, following the format but not copying ETS, IELTS or Ministry wording. Scores are labelled **estimates** where the owner of the exam does not publish a raw-to-score conversion (TOEIC's scaled score); a published conversion is used and cited where one exists |

---

## 3. Order

```text
Shared   A  verify       independent verifier; batch review of doubts      spec → content → learning → web
I        B  db-first     known words need no dictionary and no model       spec → vocabulary → web
I        C  word list    build tool → 10,000-word fixture with audio links  cmd/vocabgen
I        D  load words   seed reads the fixture; decks by level             cmd/seed
II       E  spine        23 new nodes; the course ↔ node map                migration
II       F  topic step   a runner kind that shows a topic's explanation     spec → lesson → web
II       G  foundation   generate, verify, export, load 93 nodes            cmd/foundation, cmd/seed
II       H  courses      thirteen courses; the five retired                 cmd/seed via lesson.Author
III      I  format       official spec per exam, sourced; enforced end to end spec → exam → learning → web
III      J  fixed tests  numbered, disjoint; list endpoint                  spec → exam
III      K  sitting      practice mode for a mock test                      spec → exam → web
III      L  hub          exam → tests → sit / practise / random             web
III      M  media        Part 1 photos, Task 1 charts, two voices           questionbank, media, tts
III      N  five tests   generate, verify, export, load; compose 1–5        cmd/examgen, cmd/seed
III      O  daily        generation job; next test appears on its own       questionbank, exam
```

**Dependencies.** A before G, N and O (they publish through it). B stands alone and ships first as a
quick win. C before D. E and F before H; G before H. I and J before L and N; M before N; N before O.
Parts I, II and III are otherwise independent.

## 4. Numbers reserved

| | Range |
|---|---|
| Migrations | `1700000940`–`1700000979` |
| Advisory lock IDs | `1_700_000_940`–`0979` |

---

## Shared

### Stage A — independent verification publishes; people see only the doubts

The generator's own check (§1) stays as a first filter. It is not the verification D22-1 means, because
the model that solves the item is the one that wrote it. This stage adds a second, independent pass at
publish time, used by Foundation (G), exams (N, O) and nothing else.

#### A.1 The verifier

- **A different model.** It reads the writer's model from `_provenance.model` and asks the **next
  configured provider whose model differs**. `platform/ai` gains a request option to exclude a model from
  the fallback chain; no new provider config — the four `AI_PROVIDER_*` slots exist. No such provider →
  escalate (D22-2).
- **Blind.** It sees the item redacted (`contentcontract.RedactForLearner`) — no key, no explanation — and
  answers it. Separately, it is then shown the key and explanation and asked to judge them.
- **One call per group.** A Part 3 conversation, a Part 6 text or an IELTS passage is verified whole:
  every question must pass or the whole group escalates.

#### A.2 The checklist — every line must pass to publish

| Kind | Passes when |
|---|---|
| Every kind | Gate 1 structure and safety; provenance present; fingerprint unique (BR-QUESTIONBANK-02); **the part's official specification** (Stage I: questions per group, options, question type, recording or passage shape, word limits); CEFR judged within one band of the target |
| Choice items (TOEIC Parts 1, 2, 5, 6, 7; IELTS and VSTEP reading and listening; Foundation exercises and quizzes) | The blind answer **equals the key** on every question; **exactly one option defensible** (a second defensible option is the commonest defect of generated distractors); the explanation supports the key and does not contradict the passage or script |
| Listening | Solved from the **script**; every question answerable from what is said, not from general knowledge |
| TOEIC Part 1 | Solved from the **human-written photo description** (D22-21) |
| Writing and speaking prompts | The task matches the part's specification — task type, word or time limit, the visual an IELTS Task 1 needs — and is answerable at the target level; chart data internally consistent |
| Foundation topic | D22-13: no error of fact or grammar, illustrative examples, level within one band; the verifier names the sentence it doubts |

Anything else — a parse failure, a timeout, a verifier that answers "unsure" — is a **doubt**, not a
failure.

#### A.3 Publishing without a person

- **Contract.** `content.Author` gains `ApproveVerified(ctx, versionID, verification)`: walks the draft
  `in_review → approved → published` with the existing state machine and events (TTS, reindex,
  `content.published` through the outbox, BR-CONTENT-08), and writes a `content_reviews` row whose
  reviewer is the owning admin (`RoleMembers().FirstHolderOf(RoleAdmin)`) and whose comment names the
  verifier model and time. `reviewer_id` is `NOT NULL` and references `core.users`: there is no anonymous
  approval.
- **Marking.** `_provenance.verification = {model, verdict, checked_at}`. The admin lists filter on it:
  auto-published, human-approved, escalated.
- **Bank.** `PublishQuestion` accepts a version approved by verification.
- **Rules**, through `tools/docgen/data` and `make docs`:
  BR-QUESTIONBANK-04 — "A question enters the bank only after its content version is approved, by a
  person or by an independent verifier that confirmed it."
  BR-CONTENT-10 — "Machine-authored exam and curriculum content enters as a draft, and is published by a
  person or by `ApproveVerified`; practice content may be published directly (`EnsurePublished`)."

#### A.4 The doubts: batch review

- A generation run tags its drafts with a **batch id** in `_provenance` (exam version and part, or spine
  node; run). The verifier leaves its doubts in their batch with the reason ("second option defensible",
  "blind answer B, key C").
- Spec first: `GET /admin/review-queue/batches` and `POST /admin/review-queue/batches/{id}/approve` with an
  optional `reject: [ids]` and a note.
- The WO 21 review screen gains a batch view: each item with its key, the verifier's reason and a reject
  toggle; one approve button for the rest. One transaction per batch (BR-CONTENT-11).

#### A.5 Keeping it honest (D22-3)

- **Switch**, default **off**, in `.env.example` and the declared defaults of every command that reads it.
- **Sample.** Daily, a random 2 % (at least five) of the previous day's auto-published items appear in the
  queue under "Kiểm tra mẫu". A rejection unpublishes the item: it stops being drawn, and stored mock-test
  compositions are not rewritten (BR-EXAM-12).
- **Reports.** A learner report on an auto-published item removes it from future draws and queues it.
- **Rate.** The admin screens show, per kind, the share escalated and the share of sampled items a
  person rejected; above 5 % the kind shows red — the verifier is not trustworthy for it.

**Traps.** (1) Two slots with one model name are not independent: compare models, not slot numbers.
(2) A group verified question by question can pass in pieces and fail whole; verify the group.
(3) Unpublishing never rewrites a stored composition.

**Gate.** With two providers of different models, a 30-item batch publishes its confirmed items with no
person, leaves the doubted ones in one batch with reasons, and a sampled item a person rejects stops being
drawn. With one provider, everything escalates.

---

## Part I — Vocabulary

### Stage B — the database first

Today a learner who pastes "time" — a word the database already holds — waits for a dictionary call and
a model call, and then receives a **second "time" sense** because the model worded the definition
differently from the stored one (§1). After Stage D the database holds 10,000 words: nearly every word a
learner adds will already be there.

1. **Resolve known words in one query.** At the start of `VerifyUpload`, normalise every item's term the
   way the parser does (trim, lower-case, collapse spaces, strip trailing punctuation) and read the words
   with those lemmas, and their senses, in **one** `lemma = ANY($1)` query. Phrases the same way.
2. **Known word, no meaning given** → reuse its **primary sense** (the sense in a public curated deck,
   otherwise the oldest); add it to the learner's deck and schedule its review card. **No dictionary, no
   model.** Item note `known_word`: "Đã có trong kho từ — đã thêm".
3. **Known word, meaning given** → compare the meaning with each sense's `definition_vi` after
   normalising case, whitespace and punctuation (diacritics kept: "ban" and "bàn" are different words). A
   match reuses that sense. No match → **one** model call with the existing senses as candidates: "which
   of these does the learner mean, or is it a new meaning?" A candidate → reuse it. New meaning → today's
   path adds a sense to the **existing** word. No dictionary call either way.
4. **Unknown word** → today's path (dictionary → model → materialise), then **check again**: the model's
   lemma ("went" → "go") is looked up before any sense is created, and the judge prompt receives the
   existing senses of that lemma so a restated meaning reuses one instead of duplicating it.
5. **Tell the learner at once.** `Submit` runs the same single query and returns each item marked
   `known` or `checking`, so the list says which words are already in the database before the job runs;
   the existing worker nudge then finishes known words within seconds.
6. **Optional, in the box.** Spec first: `POST /vocabulary/words/lookup` with up to 200 terms returns which
   are known; the form marks pasted lines "đã có" as the learner types.

**Traps.** (1) The learner's own meaning stays on their upload item (`provided_meaning`) and is **never
written onto a shared sense**: that sense belongs to every learner. (2) A word with senses in several
parts of speech ("book") and no meaning given takes the primary sense; do not ask the model to guess.
(3) A known word must not wait behind the AI quota: when the model is exhausted (`queued` path), known
words still resolve from the database.

**Gate.** A test with counting fakes: pasting 30 words, 25 of them in the database, makes **zero**
dictionary and **zero** model calls for the 25, adds them to the learner's deck, and creates no second
sense for any of them; the 5 unknown words take the old path. In the browser, the 25 show "Đã có trong kho
từ" within seconds.

### Stage C — the word list, built once

**`cmd/vocabgen`**, a command of its own that **declares its config sections** (§1). Four resumable
steps, each caching to the scratch directory so a crash at word 7,000 does not start over:

1. **List.** Read the source list (D22-6), lemmatise, drop proper nouns, abbreviations, profanity and
   non-words, keep the first 10,000 by rank.
2. **Meanings.** 50 lemmas per model call (D22-7). Anything failing D22-9's checks is retried once, then
   listed for a person.
3. **Pronunciation.** IPA and the recording link from the dictionary, then Wikimedia Commons by file name —
   the sources and order of `cmd/seed/audio.go`, **reusing** its code. No recording, no link; the browser
   falls back as today (D21-8).
4. **Write** `db/fixtures/vocabulary/words-{a1,a2,b1,b2,c1}.json`, each headed with every source, licence
   and date.

**Traps.** (1) A recording without its credit page is not written. (2) Clamp model CEFR to A1–C1 and
flag disagreements with the list's rank band for the reviewer. (3) About 5 MB of JSON in total; keep
each file to a few MB.

**Gate.** 10,000 unique lemmas; at least 80 % with a credited audio link; the 2 % sample read and its
corrections applied.

### Stage D — loading 10,000 words

- `seedVocabularyWords` reads the fixtures instead of `content_data.go` and writes `audio_url`,
  `audio_attribution` and `audio_licence` into **version 1** of each sense.
- `pgx.Batch` in chunks of a few hundred: the Supabase pooler is a network hop per statement, and row by
  row this is tens of minutes.
- Decks per D22-10, idempotent on slug. `make seed` drops the `seed -audio` line.

**Gate.** A fresh `make seed` loads 10,000 words in minutes with no network call; any seeded word's
flashcard plays its recording and shows the credit.

---

## Part II — Foundation

### Stage E — the spine the courses need

New nodes (migration, `ON CONFLICT (namespace, code)` like `1700000781`), each with label, description,
CEFR and prerequisite — **23** in all:

- **Word formation** (vocabulary, 5): prefixes, suffixes, word families, conversion, compound words.
- **Expressions** (vocabulary, 3): idioms, fixed expressions, common spoken expressions.
- **Skill progressions** (skill, 15): Listening — words, phrases, sentences, conversations. Speaking —
  words, phrases, sentences, paragraphs. Reading — sentences, paragraphs, vocabulary in context, main idea
  and detail. Writing — sentences, paragraphs, linking words.

**Course map** — every node in exactly one course, kept in one table the seed reads:

| Course | Nodes |
|---|---|
| Grammar Foundations | parts of speech, nouns, articles, pronouns, adjectives, adverbs, prepositions, conjunctions, modal verbs, comparatives and superlatives, passive voice, conditionals, reported speech, gerunds and infinitives, participles, common grammar mistakes |
| English Tenses | tenses and the twelve tense nodes |
| Sentence Structure | sentence structure, subject–verb–object, questions and negatives |
| Clauses | clauses, relative clauses, noun clauses, adverbial clauses |
| Sentence Patterns | the thirteen pattern nodes |
| Vocabulary Foundations | essential everyday, common verbs, high-frequency words, topic vocabulary, collocations, synonyms and antonyms, academic, workplace |
| Word Formation | the five word-formation nodes |
| Phrasal Verbs & Expressions | phrasal verbs (grammar namespace) and the three expression nodes |
| Pronunciation Foundations | the eight pronunciation nodes |
| Reading, Listening, Speaking, Writing Foundations | each skill's node and its progression |

**Gate.** A test fails if any spine node is in no course or in two.

### Stage F — a lesson step that teaches

- Spec first: a `foundation_topic` activity kind, ungraded, weight 0 like `lesson_material` (WO 20),
  whose config is the topic's published body: objective, explanation, examples.
- `lesson` registers the kind; the web renders it with the component `/foundation/topics/{code}` already
  uses, not a copy.

**Gate.** A lesson opening with a topic shows the explanation and moves on with "Tiếp tục".

### Stage G — Foundation content for 93 nodes

1. `go run ./cmd/foundation -all` for the 70 existing and 23 new nodes.
2. Stage A verifies (D22-13): confirmed nodes publish; doubted nodes wait as one batch each.
3. **`cmd/foundation -export`** writes the published topics, exercises and quizzes to
   `db/fixtures/foundation/*.json`.
4. **`cmd/seed -foundation`** loads them with the approval they already had.

**Traps.** (1) The export lists every node without published content, so a course does not silently lose
a lesson. (2) A confidently wrong explanation is worse than a wrong quiz key because it teaches; any doubt
escalates.

**Gate.** After a reset, `make seed` publishes Foundation content for every node without a model call.

### Stage H — thirteen courses, five retired

- `cmd/seed` builds each course **only** through `lesson`'s `Author` contract (`EnsureCourse`,
  `EnsureUnit`, `EnsureLesson`, `SyncActivities`): units of related nodes, one lesson per node in
  prerequisite order, activities = topic step (F), exercises, quiz.
- The five old courses leave the seed; their content moves per D22-14. Retired slugs are not reused.
- Titles and cards in Vietnamese and English.

**Traps.** (1) No direct inserts into `learn.*` as the old course seed did. (2) Placement opens lessons
below the placed level (BR-LEARNING-11): check it still finds lessons when courses span several levels.

**Gate.** The catalogue shows thirteen courses in both locales; a lesson runs from explanation to quiz.

---

## Part III — Exams

### Stage I — the official format, sourced and enforced

The tests must be what a learner will sit on the day. Today the numbers are stored and nothing obeys
them (§1). This stage writes the format down once, from the official sources, and makes every step that
touches an exam read it.

#### I.1 The specification

- **One schema**, defined in the `exam` contract and used everywhere: per part — section, order, question
  count, groups and questions per group, options per question (or "typed" with a word limit), the allowed
  question types and their mix, the recording or passage shape (speakers, length, genre), word or speaking
  time limits, minutes, plays per recording; per version — section order, total time, scoring scale and
  the conversion used. The existing `exam_parts.constraints` column holds it; a migration rewrites the
  stored rows into the one schema (retiring `options_count` and `sub_questions_per_group`).
- **Every line cites its source** — a URL or handbook section and the date checked — in the migration
  comment and in `exam_versions.source_url` and `verified_at`. A person checks the finished table against
  the sources once and signs it off in this work order's handover notes.
- **Sources, checked before writing** (brief §11): TOEIC — ETS's official test content and examinee
  handbook; IELTS — ielts.org and the test-format pages of its owners; VSTEP — the Ministry of Education's
  VSTEP.3-5 format decision (believed to be Quyết định 729/QĐ-BGDĐT, 2015 — confirm the current text), not
  Circular 01/2014, which the row cites today.

#### I.2 What each exam must specify — to be confirmed against the source, not taken from here

| Exam | Details the spec must carry beyond today's counts |
|---|---|
| TOEIC LR | Part 2: three options, question and responses spoken only. Part 3: 13 conversations × 3, some with three speakers, the last sets tied to a graphic. Part 4: 10 talks × 3, some with a graphic. Part 6: 4 texts × 4, one sentence-insertion question per text. Part 7: single passages and double and triple passage sets in the published mix. Listening 45 min driven by the recording, Reading 75 min; one play |
| IELTS Academic | Listening: four parts of 10 (social conversation, social monologue, educational discussion, academic lecture) — completion, multiple choice, matching, map or plan labelling — 30 min. Reading: three passages, 40 questions, 60 min — True/False/Not Given, matching headings and information, completion, multiple choice. Writing: Task 1 describes a visual in at least 150 words, Task 2 an essay of at least 250. Speaking: Parts 1–3, one minute to prepare Part 2. Band 0–9 in halves |
| VSTEP 3-5 | Listening: 8 short announcements or messages, 3 conversations × 4, 3 talks × 5, played once, about 40 min. Reading: 4 passages × 10, 60 min. Writing: Task 1 a letter or email of at least 120 words, Task 2 an essay of at least 250. Speaking: social interaction, solution discussion, topic development, about 12 min. Each skill 0–10, the average mapped to level 3, 4 or 5 |

#### I.3 Enforced at every step

1. **Generation.** `questionbank` passes the part's spec to the generator: `GenerateRequest` gains the
   exam part, and the prompt is built from the spec — so "TOEIC Part 3" asks for a three-question
   conversation with the right speakers, not a generic listening item.
2. **Structure check.** `VerifyItemRequest.ExamConstraints` is filled from the spec on every exam item, so
   the verifier's existing structural checks finally run; Stage A's checklist reads the same spec.
3. **Question types.** D22-25's completion kind (typed answer, variants, word limit), True/False/Not Given
   as a fixed three-option choice, and matching — each with a grader and a runner renderer.
4. **Composition.** The composer fills each part's question-type mix, not just its count.
5. **Sitting.** In exam mode: section order and each section's time from the spec; one play per
   recording, **including TOEIC Parts 1–2**; the writing box shows the word count against the minimum;
   speaking shows preparation and response timers (IELTS Part 2's minute).
6. **Report.** The score on the published scale, labelled "ước tính" where D22-26 says so.

#### I.4 The rest of the structure

1. `IELTS_ACADEMIC_2026_R2` with the parts of I.2; the coarse `IELTS_ACADEMIC_2026` set
   `is_current = false`. A new code, not an edit: parts are what tests and attempts point at.
2. Blueprint `ielts_default`, with a CEFR mix like `toeic_default`.
3. `listed` on `assess.exams` and `assess.exam_versions` (default `true`); `false` for the three
   `mock-toeic-*` rows and the versions D22-16 leaves out.

**Traps.** (1) Do not copy directions, sample questions or audio from ETS, IELTS, the Ministry, Study4 or
prep books — the format is followed, the wording is ours (D22-26). (2) A detail the source does not state
(an exact Part 7 passage mix, a timing the handbook leaves open) is recorded as "not published" and left
to the blueprint, not guessed as if official. (3) Changing a part's spec after tests exist is a new
version code, never an edit.

**Gate.** A **format conformance test** per exam composes Test 1 and asserts, part by part, counts,
groups, options, question-type mix, section order and minutes, and plays per recording against the spec;
it fails if any generated item deviates. The signed-off spec table is in the handover notes with its
sources and dates. `GET /exam-versions` returns TOEIC, IELTS R2 and VSTEP with blueprints and parts.

### Stage J — numbered, disjoint fixed tests

- `assess.mock_tests` gains `number int` (null unless `mode = 'fixed'`) and a partial unique index on
  `(blueprint_id, number) WHERE mode = 'fixed'`.
- `ComposeFixedTest(blueprint, number)` returns the stored test, or draws each part **excluding every
  question group used by tests 1 … number−1**, seeded from `(blueprint, number)`. A part the bank cannot
  fill refuses with that part named (BR-EXAM-14); no row is written.
- `ComposeNextFixedTests(blueprint)` composes `max+1, max+2, …` until one cannot be filled. Called by
  Stages N and O.
- Spec first: `GET /exam-versions/{id}/tests` → fixed tests in order: `id`, `number`, `title` ("Đề 3"),
  question count, minutes, the caller's latest attempt (status, score).
- `distinct_tests_possible` stays the coverage number and is not confused with tests that exist.
- Amend BR-EXAM-13: "A fixed test is a numbered, stored composition shared by everyone; the fixed tests of
  one blueprint share no question."

**Traps.** (1) Exclude by **group**, not activity: a Part 3 conversation is three activities together.
(2) `ON CONFLICT` on the unique index, so two workers composing Test 6 end with one row.

**Gate.** A bank holding exactly five tests' worth yields Tests 1–5, refuses Test 6 naming the short part,
and no question appears twice.

### Stage K — practice mode for a mock test

- Spec first: `POST /mock-tests/{id}/attempts` accepts the body `POST /exams/{id}/attempts` does — `mode`,
  `chosen_duration_minutes`, `unlimited`, `sections`.
- `StartMockTestAttempt` reuses `sittingDuration` and `StartSitting`'s section narrowing, so the two
  paths cannot drift. The report carries `elapsed_seconds`.

**Gate.** Practising Test 2, Reading only, no time limit: it counts up and reports the time taken.

### Stage L — the hub

`/exams` becomes three levels:

1. **Exams** — cards: name, format ("200 câu · 120 phút"), how many tests, the learner's best score.
2. **Tests of one exam** — Đề 1 … Đề N with done and score, plus **"Đề ngẫu nhiên"** (D22-18) and
   **"Tạo đề tùy chọn"** (the WO 21 composer's custom mode, moved here).
3. **One test** — "Thi thử" (full, timed) or "Luyện tập" (the settings sheet: sections, duration, no
   limit), then `ExamSittingRunner`.

The WO 21 "Đề thi thử" tab is absorbed. Empty states say what is missing ("IELTS chưa có đề — đang được
biên soạn"). Both locales, 320 and 390 px.

**Gate.** TOEIC → Đề 3 → Luyện tập (Reading only, no limit) in four taps; TOEIC → Đề ngẫu nhiên in two.

### Stage M — media the parts need

- **Part 1 photos (D22-21).** `db/fixtures/exams/toeic-part1-photos.json`: URL, credit page, licence,
  human description; CC0 and CC BY only, refused at load otherwise. The prompt gets the description, never
  the image. The credit shows whenever the photo does.
- **Task 1 charts (D22-22).** The generator returns `{chart_type, title, series}`; a renderer in `media`
  draws the SVG into `fluentra-media`; the body carries `image_url` and keeps the series. A chart whose
  data does not add up fails Gate 1.
- **Two voices (D22-23).** Scripts carry `turns: [{speaker, text}]`; `cmd/tts` and the render job voice
  each turn by speaker (`speech.tts_voice` plus a second key, added to `.env.example` and every command's
  declared defaults first) and concatenate. Scripts without turns render as today.

**Gate.** A Part 1 item shows a credited photo; a Task 1 item a rendered chart; a Part 3 conversation
plays in two voices.

### Stage N — five tests per exam, frozen

1. **`cmd/examgen`** (declares its config sections): `-exam TOEIC -tests 5` generates each part's
   questions for five tests plus the margin, sends every item through Stage A, and repeats for any part
   still short until five disjoint tests can be composed.
2. A person works through the escalated batches only — tens or hundreds of items, not ~2,200.
3. **`cmd/examgen -export`** writes every published question of the three exams — body, tags, group id,
   media links, the approval it had — to `db/fixtures/exams/{toeic,ielts,vstep}.json`.
4. **`cmd/seed -exams`** loads them (versions, approvals, bank questions, activities in the bank course),
   then runs `ComposeNextFixedTests` per blueprint. Idempotent on the fingerprint (BR-QUESTIONBANK-02).
5. Listening audio renders afterwards with `make tts`.

**Traps.** (1) Original generated items only — nothing from real TOEIC, IELTS or VSTEP papers, Study4 or
prep books (brief §11). (2) Fixtures must not reach anything the web bundle imports: they hold keys.
(3) Load in `pgx.Batch` chunks.

**Gate.** After a reset, `make migrate-up && make seed && make tts` gives TOEIC, IELTS and VSTEP five
tests each, sittable in both modes, with no network call besides TTS.

### Stage O — more every day

- Job `questionbank.generate_daily` in the worker's cron (lock in §4): picks the next exam in the rotation
  (D22-20), generates one test's worth, sends it through Stage A, stops.
- Switch and per-run cap in `.env.example` and the worker's declared defaults, off by default. Spend goes
  through the AI budget; an exhausted budget skips the day.
- After **any** publish — by the verifier or a batch approval — call `ComposeNextFixedTests`: Test N+1
  appears the day a full disjoint test's worth exists.
- The admin question screen shows, per exam: tests published, published today by the verifier, waiting
  review, parts short for the next test.

**Traps.** (1) If an exam's escalated batches from two earlier days are unreviewed, skip it: generating
faster than anyone reads the doubts only grows a backlog. Days the verifier confirms everything never trip
this. (2) The fingerprint catches exact duplicates; the prompt receives recent stems of the part to steer
away from near-duplicates.

**Gate.** With the job and auto-publish on and two providers of different models, each exam gains a test
on its day **without anyone opening the review queue** when every item is confirmed; with doubts, the
test appears when that batch is approved.

---

## 5. Final gate

WO 19 §2 in full, plus:

1. **The whole flow from nothing**, on a database reset with `scripts/reset-dev-database.sql`:
   `make migrate-up && make seed && make tts` finishes offline except TTS and gives: the two accounts,
   10,000 words with audio links, thirteen courses, and five tests each for TOEIC, IELTS and VSTEP.
2. Stage B's gate: known words cost no dictionary and no model call.
3. Stage I's format conformance test for TOEIC, IELTS and VSTEP, and the signed-off specification with
   its sources.
4. A test that the verifier refuses to publish a choice item whose blind answer differs from its key,
   and escalates everything when only one provider model is configured.
5. Every new screen at 320 and 390 px in both locales; Playwright paths for "TOEIC → Đề 1 → Thi thử →
   submit → report", "IELTS → Đề ngẫu nhiên", a Foundation lesson from explanation to quiz, and pasting a
   known word.
6. `make gen-check` and `make gen-check-web` after committing; golangci-lint with both build tags;
   Spectral; `make docs`; `go-arch-lint` in the Linux container.

## 6. What to cut, in order

1. Stage B step 6 (the live "đã có" marks in the box; the list still says it within seconds).
2. Stage O's admin line, then Stage A.5's rate display (the sample and reports still run).
3. D22-10's per-level decks (one "all words" deck works).
4. Stage M's two voices (one voice is intelligible, just less real).
5. Stage E's skill progressions (one lesson per skill to start).
6. Stage O itself — five fixed tests per exam still ship.

If D22-25's completion kind has to wait, the IELTS tests ship labelled "IELTS-style (multiple choice)"
until it lands — never as "IELTS".

Not on this list: Stage A's verifier, checklist, marking and sample (auto-publishing without them is
publishing unchecked content); Stage I's specification and its enforcement (a test that does not follow
the format is not the test the learner will sit); Stage B steps 1–5; the 10,000 words; the thirteen courses; five tests per
exam; the exam hub.
