---
doc_type: plan
phase: 4
status: planned
last_verified: 2026-09-20
---

# Phase 4 — Foundation, AI content generation, and the exam system

**Purpose.** Give Fluentra three content systems that share one spine: a **Foundation** that teaches the
English a learner actually needs, a **generation pipeline** that produces new material for that learner
every day, and an **exam system** built on a reusable **Question Bank** rather than on hand-written mock
tests.

The sentence that shapes every decision below is from the brief's §12:

> Foundation knowledge should be reusable across all exams instead of duplicating the same
> grammar/vocabulary content for TOEIC, IELTS, VSTEP, etc.

That is only achievable if there is **one identifier for a piece of English knowledge** that a Foundation
lesson, a generated drill, and a TOEIC question all point at. Building the Foundation, the generator and
the Question Bank as three separate content stores would satisfy every other requirement in the brief and
still fail §12. Section 6 below is that identifier; everything else hangs off it.

**Read first.** [ADR-0015](../adr/ADR-0015-content-exercise-core.md),
[ADR-0011](../adr/ADR-0011-ai-provider-abstraction.md),
[ADR-0012](../adr/ADR-0012-prompt-versioning.md),
[ADR-0018](../adr/ADR-0018-media-presigned-upload.md), and the `AGENT.md` of `content`, `lesson`,
`learning`, `grammar`, `questionbank`, `exam` and `platform/ai`.

**This is a programme plan, not a work order.** It locks the decisions that all twelve steps share. Each
step gets its own work order at the detail level of [work order 15](phase-3-work-order-15.md); the first
is [work order 16](phase-3-work-order-16.md). Work orders for later steps are written as their
predecessors land, because specifying P9 before P7 exists is invention, not planning.

---

## 1. What exists today

Checked on 2026-09-20 against `feat/phase-3-work-order-15` at `abbe8af`.

| Piece | State | Where |
|---|---|---|
| Content authoring core | **Built.** `content.content_items` + `content_versions` (jsonb `body`, `cefr_level`, `media_refs`), status enum `draft`/`in_review`/`approved`/`published`/`archived`, published versions immutable by trigger | `db/migrations/content/1700000190` |
| Hierarchical taxonomy | **Built, barely used.** `content.taxonomies` (`namespace`, `code`, `label`, `parent_id`, unique on `(namespace, code)`) and `content.content_tags`. One namespace seeded: `course_topic`, 20 rows | `1700000190`, `1700000746` |
| Course structure | **Built.** `learn.courses` to `course_units` to `lessons` to `activities`; `activities.content_version_id` is the join to content | `db/migrations/lesson/1700000200` |
| Lesson prerequisites | **Built.** `learn.lesson_prerequisites` (`lesson_id`, `requires_lesson_id`, `min_score`) with cycle detection in Go | `1700000200`, `lesson/domain/graph.go` |
| Generation pipeline | **Built, and the most valuable thing here.** Generate, parse, own-answer-scores-full-marks, structure, blind solve, deduplicate, redact, publish — over 6 activity kinds. Slot targets, ceilings, running-low growth, 3 retries | `learning/service/practice_pool.go`, `exam_pool.go`, `placement_pool.go` |
| AI platform | **Built.** Provider registry, routing, prompt versioning (`prompts/*.v1.md`), exact-hash cache, per-task budgets, usage accounting. 10 tasks registered | `internal/platform/ai/` |
| Media platform | **Built.** Piper TTS, OpenAI-compatible ASR (`HTTPTranscriber`), dispatch | `internal/platform/media/` |
| Object storage | **Built.** 3 buckets (`fluentra-avatars`, `fluentra-media`, `fluentra-exports`), presigned upload, ADR-0018 | `internal/platform/storage/` |
| Async work | **Built.** River queue, cron, transactional outbox with backoff and dead-lettering, trace propagation | `internal/platform/job/`, `db/migrations/job/` |
| Learner upload precedent | **Built.** `POST /me/vocabulary/uploads` returns 202, an hourly job processes it, the learner watches counts move. `skill.vocab_uploads` + `vocab_upload_items` | `db/migrations/vocabulary/1700000270` |
| `exam` module | **Built, but hard-coded.** `assess.exams` (`format`, `total_minutes`) to `exam_sections` (`position`, `skill`, `item_count`, `item_kinds` jsonb) to `exam_attempts` to `score_reports`. **No exam version, no parts, no question types, no blueprint.** Three seeded rows, all `mock_toeic`, all four sections of 3/2/4/4 items — which is not the TOEIC format | `db/migrations/exam/1700000630` |
| `questionbank` module | **Specification only.** `AGENT.md` names 5 tables in `assess`, 5 endpoints, a `Reader` contract. No migration, no Go beyond two `doc.go` files | `internal/modules/questionbank/` |
| `grammar` module | **Specification plus a grader.** `AGENT.md` names 5 tables in `skill` and 7 business rules. Only `service/grader.go` exists; no migration | `internal/modules/grammar/` |
| `reading` module | **Specification only** | `internal/modules/reading/` |
| Permissions | `content.read.published`, `content.create`, `content.edit`, `content.review`, `content.publish`, `moderation.read`, `moderation.act` exist. **`questionbank.*` does not** — `questionbank/AGENT.md` §6 cites three permissions that are not in `permissions.go` | `rbac/contract/permissions.go` |
| Highest migration | `1700000775` | `db/migrations/studio/` |

### Three things follow from this table

**The generation pipeline in the brief's §2 and §8 is built.** `practice_pool.go` already does
generate, validate, deduplicate, quality check, publish — including a blind solve that catches a wrong
answer key by making the model answer the redacted item and disagreeing with it. It is **unexported**, and
its only callers are inside `learning/service`. Most of P6 and P11 is lifting that pipeline into something
the Foundation, the Question Bank and user-uploaded resources can all call — not writing a new one.

**The exam module models an exam as one flat list of sections.** The brief's §5 requires
Exam to Version to Section to Part to Question Type, and forbids hard-coding structures. The existing
`assess.exams` seed hard-codes three. This is the one place where the existing schema must be extended
rather than reused, and §7 below says how without breaking the attempts already stored against it.

**`content.taxonomies` is the spine the brief needs, already built and almost unused.** It is a namespaced
hierarchy with a parent pointer and a uniqueness constraint. It needs one thing it does not have —
prerequisite edges, which are a DAG and not the same shape as `parent_id` containment. See §6.

---

## 2. Exam structures, verified

The brief's §11 requires current specifications from authoritative sources, with source, URL and
verification date recorded, and warns against assuming old formats are current. **That warning was
correct and it caught a real error.** TOEFL iBT changed substantially in January 2026: it is now
about 1.5 hours with section-adaptive routing and task types (`Complete the Words`, `Build a Sentence`,
`Listen and Repeat`) that did not exist in the 2023 format. A plan written from memory would have
specified the superseded structure.

All rows verified **2026-09-20**.

| Exam | Structure | Source |
|---|---|---|
| **TOEIC Listening & Reading** | 7 parts, 200 questions, about 2h. Listening Parts 1-4 = 6 / 25 / 39 / 30 questions, 45 min. Reading Parts 5-7 = 30 / 16 / 54 questions, 75 min. Unchanged for 2026 | [ETS Global](https://www.etsglobal.org/dz/en/help-center/test-content/format-questions-toeic-listening-reading), [990prep](https://990prep.com/en/guides/toeic-test-format-breakdown) |
| **IELTS Academic** | 4 sections, 2h45m. Listening 4 parts of 10 = 40 questions, about 30 min. Reading 3 passages, 40 questions, 60 min. Writing 2 tasks (150 words / about 20 min; 250 words / about 40 min), 60 min. Speaking 3 parts, 11-14 min | [ielts.org](https://ielts.org/take-a-test/test-types/ielts-academic-test) |
| **TOEFL iBT** | **Changed January 2026.** About 1.5h, no scheduled break. Reading 50 items / 30 min; Listening 47 items / 29 min; Writing 12 items / 23 min; Speaking 11 items / 8 min. Reading and Listening are **section-adaptive** (Stage 1 to Stage 2 routing). Scored 1.0-6.0 alongside legacy 0-30 / 0-120 | [ETS](https://www.ets.org/toefl/test-takers/ibt/about/content.html), [Study.com](https://study.com/resources/all-toefl-test-changes.html) |
| **Cambridge B2 First** | 4 papers. Reading and Use of English 7 parts / 52 questions / 1h15m (40% of grade); Writing 2 parts / 1h20m (20%); Listening 4 parts / 30 questions / about 40 min (20%); Speaking 4 parts / 14 min (20%) | [Cambridge English](https://www.cambridgeenglish.org/exams-and-tests/qualifications/first/format/) |
| **VSTEP 3-5** | Listening 3 parts / 35 questions / about 40 min; Reading 4 passages of 10 = 40 questions / 60 min; Writing 2 tasks (letter 120+ words = one third; essay 250+ words = two thirds) / 60 min; Speaking 3 parts / 12 min. Circular 01/2014/TT-BGDDT | [ULIS-VNU](https://vstep.vnu.edu.vn/test-format/) |
| **Vietnam THPT English** | 40 multiple-choice questions, 50 min, 0.25 points each. Four task shapes: fill a notice or advert, ordering, cloze, reading comprehension. Format introduced 2025 under the 2018 curriculum; 60-65% at recall and comprehension level | [Thu vien Phap luat](https://thuvienphapluat.vn/phap-luat/ho-tro-phap-luat/cau-truc-de-thi-tieng-anh-tot-nghiep-thpt-nam-2026-cap-nhat-moi-nhat-cach-tinh-diem-mon-tieng-anh-t-431248-267517.html), [VnExpress](https://vnexpress.net/48-ma-de-tieng-anh-thi-tot-nghiep-thpt-2026-chi-tiet-day-du-nhat-5083987.html) |

**These rows are data, not schema.** They belong in seeded `exam_versions` rows with their own
`source_url` and `verified_at`, not in Go constants — which is the brief's "do not hard-code exam
structures" made concrete. A format change then becomes a new version row, and the mock tests already
taken against the old one stay reproducible.

**Two consequences worth naming now.**

TOEFL's section-adaptive routing cannot be represented by a fixed blueprint. Either TOEFL ships as a
non-adaptive approximation clearly labelled as such, or the blueprint model carries stage routing. §7
recommends the first for P8 and defers the second; presenting a fixed test as adaptive would be the worse
of the two.

TOEIC Part 3 and Part 4, and IELTS Listening, are **passage-grouped**: 39 Part 3 questions are 13
conversations of 3. Sampling questions independently would produce a test that cites conversations it
never plays. Group integrity is a constraint on the sampler, not a nice-to-have — see §7.

---

## 3. Decisions taken

Change these only with a reason written down.

| # | Decision | Why |
|---|---|---|
| D1 | **One knowledge spine** in `content.taxonomies`, extended with a prerequisite DAG. Foundation topics, generated items and Question Bank questions all tag against it | This is the only mechanism that delivers §12. Three parallel taxonomies would each be defensible and would jointly make Foundation reuse impossible |
| D2 | `skill.grammar_points` keeps the role `grammar/AGENT.md` gives it — rule statement, canonical examples, common errors, error tagging — and **references** a spine node rather than being a second taxonomy | Honours the existing spec without forking the DAG. The learning path needs one graph, not one per strand |
| D3 | The Question Bank is **`assess.questions`, per `questionbank/AGENT.md`**, extended with the columns the brief's §6 requires that the spec omits: exam, version, section, part, topic, tags, provenance, media, group id, fingerprint | The spec was written before the brief. Extending a specified table beats inventing a parallel one |
| D4 | A Foundation item's nine required fields (objective, explanation, examples, exercises, quiz, CEFR, prerequisite, related topics, review questions) live in `content_versions.body` jsonb against a **versioned JSON Schema**, not in new columns | `body` is already jsonb, GIN-indexed and immutable once published. Nine columns would freeze a shape the brief expects to grow |
| D5 | **Exam versions are data.** Sections, parts, question types, counts, durations, scoring and distributions are rows with `source_url` and `verified_at` | The brief's §5 and §11. Also the only way a 2026 TOEFL change does not invalidate 2025 score reports |
| D6 | **Mock tests are compositions, not copies.** A mock test stores the *list of question ids* it drew plus the blueprint and seed. A retake replays the stored list; a new test draws again | The brief's §7. Copying question rows per test is how a bank of 500 becomes 20 unusable duplicates |
| D7 | **Nothing AI-generated reaches a learner at `published` without passing all eight checks in the brief's §8.** The existing six become eight; the state machine is the one `content.authoring_status` already has | The brief's §8, and `content` already enforces it |
| D8 | Third-party URLs are **referenced, not copied**: metadata, embed, and the learner's own notes. Extraction runs only on files the user uploaded or URLs that are unambiguously theirs | The brief's §3. Also the difference between a study tool and a piracy tool |
| D9 | Media processing is **derivative-only**: the original is preserved untouched, every optimised rendition is a derived object that can be regenerated and deleted | The brief's §4. A lossy pipeline that overwrites the source destroys a user's upload on a bug |
| D10 | `questionbank.read`, `questionbank.create` and `questionbank.review` are **added to `permissions.go`** in the work order that first mounts a questionbank endpoint | They are cited by `questionbank/AGENT.md` §6 and do not exist. Mounting a handler against a permission nobody holds fails open or fails silently |

---

## 4. Open — the owner decides before P8

These do not block P1-P7. They do block shipping an exam.

1. **Which exam families ship first.** Six are specified in §2. TOEIC and VSTEP are the ones Vietnamese
   learners sit most; TOEFL is the one that just changed and will cost the most to model. Recommendation:
   TOEIC L&R and VSTEP 3-5 first, Vietnam THPT third (it is 40 questions and one skill), IELTS fourth.
2. **Whether TOEFL ships non-adaptive**, clearly labelled, or waits for stage routing.
3. **Scoring fidelity.** A real TOEIC scaled score comes from an equating table Fluentra does not have.
   Ship a clearly-labelled estimate, or ship raw scores only? Claiming "TOEIC 780" from a sampled mock
   test is a claim the system cannot support.
4. **Who reviews generated questions**, and their throughput. §8 requires human review before publication
   for exam questions. At 500 questions per exam version this is the binding constraint on the whole
   programme, not the generation.
5. **Copyright posture on uploaded PDFs.** D8 covers third-party URLs. A user uploading a textbook PDF is
   a different question, and the honest answer may be "extract for that user only, never into the shared
   bank."

---

## 5. Module map

The brief's §9 suggests nine domains. Seven already exist here under different names. **No new
microservice; no new top-level structure.**

| Brief's domain | Fluentra module | State |
|---|---|---|
| `content` | `content` | Built. Gains the spine (§6) and Foundation body schemas |
| `content_generation` | `learning` pools, promoted | Built but unexported. P6 lifts it to a callable generation service |
| `learning` | `learning` | Built. P12 consumes the spine for paths |
| `media` | `platform/media` + `platform/storage` | Built for audio. P3 adds image, document and video renditions |
| `resource_ingestion` | **new module `resource`** | New. Schema `resource`. Follows the `vocab_uploads` precedent exactly |
| `question_bank` | `questionbank` | Specified, unbuilt. P7 |
| `exam` | `exam` | Built flat. P8 adds version, part and blueprint |
| `mock_test` | `exam`, table `assess.mock_tests` | Not a separate module: it shares `assess`, every foreign key, and the attempt lifecycle. A module boundary here would be crossed on every call |
| `assessment` | `learning` + `exam` | Built (grading, score reports) |

One new module (`resource`), one promoted service (generation), one extended module (`exam`), two
specified-but-unbuilt modules finally built (`questionbank`, `grammar`).

---

## 6. The spine

The single most important table in this programme.

```text
content.taxonomies                      -- exists
  namespace  'grammar' | 'vocabulary' | 'pattern' | 'pronunciation' | 'skill' | 'course_topic'
  code       'PRESENT_PERFECT'          -- stable, referenced everywhere, never renamed
  parent_id  containment (Tenses > Present Perfect)

content.taxonomy_prerequisites          -- NEW, work order 16
  node_id, requires_node_id, created_at
```

`parent_id` and `requires_node_id` are **different relations** and this is the mistake to avoid. "Present
Perfect is inside Tenses" is containment. "Present Perfect requires Past Simple" is a prerequisite. The
brief's §1 example — Tenses, then Present Simple, Present Continuous, Past Simple, Present Perfect,
Future forms — is a prerequisite chain whose members share a parent. One column cannot carry both.

Prerequisites form a **DAG**: cycles must be refused at write time. `lesson/domain/graph.go` already has
`DetectCycle` with 3-colour DFS; P1 reuses the algorithm rather than writing a second one.

**Everything references a node code:**

- a Foundation item, through `content_tags`
- a generated drill, so the generator can be asked for "a `PRESENT_PERFECT` item at B1"
- a Question Bank question, so a TOEIC question and an IELTS question about the present perfect are
  *the same knowledge*, which is §12
- `skill.grammar_points`, for rule detail and error tagging (D2)
- a learner's mastery and weakness profile, so "weak areas" in §2 is a set of node codes

**The codes are an API.** Once a question is tagged `PRESENT_PERFECT`, renaming that code breaks every
row pointing at it. Codes are `SCREAMING_SNAKE`, unique per namespace, and additive only.

---

## 7. The exam model

`assess.exams` today is exam to sections. The brief needs five levels. The extension, preserving what is
already stored:

```text
assess.exams              -- exists: slug, title, level, format, total_minutes
  assess.exam_versions    -- NEW: code ('TOEIC_LR_2026'), effective_from, source_url,
                          --      verified_at, scoring jsonb, is_current
    assess.exam_parts     -- NEW: version_id, section, part_number, question_type,
                          --      question_count, duration_minutes, group_size
assess.blueprints         -- NEW: version_id plus difficulty, topic and skill distribution
assess.mock_tests         -- NEW: blueprint_id, seed, composition jsonb (question ids)
```

`assess.exam_sections` stays where it is; existing `exam_attempts` keep resolving. A version row carries
the §2 research: `source_url` and `verified_at` are columns, so "verify current specifications" becomes a
query rather than a promise.

**Sampling honours group integrity.** `group_size` on a part says a Part 3 slot draws **one conversation
of 3 questions**, not 3 questions. The sampler's unit is the group; a group is atomic. Getting this wrong
produces tests that reference audio they never play, and it is not visible until a learner sits one.

**The bank must be able to fill the blueprint.** A TOEIC version needs 200 questions per test drawn from
a bank large enough that two tests differ meaningfully. The brief says it plainly: do not claim unlimited
unique tests unless the Question Bank contains enough unique questions. P9 therefore ships a
**coverage report** — per part, how many questions exist, how many tests can be composed before reuse —
and the UI states the real number. A "generate a new test" button that silently returns the same
questions is worse than a button that says the bank is thin.

---

## 8. The four pipelines

All four are async (`platform/job`), all four end at the same eight checks, all four write versioned
traceable content.

**Generation (P6, P10)** — lift `practice_pool.go` into a generation service that takes
`(node_code, cefr, kind, count)` and returns verified items. Daily generation (P10) is that service driven
by learner level, completed content, weak areas and learning goal, where "weak areas" is a set of spine
node codes. Every generated item records prompt version, model, provider and request id — ADR-0012 and
`ai.ai_requests` already carry this; the item must store the link.

**Ingestion (P2, P4)** — the `resource` module. Upload via presigned PUT (ADR-0018) or submit a URL,
producing a row at `pending`; a job extracts, classifies and generates; the learner watches the status
move. This is `skill.vocab_uploads` again, a shape that already works in production here. Per D8, a
third-party URL is referenced, not scraped into the bank.

**Media (P3)** — `platform/media` handles audio. Adds: images (resize, compress, thumbnail, and
**preserve readable text** — the brief is explicit, so text-bearing images take a near-lossless path
rather than the photo path), documents (preserve original, extract text, preview), video (preserve
original, HLS ladder, thumbnails, transcript via the existing `HTTPTranscriber`). Per D9, originals are
never overwritten.

**Quality (P11)** — the brief's §8 eight checks, in order: schema, answer, duplicate, CEFR, difficulty,
exam structure, provenance, human review. Six exist in `practice_pool.go`. The two new ones are
**CEFR and difficulty validation** (does a B1-labelled item actually sit at B1) and **exam-structure
validation** (does this question match the part it claims). States are the existing
`content.authoring_status`: generated maps to `draft`, validated to `in_review`, human reviewed to
`approved`, published to `published`.

---

## 9. Ordering

The brief's P1-P12, mapped to work orders. Each is independently testable and leaves the system working.

| Step | Work order | Delivers | Gate |
|---|---|---|---|
| P1 | **WO 16 — landed** | The spine: namespaces, prerequisite DAG, cycle refusal, Foundation body schemas, read API, seeded topic codes | Both of the brief's example chains resolve in prerequisite order |
| P2 | **WO 17** | `resource` module: upload and import, validate, status lifecycle | A PDF upload reaches `validated` and its owner can see it. **Not** `extracted` — extraction is P4 |
| P3 | WO 18 | Image, document and video renditions, originals preserved | A 4K video plays at 3 bitrates; original byte-identical |
| P4 | WO 19 | Extraction, transcription and classification onto spine nodes | An uploaded PDF yields tagged text at a CEFR level |
| P5 | WO 20 | Foundation content across all five strands | Every seeded node has objective, explanation, examples, exercises, quiz, review questions |
| P6 | WO 21 | Generation service, lifted and callable | `learning`'s pools call the shared service; behaviour unchanged |
| P7 | WO 22 | `questionbank`: tables, permissions, authoring, grouping, fingerprint | 200 TOEIC-shaped questions, no duplicates, groups intact |
| P8 | WO 23 | Exam versions, parts and blueprints, with §2 seeded and sourced | TOEIC version reproduces 6/25/39/30 and 30/16/54 |
| P9 | WO 24 | Mock test generator and coverage report | 20 TOEIC tests, measured overlap, honest count |
| P10 | WO 25 | Daily generation driven by the learner | Weak-area drills appear next morning |
| P11 | WO 26 | The remaining two checks and the review queue | A mislabelled-CEFR item is refused |
| P12 | WO 27 | Learning path from the spine | A learner's path respects prerequisites |

**P1 is on the critical path for everything.** P2-P4 (ingestion and media) are independent of P7-P9
(bank and exams) and can run in parallel by different hands. P5 depends on P1; P10 depends on P6 and P12.

---

## 10. Risks, and what to cut

| Risk | Mitigation |
|---|---|
| **Human review is the bottleneck.** §8 requires review before publishing exam questions; 6 exams at 500 questions is thousands of items | Decide §4.4 before P7. Consider trusted-reviewer tiering as `studio` already does for creators |
| **The spine's codes get renamed** after content points at them | Codes are additive and immutable; add a deprecation flag, never a rename. Enforce in the work order, not by convention |
| **Exam formats change under us** — TOEFL just did | `verified_at` and `source_url` are columns; a quarterly job flags versions unverified for 180 days |
| **The bank is too thin** to generate distinct tests | The coverage report ships *with* the generator (P9), not after. Never claim what the bank cannot back |
| **AI cost** at daily-generation scale | `ai.ai_budgets` per task exists and is enforced. Set a budget for each new task *in the same migration* that adds the task |
| **Video transcoding cost and latency** | P3 is the most expensive step for the least learning value. It is the first thing to cut: ship audio, documents and images, and treat video as reference-only until demand is real |
| **Scope**: this brief is 12 work orders, comfortably a quarter | The ordering above leaves a working system after every one. If the programme is cut short, P1 + P5 + P7 + P8 + P9 is a complete exam product without ingestion or daily generation |

**If only three steps can be built: P1, P7, P8.** The spine, the bank, and real exam structures. That is
the part with no workaround — Foundation content can be seeded by hand and generation can stay where it
is, but a bank without a spine can never deliver §12.

---

## 11. What this plan does not answer

- The Foundation's actual **content** — the wording of 200 or more explanations. P5 is a content project
  with an editorial owner, not an engineering step, and it is sized in that work order.
- **Adaptive testing** (the brief's §7 "adaptive tests"). Real adaptivity needs item response theory,
  which needs `question_stats` populated from real attempts. It is unreachable until P7 has been live long
  enough to accumulate them. Until then "adaptive" means weak-area weighted, and should be labelled that.
- **Pronunciation scoring** beyond what `speaking` already does — see work order 14 §6.
- **Whether any of this is sold.** The marketplace in work order 15 prices community courses; whether an
  exam bank is free, included, or paid is a product decision with its own work order.
