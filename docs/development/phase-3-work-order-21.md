---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-22
---

# Phase 3 — work order 21: a UI for what work order 19 built

**Purpose.** [Work order 19](phase-3-work-order-19.md) built phase 4's backend — learner resources,
renditions, extraction, the review queue, Foundation content, the question bank, exam versions, mock tests,
node mastery and the learning path — but almost none of it has a screen. This work order gives each piece
one, so the owner can test it in a browser, and wires the one flow WO 19 left unconnected: generating
practice from a learner's own upload.

**Read first.** WO 19 §§1–3 and the `AGENT.md` of `resource`, `content`, `questionbank`, `exam` and
`learning`; `web/AGENT.md` (touch targets, bottom nav, i18n); `web/src/features/admin/model/sections.ts`.

---

## 1. What a browser can reach today

Verified 2026-09-22 against `web/src` and the dev database.

| WO 19 stage | API | Screen | Data in dev |
|---|---|---|---|
| A–B resources, renditions, extraction | `/me/resources*` | **none** — no route, no API client | 0 resources |
| B → C generate from a resource | `Generator` supports `purpose: "resource"` | **none**, and **no endpoint or job calls it** | — |
| E review queue | `GET /admin/review-queue`, `/admin/content/{id}/review\|publish` | **none** | 0 drafts |
| D Foundation topics | `/foundation/topics*`, `/admin/foundation/topics*` | learner topic page and path view exist; no admin screen | **0 published topics**, so the path is empty |
| F question bank | `/admin/questions`, `/admin/questions/generate`, `…/{id}/stats` | **none** | 0 questions |
| G exam versions | `GET /exam-versions` | none; only the renamed "Mixed-skill practice exam" is visible | 6 versions, 2 blueprints |
| H mock tests, coverage | `POST /mock-tests`, `/mock-tests/{id}/attempts`, `/admin/exams/versions/{id}/coverage` | **none** | 0 mock tests |
| I node mastery, daily weak slot | `/practice/daily` | daily set exists; the weak slot is invisible without mastery rows | 0 mastery rows |
| J learning path | `/me/foundation/path`, `/me/foundation/next` | `FoundationPathView` on the topic page | empty (no topics) |

So the order below is **data first**: a screen over an empty table proves nothing.

**Flashcard pronunciation (added 2026-09-22).** Not a WO 19 gap, but found while testing it and small
enough to ride along as Stage G. Every speaker button falls back to browser speech synthesis because no
card carries a recording, and on some Android phones synthesis never starts at all: a Xiaomi (HyperOS,
imported ROM) with media volume up and an English Google TTS voice installed stays silent, because the OS
stops Chrome reaching "Speech Services by Google". Branch `fix/android-speech-silent` makes the button
detect that and say so; only a recording makes it speak.

| Fact | Where |
|---|---|
| `PronounceButton` already prefers `audio_url` and falls back to synthesis on a missing or broken file | `web/src/components/ui/pronounce-button.tsx` |
| `FreeDictionaryAPI.Lookup` already returns a human recording URL with its attribution page and licence (mostly CC BY / BY-SA, which require credit) | `vocabulary/repository/dictionary.go:20-44` |
| A learner **upload** stores `audio_url` in the sense body, but drops `AudioAttribution` and `AudioLicence` | `vocabulary/service/upload.go` (sense body builder) |
| The **210 seeded words** and the lessons' `vocab_flashcard` configs have no `audio_url`: 0 of 218 flashcard versions in dev | `cmd/seed/content_data.go`; dev DB |
| A published version cannot be edited (BR-CONTENT-01): adding audio means a new version | `content` AGENT.md |
| "eat" resolves to `eat-us.mp3` (BY-SA 3.0) and `eat-uk.mp3` (BY 3.0 US) | checked 2026-09-22 |

---

## 2. Decisions

| Question | Default |
|---|---|
| D21-1. Where learner resources live | A new page **`/my-resources`** ("Tài liệu của tôi"), linked from Practice, same pattern as `/my-writing` |
| D21-2. How much a resource may generate | **One set of 10 practice items per resource**, regenerable once a day; kinds limited to the vocabulary and grammar choice kinds the practice pool already grades. Budgeted under the existing `generation` AI task |
| D21-3. Who sees generated items | Only the owner (BR-RESOURCE-12). They never enter the shared bank or the review queue |
| D21-4. Admin screens | New sections in `ADMIN_SECTIONS`, each gated by the permission its API already declares (`content.review`, `questionbank.read`, …) |
| D21-5. Mock tests for learners | A second tab on `/exams`: pick a version and blueprint, compose, sit it with the existing runner |
| D21-6. Where pronunciation audio comes from | **The dictionary's recording, stored as a link** — `DictionaryEntry` already argues against copying the files. Prefer the US recording, then UK, then any. Synthesis stays the fallback for words with none |
| D21-7. Crediting a recording | The body carries `audio_attribution` and `audio_licence` beside `audio_url`; whenever the recording (not synthesis) plays, the card shows one line: "Âm thanh: Wikimedia Commons · CC BY-SA" linking the attribution page |

---

## 3. Order

```text
0  data            seed and generate enough to look at
A  my resources    upload, list, detail, delete               API exists
B  generate        endpoint + button: practice from my upload new endpoint
C  review queue    admin: approve / reject drafts             API exists
D  bank + coverage admin: questions, generate, coverage       API exists
E  mock tests      learner: compose and sit                   API exists
F  path + weak     learner: path entry point, weak slot label API exists
G  pronunciation   recorded audio on every flashcard          backfill + attribution
```

A–B are the learner-material branch; C–D unblock E–F, because nothing publishes without review. G stands
alone and may go first: it is the one learners already hit. One commit per stage:
`feat(web): wo21 stage X — …`.

## 4. Numbers reserved

Migrations `1700000920`–`0939` (Stage B only, if a table is needed for D21-2's daily limit).

---

## Stage 0 — data

1. `make seed`, then `go run ./cmd/foundation` to draft Foundation topics, and
   `POST /admin/questions/generate` for one VSTEP blueprint's parts (WO 19 §§D, F).
2. Upload one PDF, one MP3 and one MP4 through the API, then `go run ./cmd/media -all` and run the worker,
   so resources reach `validated` with renditions and extraction.

**Gate.** The review queue lists drafts; each test resource has renditions and an extraction.

## Stage A — my resources (learner)

- `web/src/features/resource/api` — the upload helper WO 20 also needs: intent → PUT → confirm, then poll
  `GET /me/resources/{id}` until a terminal status.
- `/my-resources`: drag-and-drop or file picker (accept list = `resource/domain/mime.go`, 50 MB), quota
  bar (50 files / 250 MB), list with thumbnail, kind, status chip, size.
- Detail: PDF preview image + "open" link, `<audio>`/`<video playsInline>` from renditions, extracted text
  (collapsible), transcript for audio/video, CEFR estimate and spine topics from `classification`, delete
  with confirm.
- Status copy for every state, including `rejected` with its reason and "processing" while renditions are
  `pending` — the pipeline runs in Actions, so minutes are normal.

**Gate.** Upload → validated → detail shows preview, text and CEFR on a 390 px phone; delete removes it.

## Stage B — generate practice from a resource

- Spec first: `POST /me/resources/{id}/practice` → 202; `GET /me/resources/{id}/practice` → the items.
- Service (`learning`, called through its contract): requires an owned, `extracted` resource; calls
  `Generator` with `purpose: "resource"`, `OwnerID`, `SourceText` (truncated to the model's budget);
  enforces D21-2.
- UI: "Tạo bài tập từ tài liệu này" on the detail page; the set opens in the existing lesson runner.

**Traps.** The extracted text is untrusted input to the model (WO 19 B.5 trap 2). Another user's
resource id must be a 404, not a generation.

**Gate.** A learner generates and completes a set from their PDF; a second learner cannot see or start it.

## Stage C — review queue (admin)

- Section "Duyệt nội dung": `GET /admin/review-queue` list, filter by kind and purpose; detail renders the
  item with the learner runner's components (answer key visible to the reviewer); approve / reject with a
  note via `/admin/content/{id}/review` and `/publish`.

**Gate.** Approving a Foundation draft makes it appear on `/foundation/topics/{code}`.

## Stage D — question bank and coverage (admin)

- Section "Ngân hàng câu hỏi": search/filter `/admin/questions`, per-item stats, "generate" form for
  `/admin/questions/generate` (drafts land in Stage C's queue).
- Coverage: per exam version, `/admin/exams/versions/{id}/coverage` as a table of parts with the
  bottleneck highlighted and "distinct tests possible".

**Gate.** Generate → review → approve raises the coverage number.

## Stage E — mock tests (learner)

- `/exams` tab "Đề thi thử": versions from `/exam-versions`, blueprint and mode (fixed / random / weak
  topic / custom parts), `POST /mock-tests`, then `/mock-tests/{id}/attempts` into `ExamSittingRunner`.
- A composition the bank cannot fill shows the part that is short (WO 19 H.5 trap 1), not an error.
- Retake replays the same composition (BR-EXAM-12); "new test" composes again.

**Gate.** Compose, sit and see the report; the displayed test count equals the coverage number.

## Stage F — learning path and weak slot (learner)

- Dashboard entry to the Foundation path via `/me/foundation/next`; the path view shows mastered, next and
  locked nodes.
- The daily set labels items drawn for a weak node ("Ôn lại: Thì hiện tại hoàn thành").

**Gate.** After five correct attempts a node shows as mastered and "next" moves on.

## Stage G — recorded pronunciation on flashcards

- **Carry the credit.** `upload.go`'s sense body adds `audio_attribution` and `audio_licence` from the
  `DictionaryEntry` it already has (D21-7). Pick the recording by D21-6 inside `FreeDictionaryAPI`, not in
  each caller, so upload and backfill cannot disagree.
- **Backfill the seed.** A `-audio` mode on `cmd/seed` (no new binary): for every word sense and every
  lesson `vocab_flashcard` whose current version has no `audio_url`, look the lemma up and publish a **new
  version** with the three fields (BR-CONTENT-01 forbids editing the old one). Idempotent: a version that
  already has `audio_url` is skipped. Rate-limited to the dictionary's courtesy pace, one lookup per lemma
  even when several senses share it. Also run it from `make seed` so a fresh database gets audio.
- **Show the credit.** `FlashcardFront`, `FlashcardBack` and `ExerciseFlashcard` read the two new fields;
  `PronounceButton` reports which source actually played, and the card renders the attribution line only
  when it was the recording.
- **Contract.** The flashcard body is untyped `content` JSON; document the three fields where the
  vocabulary card shape is described (`vocabulary/API.md`, the sense schema in the spec if it lists
  fields) before the handler change, per CLAUDE.md rule 2.

**Traps.**

1. **Hotlinked files can move.** A 404 must fall through to synthesis, which `PronounceButton` already
   does — keep a test that proves it, because it is the only thing standing between a dead link and a
   silent button.
2. **Attribution is not optional.** A recording played without its credit line breaches CC BY-SA; a
   card whose body has `audio_url` but no `audio_attribution` must not play the recording.
3. **Do not fetch audio in the browser** from the dictionary at tap time: it puts a third party on every
   flashcard view and turns a slow upstream into a slow button.

**Gate.** On the Xiaomi that stays silent with synthesis, tapping "eat" in SRS review and in a lesson
flashcard plays the recording and shows the credit; a word the dictionary has no recording for still
reaches synthesis and, when that is silent, the hint from `fix/android-speech-silent`.

---

## 5. Final gate

WO 19 §2 in full, plus: every new screen at 320 px and 390 px in both locales (the narrow-320 suite),
Playwright paths for Stages A, C and E, and `make gen-check-web` after committing.

## 6. What to cut, in order

1. Stage F's weak-slot label.
2. Stage D's generate form (generate from the CLI).
3. Stage B — ship A alone; the page is still useful as a private library.

Stage G is not on this list: it is the one stage a learner already feels, and it is small.
