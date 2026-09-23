---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-23
---

# Phase 3 — work order 22: real mock tests — TOEIC, IELTS, VSTEP, five tests each, and more every day

**Purpose.** Work order 19 built the exam machinery — versions, parts, blueprints, a question bank, a
composer — and work order 21 put a screen on it. A learner still cannot sit a TOEIC test: the bank holds
no questions, a blueprint may store only one fixed test, IELTS has no blueprint, and the exam hub lists
three generic "Mixed-skill practice exam" entries told apart by level. This work order makes the exam hub
what the owner asked for:

1. The learner picks an **exam** (TOEIC, IELTS, VSTEP), not a level.
2. Each exam has **at least five complete tests** from a fresh `make seed`.
3. The learner picks **which test** to sit — Test 1 to Test N — or a **random** one, in exam mode or
   practice mode.
4. **New tests appear over time**: a daily job generates questions, a reviewer approves them, and each
   time a full test's worth is published the next numbered test is composed.

**Read first.** WO 19 §§F–H; WO 21 Stage E; `exam`, `questionbank` and `content` `AGENT.md` (BR-EXAM-12
to 16, BR-QUESTIONBANK-01 to 05, BR-CONTENT-10); the owner's content-system brief of 2026-09-20 §§5–8
and §11 (verify exam formats from official sources; never silently publish AI exam questions).

---

## 1. What exists today

Verified 2026-09-23 against the code and migrations.

| Fact | Where |
|---|---|
| Six exam versions: TOEIC LR, VSTEP 3-5, IELTS Academic, Cambridge B2 First, THPT 2026 (current), TOEFL iBT (not current) | `db/migrations/exam/1700000860_create_exam_structures.sql` |
| Two blueprints only: `toeic_default`, `vstep_default`. **IELTS has none**, so no IELTS test can be composed | same |
| IELTS parts are coarse: Listening is one 40-question part with group size 1, Reading likewise. The real test has four recorded parts and three passages | same |
| **One fixed test per blueprint.** `ComposeMockTest` with `mode=fixed` returns the stored row if one exists (`findFixedMockTest(blueprint.ID)`) | `exam/service/composer.go` |
| No endpoint lists a version's fixed tests. `POST /mock-tests` composes; `POST /mock-tests/{id}/attempts` starts one | `api/openapi/openapi.yaml` |
| `StartMockTestAttempt` takes no body: a mock test is always sat in exam mode — no practice mode, no section choice, no "no time limit" (BR-EXAM-17) | `composer.go` |
| The exam hub (`ExamList`, first tab of `/exams`) lists `assess.exams`: three `mock-toeic-a2/b1/b2` rows renamed "Mixed-skill practice exam", plus `toeic-lr-2026` and `vstep-3-5` | `1700000860`, `1700000870` |
| The question bank holds **0 questions**; no job generates any. Generation is `POST /admin/questions/generate`, one part at a time | dev DB; `questionbank` |
| A generated question is a draft until a person approves it (BR-QUESTIONBANK-04, BR-CONTENT-10). The review queue approves **one item at a time** | WO 21 Stage C |
| TOEIC Part 1 (`photo_description`) needs a photograph; IELTS Writing Task 1 needs a chart. Nothing produces either | — |
| TTS renders a listening script in **one voice** (`-voice`, `speech.tts_voice`); no speaker turns | `cmd/tts` |

### How many questions "five tests" is

Five tests that share no question — otherwise Test 3 is partly Test 1 again.

| Exam | Per test | Five tests | With a 20 % rejection margin |
|---|---|---|---|
| TOEIC LR | 200 (Part 1 6, Part 2 25, Part 3 13×3, Part 4 10×3, Part 5 30, Part 6 4×4, Part 7 54) | 1,000 | ~1,200 |
| IELTS Academic | 40 listening + 40 reading + 2 writing + 3 speaking | 425 | ~510 |
| VSTEP 3-5 | 35 listening + 40 reading + 2 writing + 3 speaking | 400 | ~480 |
| **Total** | | **1,825** | **~2,200 generated items** |

That is too many to hand-author and too many to generate on every `make seed`. Hence D22-3.

---

## 2. Decisions

| Question | Default |
|---|---|
| D22-1. What the learner chooses first | **The exam** — TOEIC, IELTS, VSTEP — as cards. Level is metadata shown on the card, never the choice. The three "Mixed-skill practice exam" rows leave the hub (D22-7) |
| D22-2. Which exams are in scope | **TOEIC LR, IELTS Academic, VSTEP 3-5.** Cambridge B2 First, THPT and TOEFL keep their version rows but are not listed to learners until they get the same treatment |
| D22-3. How five tests reach a fresh database | **Generate once, review once, freeze into fixtures.** The questions are generated on a dev database, reviewed, and exported to `db/fixtures/exams/*.json`. `make seed` loads the fixtures offline and records the approval they already had. No model call and no network on `make seed` |
| D22-4. Are fixed tests disjoint | **Yes.** Fixed test N draws only from questions no earlier fixed test of the same blueprint uses. A bank that cannot fill the next test does not get one |
| D22-5. What "random" means | **A freshly composed test** (existing `mode=random`), preferring questions the learner has not seen. Not "pick one of Test 1–5 at random", which is one tap of the learner's own |
| D22-6. Who approves generated questions | **A person, as today** (BR-QUESTIONBANK-04). What changes is the unit: a reviewer approves a **batch** — one generated test's worth for one part — and rejects individual items inside it. Auto-publishing is not in this work order; see D22-9 |
| D22-7. The three old "Mixed-skill" exams | **Unlisted, not deleted.** Attempts reference them; a `listed` flag hides them from the hub |
| D22-8. Practice mode for a mock test | **Yes, with the exam hub's settings**: sections, duration, no time limit (BR-EXAM-17). Exam mode stays full-length and timed |
| D22-9. Daily generation | **One test's worth per day, rotating** TOEIC → IELTS → VSTEP, into the review queue as one batch per part. Off unless enabled; bounded by the existing AI budget. When a batch set completes a disjoint test, the next numbered test is composed automatically. Auto-publish without review stays out of scope: the owner's brief says never silently publish AI exam questions |
| D22-10. Photos for TOEIC Part 1 | **Openly licensed photographs by link**, the D21-6 pattern: Wikimedia Commons or Openverse, CC0 or CC BY only, stored as `image_url` + `image_attribution` + `image_licence`. A person writes a one-line description of each photo; the model writes the four statements from the description, never from the image |
| D22-11. Charts for IELTS Writing Task 1 | **Rendered by our code** from model-generated data (bar, line, pie, table) to SVG, stored in `fluentra-media`. No licence question, and the data behind the chart is the answer key a grader can read |
| D22-12. Voices in listening | **Two voices** for conversations: the script carries speaker turns and TTS renders each turn with its speaker's voice. Monologues keep one voice |

---

## 3. Order

```text
A  structure     IELTS parts, IELTS blueprint, the listed flag           migration only
B  fixed tests   numbered, disjoint fixed tests; list endpoint           spec → exam
C  sitting       practice mode for a mock test                           spec → exam → web
D  hub           exam cards → test list → sit / practise / random        web
E  media         Part 1 photos, Task 1 charts, two-voice listening       questionbank, media, tts
F  batch review  approve a generated batch                               spec → content → web
G  fixtures      generate, review, export, and load in make seed         cmd/seed, cmd/examgen
H  daily         the daily job and auto-composed next test               questionbank, exam
```

A–D can ship on an empty bank: the hub then shows each exam with "no tests yet". E–G is the content;
H is what keeps it growing. One commit per stage: `feat(exam): wo22 stage X — …`.

## 4. Numbers reserved

| | Range |
|---|---|
| Migrations | `1700000940`–`1700000959` |
| Advisory lock IDs | `1_700_000_940`–`0959` |

---

## Stage A — structure

1. **IELTS, verified.** Before writing a row, read the current format on ielts.org and record source URL
   and date in the migration comment (brief §11). Add a new version `IELTS_ACADEMIC_2026_R2` with the real
   parts — Listening Parts 1–4 (10 questions each, one recording per part), Reading Passages 1–3 (grouped
   by passage), Writing Task 1 and Task 2, Speaking Parts 1–3 — and set the coarse
   `IELTS_ACADEMIC_2026` to `is_current = false`. A new code rather than an edit: parts are what fixed
   tests and attempts point at.
2. **Blueprint** `ielts_default` for the new version, with a CEFR mix the way `toeic_default` has one.
3. **`listed`** on `assess.exams` and on `assess.exam_versions` (default `true`); `false` for the three
   `mock-toeic-*` rows and for the versions D22-2 leaves out.

**Gate.** `GET /exam-versions` returns TOEIC, IELTS R2 and VSTEP, each with a blueprint and its parts.

## Stage B — numbered, disjoint fixed tests

- **Schema.** `assess.mock_tests` gains `number int` (null unless `mode = 'fixed'`) and a partial unique
  index on `(blueprint_id, number) WHERE mode = 'fixed'`.
- **Composer.** `ComposeFixedTest(blueprint, number)`: returns the stored test if it exists; otherwise
  draws each part **excluding every activity used by fixed tests 1 … number−1**, with a seed derived from
  `(blueprint, number)` so a re-run composes the same test. A part the bank cannot fill refuses with that
  part named (BR-EXAM-14) and no row is written.
- **`ComposeNextFixedTests(blueprint)`**: composes numbers `max+1, max+2, …` until the bank cannot fill
  one. Called by the fixture loader (G) and the daily job (H).
- **Spec first.** `GET /exam-versions/{id}/tests` → the version's fixed tests in number order: `id`,
  `number`, `title` ("Test 3" / "Đề 3"), question count, minutes, and the caller's latest attempt on it
  (status, score) so the list can say "done — 710".
- **Coverage.** `distinct_tests_possible` stays the coverage report's number; the list shows fixed tests
  that exist. The two must not be confused in copy (WO 21 Stage E trap).
- **Rules.** Amend BR-EXAM-13: "A fixed test is a numbered, stored composition shared by everyone; the
  fixed tests of one blueprint share no question." Through `tools/docgen/data`, then `make docs`.

**Traps.** (1) The disjoint draw must exclude by **question group**, not by activity: a Part 3
conversation is three activities that travel together. (2) `ON CONFLICT` on the unique index, so two
workers composing Test 6 at once end with one row.

**Gate.** With a bank that holds exactly five tests' worth, `ComposeNextFixedTests` makes Tests 1–5 and
refuses Test 6 naming the short part; no activity appears in two tests.

## Stage C — practice mode for a mock test

- Spec first: `POST /mock-tests/{id}/attempts` accepts the body `POST /exams/{id}/attempts` already does —
  `mode`, `chosen_duration_minutes`, `unlimited`, `sections`.
- `StartMockTestAttempt` reuses `sittingDuration` and the section narrowing of `StartSitting`, so the two
  paths cannot drift. Exam mode ignores the practice fields, as it does today.
- The report of a mock-test attempt carries `elapsed_seconds` like any other.

**Gate.** A practice sitting of Test 2 with only Reading and no time limit starts, counts up, and reports
the time taken.

## Stage D — the hub

`/exams` becomes three levels, replacing the level-labelled list:

1. **Exams** — cards for the listed families: name, format line ("200 câu · 120 phút"), how many tests
   exist, the learner's best score.
2. **Tests of one exam** — Test 1 … Test N with done/score, plus two actions at the top: **"Đề ngẫu
   nhiên"** (D22-5) and **"Tạo đề tùy chọn"** (the WO 21 composer's custom mode, moved here).
3. **One test** — "Thi thử" (exam mode, full length, timed) or "Luyện tập" (the existing settings sheet:
   sections, duration, no time limit), then `ExamSittingRunner` as today.

The WO 21 "Đề thi thử" tab is absorbed: one place to choose an exam. Empty states say what is missing
("IELTS chưa có đề — đang được biên soạn") rather than showing an error. Vietnamese and English, 320 px
and 390 px.

**Gate.** From `/exams`, a learner reaches TOEIC → Test 3 → Luyện tập (Reading only, no limit) in four
taps, and TOEIC → Đề ngẫu nhiên in two.

## Stage E — media the parts need

- **Part 1 photos (D22-10).** A curated list `db/fixtures/exams/toeic-part1-photos.json`: URL, credit
  page, licence, and a human-written description. Only CC0 and CC BY. The generator's
  `photo_description` prompt receives the description, never the image. The card shows the credit line
  whenever the photo is shown.
- **Task 1 charts (D22-11).** The generator returns `{chart_type, title, series}`; a renderer in `media`
  draws the SVG, stores it in `fluentra-media`, and the body carries `image_url`. The series stays in the
  body for the writing grader.
- **Two voices (D22-12).** A listening script carries `turns: [{speaker, text}]`; `cmd/tts` and the render
  job synthesise each turn with its speaker's voice (`speech.tts_voice` plus a second key, added to
  `docs/deployment/configuration.md` **before** the code reads it) and concatenate. Scripts without turns
  render as today.

**Traps.** A photo whose licence is not CC0 or CC BY is refused at load, not at display. A chart whose
data does not add up (a pie over 100 %) fails Gate 1.

**Gate.** A TOEIC Part 1 item shows a credited photo; an IELTS Task 1 item shows a rendered chart; a Part
3 conversation plays in two voices.

## Stage F — batch review

- A generation run tags its drafts with a **batch id** in `_provenance` (exam version, part, run).
- Spec first: `GET /admin/review-queue/batches` (batch, part, count, how many already decided) and
  `POST /admin/review-queue/batches/{id}/approve` with an optional `reject: [ids]` list and a note.
- The WO 21 review screen gains a batch view: items one under another, each with its answer key and a
  reject toggle, and one approve button for the rest.
- Approval runs the same transition per item, in one transaction per batch (BR-CONTENT-11).

**Gate.** A 30-item Part 5 batch is reviewed and approved in one screen; the two rejected items stay out
of the bank.

## Stage G — five tests per exam, frozen

1. **`cmd/examgen`** (new command, its own config section declared — see WO 21's `cmd/foundation`
   fixes): `-exam TOEIC -tests 5` generates each part's questions for five tests plus the margin, as
   batches in the review queue.
2. A person reviews the batches (Stage F).
3. **`cmd/examgen -export`** writes every published bank question of the three exams, with its content
   body, tags, group id and media links, to `db/fixtures/exams/{toeic,ielts,vstep}.json`.
4. **`cmd/seed -exams`** loads the fixtures: content items and published versions with an approval row
   naming the fixture's reviewer and date, bank questions, activities in the bank course, and then
   `ComposeNextFixedTests` for each blueprint. Idempotent on the question fingerprint (BR-QUESTIONBANK-02).
   `make seed` runs it.
5. Listening audio is rendered after the load by `make tts`, as for the seeded listening course.

**Traps.** (1) The fixtures are original generated items; do not paste questions from real TOEIC, IELTS
or VSTEP papers, Study4, or prep books (brief §11). (2) The fixture file must not carry answer keys into
anything the web bundle imports. (3) ~2,000 items through a pooled Supabase connection one row at a time
is slow; load in `pgx.Batch` chunks.

**Gate.** On a database reset with `scripts/reset-dev-database.sql`, `make migrate-up && make seed &&
make tts` gives TOEIC, IELTS and VSTEP five tests each, sittable in both modes, with no network call
besides TTS.

## Stage H — more every day

- **Job** `questionbank.generate_daily`, in the worker's cron (lock in §4). Each day it picks the next
  exam in the rotation (D22-9), generates one test's worth of every part as review batches, and stops.
- **Switch and bounds.** A config key for enabling it and one for the per-run cap, added to
  `docs/deployment/configuration.md` first (CLAUDE.md rule 3). Spend goes through the existing AI budget,
  so an exhausted budget skips the day rather than failing it.
- **Next test.** After a batch approval (Stage F), call `ComposeNextFixedTests` for that blueprint: the day
  a full disjoint test's worth is published, Test N+1 appears in the hub.
- **Admin line.** The question-bank screen shows, per exam, "tests published / questions waiting review /
  parts short for the next test", so the operator sees why Test 6 has not appeared.

**Traps.** (1) Idle generation: if the review queue already holds two unreviewed days for an exam, skip
it — generating faster than anyone reviews only grows a backlog. (2) Duplicates across days: the
fingerprint catches exact ones; the prompt receives recent stems of the same part to steer away from
near-duplicates.

**Gate.** With the job enabled for three days and each day's batches approved, each exam gains one test.

---

## 5. Final gate

WO 19 §2 in full, plus: the hub at 320 and 390 px in both locales; Playwright paths for "TOEIC → Test 1 →
Thi thử → submit → report" and "IELTS → Đề ngẫu nhiên"; `make gen-check` and `make gen-check-web` after
committing; `go-arch-lint` in the Linux container.

## 6. What to cut, in order

1. Stage H's admin line (the job works without it).
2. Stage E's two-voice listening (one voice is intelligible, just less real).
3. Stage H itself — five fixed tests per exam still ship.

Stages A–D and G are the owner's ask and are not on this list.
