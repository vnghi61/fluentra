---
doc_type: handoff
phase: 3
status: planned
last_verified: 2026-09-11
---

# Phase 3 — work order 12

**Purpose.** A long mock exam in all four skills — listening, reading, writing and speaking — sat in
60 to 90 minutes, in two modes: an **exam mode** with fixed timing and no going back, and a
**practice mode** where the learner chooses the duration. Both submit themselves when time runs out,
including when the learner has closed the tab. Each sitting is **drawn for that learner from an exam
pool**, not generated per exam. Two of the four skills do not exist yet, so this order builds them
first — WP21 — on free services rather than a paid speech API.

**Starts after** [phase-3-work-order-11.md](phase-3-work-order-11.md) is merged. It depends on what
that order builds: activities that are never deleted under an attempt (§3.0), asynchronous grading
and `CompleteAsyncGrading` (§3.3), question sets and `item_results` (§3.6), reporting a bad item
(§3.10), and the pool, its machine checks, its blind solve and `learn.item_exposures` (§3.11).

**Read first.** [phase-3-plan.md](phase-3-plan.md),
[ADR-0015](../adr/ADR-0015-content-exercise-core.md),
[ADR-0025](../adr/ADR-0025-anonymous-curriculum-access.md), and the `AGENT.md`, `API.md` and
`DECISIONS.md` of `exam`, `listening`, `speaking` and `platform/media`.

---

## 1. What exists

Checked against `main` at `bc0ec3e`.

| Piece | State |
|---|---|
| `exam`, `listening`, `speaking`, `platform/media` | Documentation only. No Go code, no tables |
| Speech in the AI layer | None. `platform/ai` has one adapter, `POST {base}/chat/completions`, and `Provider` has only `Name` and `Complete` |
| `SPEECH_PROVIDER`, `SPEECH_API_KEY`, `SPEECH_REGION` | In `.env.example`; **read by nothing** |
| Audio in the web app | `pronounce-button.tsx` plays a dictionary `audio_url` with `new Audio`, else `speechSynthesis` |
| Recording in the web app | None — no `MediaRecorder`, no `getUserMedia` |
| Storage | `MinIOStore` has `PresignPut`, `PresignGet`, `Put`, `Get`, `Delete`; bucket `fluentra-media` exists |
| R2 CORS | `PUT`, `GET`, `HEAD` from the two production origins, header `content-type` (`deploy/r2/cors.json`) |
| Timed sessions | None. Nothing in `learning` or `lesson` has a deadline |
| Scheduled River jobs | None. River is `v0.43.0`; nothing uses `InsertOpts.ScheduledAt` |
| Hosting | Render free plan: "a separate process is not free on that plan" (HANDOFF-PHASE3 §6) |

### The specs contradict ADR-0015 in three places

- `listening/API.md` specs `POST /api/v1/listening/attempts` and `.../attempts/{id}/submit`, and its
  front matter owns `listening_attempts`.
- `speaking/API.md` specs `POST /api/v1/speaking/attempts`, and its front matter owns
  `speaking_attempts`.
- `exam` owns `attempt_answers` — a second place for an answer to a graded item.

ADR-0015: "A skill module that defines its own attempt table fails review." These routes and tables
are revised in §3.1, not built.

### Exam pool items would leak their answers

This order keeps exam items as activities in the exam pool's lessons, for the reason in §2. As
things stand, `POST /activities/{id}/grade` — the preview route, which needs no account — returns
`correct_answer` for any activity, and `GET /lessons/{id}` returns a lesson's items to anyone. A
visitor could read the whole exam pool and collect its answer key. §3.6 closes both.

---

## 2. Decisions taken

### From the owner

| Question | Decision |
|---|---|
| Format | **Four skills, 60–90 minutes**, on Fluentra's own scale — not an official TOEIC score |
| Timing | **Both modes.** Exam: fixed, section by section, no going back. Practice: learner-chosen duration |
| Expiry | **Submits itself** in both modes, including when the client is gone |
| Where | **This order**, after work order 11 |
| Speech | **Free, on the server.** Audio made once per item; recordings transcribed by a free tier |
| Where exam items come from | **A pool of 50 per kind per level**, topped up a few at a time |
| How a sitting gets its items | **Drawn per learner** at the start, from items that learner has not seen |
| Exam pool and practice pool | **Separate** — no exam item ever appears in practice |

### Taken in this plan — the owner may overrule

| Question | Decision |
|---|---|
| Exam items reviewed before publishing? | **No**, the same as the practice pool, with the same checks and reports |
| Exam mode form | Listening 20 min, reading 25 min, writing 20 min, speaking 10 min — **75 minutes** |
| Practice mode duration | Learner chooses **10 to 180 minutes**; free navigation between sections |
| Listening plays | Exam mode **one**; practice mode **three** |
| Sittings a day | `EXAM_DAILY_SITTINGS_LIMIT`, default **5** per learner |
| Score | Each section **0–100**, overall the mean, plus an **estimated CEFR band** |
| Pronunciation | **Not scored.** Shown as "not assessed", never as zero |
| Recordings a day | `SPEECH_DAILY_RECORDINGS_LIMIT`, default **30** per learner |

**Why exam items are lesson activities.** ADR-0015 exists so that "attempt lifecycle correctness is
implemented and tested once" and "mixed-skill lessons are natural". If exam items are activities,
every answer is an ordinary `learn.attempts` row, graded by the grader already registered for its
kind — including the asynchronous path work order 11 builds for writing. `exam` owns only what is
genuinely an exam's: the sitting, the clock, section order, expiry, the report and integrity signals.

**Why a pool and not a form per exam.** An exam form holds around fifteen items across six kinds and
three levels. Generating a fresh form for every sitting would multiply the model's cost by the number
of sittings; generating a shared form per week would give a returning learner items they have
already seen, answers included. A pool of fifty per slot, drawn per learner, costs a fixed amount up
front and grows only when learners run low.

**Why a limit on sittings.** Starting a sitting spends items: they count as seen the moment they are
served. Without a limit, a script starting sittings in a loop would mark the whole pool as seen and
push the top-up to generate — and pay for — items up to the ceiling. Five a day is more than any
learner sits for real.

**What drawing per learner costs.** Two learners' sittings hold different items, so their scores are
close but not strictly comparable. The report says so, and exam scores have no leaderboard.

**Why audio is made once.** A listening item whose audio the browser synthesises must send its
script to the browser, and the script is the answer key: devtools shows it.
`listening/DECISIONS.md` already says "Seeing the transcript first turns a listening exercise into a
reading exercise". Audio rendered once and stored in `fluentra-media` sends the learner a sound file
and nothing else, costs nothing per listen, and sounds the same on every device.

**Why a free transcription tier and not the browser's speech recognition.** The browser's recognition
exists only in some browsers, sends audio to whichever company made the browser, and hands the server
a transcript the client produced — which the client can also write. `exam/DECISIONS.md`: "Client
timing is trivially manipulable"; so is a client transcript.

**What "not assessed" costs.** `speaking/DECISIONS.md` decided to buy phoneme-level assessment, and
the owner has chosen not to pay yet. A speaking score built from a transcript measures what was said —
relevance, grammar, vocabulary, length — and not how it sounded. Transcription also tends to turn a
mispronounced word into the real word it resembles, so a read-aloud accuracy figure built on it runs
high. The report says both, in words. The adapter sits behind an interface so a paid assessor can be
added later without rewriting speaking.

---

## 3. The work

In this order. Each step is at least one commit and releasable on its own.

### 0. Spike: find out what free actually runs

No production code. Write the findings into §4 of this document before §3.3 starts, because three of
them decide how §3.3 is built.

1. **Text to speech on this hosting.** Pick an open-source engine that runs on CPU with an English
   voice. Find out how the Render worker is built — the repository has no production Dockerfile and
   no `render.yaml`, so it is set in the Render dashboard — whether the engine can be installed in
   that build, and whether rendering a two-minute clip fits the worker's memory and time. **If it does
   not fit, render audio offline**: a `cmd/` command the owner or a CI job runs, uploading to
   `fluentra-media`. A separate Render service is not an option on the free plan.
2. **Transcription on a free tier.** Choose an OpenAI-compatible provider that offers
   `POST /audio/transcriptions` on a free tier. Record: accepted formats — send a WebM/Opus recording
   from Chrome and an MP4 from Safari, do not assume; the size and duration limits; the rate limits;
   latency for a 45-second clip; and **what the terms say about using submitted audio**, which the
   owner has to read before learners' voices are sent there.
3. **Scheduled jobs.** Confirm `InsertOpts.ScheduledAt` in River `v0.43.0`, and what happens to a job
   scheduled while the worker is asleep.

### 1. Make the records match the design

- `listening`: retire `audio_items`, `transcripts`, `listening_attempts`; the table this order adds is
  `listening_plays`. Replace the attempt routes in `API.md` with §3.4's.
- `speaking`: retire `speaking_tasks`, `speaking_attempts`, `pronunciation_scores`; the table is
  `speaking_feedback`. Replace the attempt routes with §3.5's.
- `exam`: retire `attempt_answers`. Keep `exams`, `exam_sections`, `exam_attempts`, `score_reports`,
  `integrity_events`, and define `exam_attempts` in the doc as **a sitting**, not a graded answer.
- `platform/media`: the table is `tts_cache`; retire `media_derivatives` and `transcripts` until they
  exist.
- Front matter, `API.md` summaries and `DECISIONS.md` tables are generated. Change them at the docgen
  source and regenerate.

**Architecture edges.** `m_exam` may today depend on `p_job`, `p_ai`, `p_telemetry`,
`c_questionbank`, `c_writing`, `c_speaking` and `c_learning`. It needs `c_lesson` (a section's
activities) and `c_listening` (the play policy). Add them to `.go-arch-lint.yml` **and** to the
dependency graph in `MODULE_INDEX.md` §3 — `EXM --> LSN & LIS` — because `check-drift.mjs` compares
the two and fails when they differ. Run `make arch` in the Linux container, not on Windows.

### 2. Grade a question set in one place

Reading grades a passage's questions; listening grades a script's questions the same way;
`m_listening` may not depend on `c_reading`. Do not copy the code. Move question-set grading into
`content/contract`, beside `RedactForLearner`: the redaction and the grading describe the same body
shape, and a change to the shape should change both in one file. Reading calls it with no change in
behaviour, and its tests prove that before listening uses it.

### 3. `platform/media`: speech in, speech out

**Two adapters, each behind an interface with a mock.**

- `Synthesiser` — text and voice in, an audio file out. The engine chosen in §3.0, or the offline
  command if it did not fit.
- `Transcriber` — a recording in, text and word timings out. Calls the provider's
  `/audio/transcriptions`.

**`content.tts_cache`**: text hash, voice, engine and engine version, object key. A rendered clip is
never rendered twice.

**Config.** A `SPEECH` section, declared in `Options.EnvSections` in
`internal/shared/config/config.go` — sections are allowed explicitly, and an undeclared one is ignored
silently. Keys: `SPEECH_TTS_ENGINE`, `SPEECH_TTS_VOICE`, `SPEECH_ASR_BASE_URL`, `SPEECH_ASR_MODEL`,
`SPEECH_ASR_API_KEY`, `SPEECH_ASR_TIMEOUT`, `SPEECH_DAILY_RECORDINGS_LIMIT`. Remove the three unread
`SPEECH_*` keys from `.env.example` in the same commit, and say why in it.

**No quota here yet.** Transcription does not go through `ai.Router`, so `ai.ai_budgets` does not
meter it. The per-learner daily limit is the only guard; test it.

### 4. Listening

**The kind.** `listening_comprehension`: a script, an audio object key, a voice, and a question set in
the shape §3.2 grades. The **script is answer-bearing**: `RedactForLearner` removes it, and a test
asserts that no learner-facing body carries a script, an answer or a transcript.

**Plays are counted by the server.** `skill.listening_plays`: user, content version, context — a
learning attempt or an exam sitting — and time.

| Route | Does |
|---|---|
| `POST /listening/items/{versionId}/plays` | Checks the policy, records the play, returns a presigned `GET` valid for the clip's length plus a minute |
| `GET /listening/items/{versionId}/transcript?attempt_id=` | The script, only for a graded attempt of the caller's |

Spec both before the handlers. The URL expiring is what makes the limit real; a URL that lives for a
day is a download.

**The runner** plays through the play route, shows plays left, and never receives the script before
the attempt is graded. Reuse the `new Audio` path in `pronounce-button.tsx`.

### 5. Speaking

**The kind.** `speaking_task`, of two types: `read_aloud` with a reference text, and `respond` with a
prompt and a speaking time limit.

**Recording.** `POST /speaking/upload-intent` (as specced) returns a presigned `PUT` to
`fluentra-media` under a key that contains the user id. The browser records with `MediaRecorder`,
uploads directly, then submits an ordinary learning attempt whose response is the object key. The
server checks that the key belongs to the caller and that the object exists before accepting it.

**Grading is asynchronous**, through work order 11's path: `speaking.grade_recording` transcribes,
calls `speaking_grade.v1` with the **transcript only** — `speaking/DECISIONS.md`: "Send audio to the
LLM? No" — and completes the attempt through `learning`. For `read_aloud`, word accuracy against the
reference text is computed in Go, not asked of the model.

**`skill.speaking_feedback`**: `attempt_id` unique, `user_id` FK to `core.users ON DELETE CASCADE`,
`recording_key`, `recording_deleted_at`, `transcript`, `criteria` jsonb, `read_aloud_accuracy`,
`words_per_minute`, `feedback_en`, `feedback_vi`, `prompt_version`, `model`, `asr_model`.

**Voice data has three rules the database cannot enforce by itself:**

- **Account erasure deletes the objects.** The cascade removes the row; nothing removes the file in
  R2. Delete the recordings in the erasure path, and test it against MinIO.
- **Ninety days.** `speaking/DECISIONS.md` sets it. `speaking.purge_recordings` deletes older objects
  and sets `recording_deleted_at`; the scores stay. `DELETE /speaking/attempts/{id}/recording` does
  the same on request.
- **A notice before the first recording**, saying the recording is sent to a transcription service,
  kept for ninety days, and deletable. The owner fills in the provider from §3.0.

`platform/media/DECISIONS.md` says raw uploads are deleted "after derivatives are verified". This
order makes no derivatives: the upload is the only copy. Record that revision.

### 6. The exam, on the server

**Tables, schema `assess`** (create the schema in the migration if the bootstrap has not):

| Table | Holds |
|---|---|
| `exams` | A **template**, not a form: slug, level, the exam-mode total |
| `exam_sections` | The template's sections: position, skill, exam-mode duration, and how many items of which kind |
| `exam_attempts` | The sitting: user, exam, mode, chosen duration, `started_at`, `deadline_at`, current section, status, **the activities drawn for each section**, draft answers, `submitted_at`, `submitted_by` (`learner` or `expiry`) |
| `score_reports` | One per sitting: per-section scores, overall, CEFR estimate, status `pending`, `ready` or `partial` |
| `integrity_events` | Sitting, kind — tab hidden, window blurred, paste — and time |

`user_id` on the sitting cascades from `core.users`. Draft answers are bounded: an essay by
characters, a recording by key, never by what the client claims.

**Routes: follow `exam/API.md`.** One addition, `GET /exam-attempts` — the learner's sittings,
paginated with the total count — added to the spec first. `POST /exams/{id}/attempts` takes the mode
and, in practice mode, a duration, clamped to 10–180 minutes on the server.

**Starting a sitting** draws its items (§3.7), writes them to the sitting, and writes
`learn.item_exposures` for every one through a `learning/contract` method — **in one transaction**.
A sitting that fails to start marks nothing as seen. A sitting started and abandoned has still shown
its items, and they count as seen.

**The daily limit.** `EXAM_DAILY_SITTINGS_LIMIT`, default 5, counted per learner per date in
`Asia/Ho_Chi_Minh`. The sixth start is `429 EXAM_DAILY_LIMIT_REACHED`. Declare an `EXAM` section in
`Options.EnvSections`, as §3.3 does for `SPEECH`.

**The clock is the server's.**

- `deadline_at` is fixed at start and never extended. In exam mode each section also has its own end.
- Every write — autosave, finish a section, submit — compares the server's clock with the deadline,
  with five seconds of grace for the network. Past it, the write is refused; what was saved before it
  counts.
- `GET /exam-attempts/{id}` returns `remaining_seconds` computed on the server. The client counts down
  locally and re-syncs on every autosave and whenever the tab becomes visible again.
- Exam mode: finishing a section, or its time running out, moves on and closes it. There is no route
  back.

**Expiry, when nobody is there.** Three mechanisms, one outcome:

1. At start, insert `exam.expire_attempt` with `ScheduledAt` at the deadline.
2. `exam.sweep_expired`, lock `1_700_000_601`, every minute, for sittings past their deadline still
   `in_progress` — for the worker that slept through step 1.
3. **Reading a sitting past its deadline is itself a trigger.** The API marks it expired, enqueues the
   submission, and nudges the worker. A learner returning after the deadline sees their result being
   built, not a clock at zero.

All three move a sitting out of `in_progress` with `WHERE status = 'in_progress'` and act only if they
did.

**Submission** turns each saved answer into a learning attempt and submits it through a new
`learning/contract` method, with an idempotency key derived from the sitting and the activity, so a
retried submission grades nothing twice. An unanswered item scores zero in the report and creates no
attempt.

**The report** is `pending` until every asynchronous grade settles, listening to the same completion
work order 11 emits. A section whose grading failed is **"not scored"**, and the report is `partial`
— never a zero the learner did not earn. A report still pending after an hour becomes `partial`.
`exam/DECISIONS.md`: reports are stored, not recomputed.

**Closing the leak (§1).**

- `GradePreview` refuses any activity in the exam pool, before the grader runs.
- `GET /lessons/{id}` refuses an exam pool lesson outright — not hidden from the catalogue, refused.
- `learning` refuses `POST /attempts/{id}/submit` for an exam pool activity outside a sitting. It asks
  through an interface `learning` declares and `cmd/api` implements with `exam` — `learning` must not
  import `exam`, which already depends on it.
- Exam pool activities count toward no course progress and appear in no "continue learning".

**Integrity signals are recorded, not enforced.** They travel with autosave rather than on a route of
their own, and the report shows them to the learner. `exam/DECISIONS.md`: "Auto-invalidate on
integrity signals? No — record and show."

**Tests:**

- A write one second after the deadline is refused; one second before, it counts.
- A sitting the learner abandons is submitted by the scheduled job; with the job removed, by the
  sweep; with both removed, by the next read.
- Two expiry paths racing produce one submission and one set of attempts.
- A preview grade for an exam pool activity returns no answer and calls no grader; `GET /lessons/{id}`
  for an exam pool lesson returns nothing.
- A writing section whose grading fails leaves the report `partial`, that section "not scored".
- The sixth sitting in a day is refused, and marks nothing as seen.

### 7. The exam pool

**The same machinery as work order 11 §3.11**, pointed at a second pool: append-only lessons under a
course `pool-exam`, the same checks and blind solve, the same `learn.item_exposures`. **Never the same
items**: a practice selection never draws from `pool-exam`, and a sitting never draws from
`pool-practice`. Test both directions.

**Slots.** Eighteen — six kinds at A2, B1 and B2:

| Kind | Per item | Target per slot |
|---|---|---|
| `listening_comprehension` | A clip and four or five questions | 50 |
| `reading_comprehension` | A passage and five questions | 50 |
| `grammar_sentence_transform` | One rewrite or error correction | 50 |
| `writing_prompt` | A 150-word essay prompt with a model answer | 50 |
| `speaking_task` `read_aloud` | A text to read | 50 |
| `speaking_task` `respond` | A prompt and a time limit | 50 |

**The top-up.** `learning.top_up_exam_pool`, lock `1_700_000_213`, hourly, on work order 11's rule: at
most five items per slot per run, until 50; after that only when a learner active in the last 14 days
has fewer than 10 unseen items in the slot; never past 200.

- **Listening items** are checked on the script — the blind solve reads the script, not the audio —
  and **cannot be drawn until their audio exists**. A new AI task, `listening_generate`, writes the
  script and its questions.
- **Speaking items** have no answer key and skip the blind solve; they still pass the structure and
  duplicate checks.

**Drawing a sitting.** Exam mode and practice mode draw the same composition; only the clock differs.

| Section | Exam-mode time | Drawn |
|---|---|---|
| Listening | 20 min | Three clips |
| Reading | 25 min | Two passages |
| Writing | 20 min | One essay prompt and three rewrite items |
| Speaking | 10 min | Two read-aloud texts and two responses, 45 seconds each |

Items are chosen at random among active items the learner has not seen, at the sitting's level. When
a slot has too few, the draw fills from the items that learner saw longest ago, and the repeat is
logged so the top-up has its signal. A sitting is never short.

**Tests:**

- Two sittings by one learner share no item while the slots have unseen items.
- A learner who has seen every item still gets a full sitting.
- A listening item without audio is never drawn.
- A practice daily set never contains an exam pool item, and a sitting never contains a practice
  pool item.

### 8. The exam, in the browser

- **`/exams`** — a start button for each mode at each level, a duration picker for practice, the
  number of sittings left today, and the learner's past sittings.
- **The sitting** — full screen. The clock from the server. Section steps, locked behind in exam mode.
  The listening player with plays left. The recorder with microphone permission, a visible time limit,
  and re-recording allowed in practice mode only. Autosave every 15 seconds and on every section
  change. At zero: "Time's up — submitting", then the result screen. Offline at zero: the server
  submits anyway, and the next visit shows the result.
- **The report** — per-section scores, overall, estimated CEFR band, the words **"Not an official
  TOEIC score"**, "pronunciation not assessed", a note that sittings differ from learner to learner,
  item-by-item review from `item_results`, links to writing and speaking feedback, integrity signals,
  and a report button on each item.
- **Before the first recording**, the voice notice from §3.5.

Every screen lazy, every string through `t()` in both locales, every screen checked at 320 px.

### 9. Close the record

- Set `status` for `listening`, `speaking`, `exam` and `platform/media` at the docgen source, with the
  tables they actually own.
- Revise `speaking/DECISIONS.md` (transcript-based scoring, pronunciation not assessed, and why),
  `platform/media/DECISIONS.md` (no derivatives), `exam/DECISIONS.md` (sittings drawn per learner from
  a pool), and `listening/DECISIONS.md` if the play limits differ from what it records.

---

## 4. Spike findings

_Empty until §3.0 is done. Its three findings go here, dated, before any work on §3.3 starts —
including the ones that say free did not fit._

---

## 5. What must not change

- **ADR-0015.** No attempt table in `listening`, `speaking` or `exam`. Every graded answer is a
  `learn.attempts` row.
- **ADR-0025.** No answer, script or transcript reaches a learner before their answer is graded — in
  a lesson body, a preview grade, a presigned URL's object, or an exam draft.
- **The two pools never share an item.**
- **Pool lessons are append-only**, and nothing deletes an activity a learner has attempted.
- **Server time.** No deadline, section end or play count is decided by the client.
- **Voice.** Audio never reaches the language model; recordings are deleted at ninety days and on
  erasure.
- **Reports are stored**, and a failed grade is "not scored", not zero.
- **Integrity signals are shown, never enforced.**
- **BR-CONTENT-01** and the **200 kB bundle budget**, as before.

---

## 6. Migration numbers and lock ids

Migrations: take `1700000600` and upward, leaving work order 11 the range from `1700000490`.
Lock ids: `1_700_000_601` for the exam sweep, `1_700_000_213` for the exam pool top-up, and the next
free number for `speaking.purge_recordings` — check every `module.go` first.

---

## 7. Not in this order

- **Phoneme-level pronunciation scoring**, behind an interface beside `Transcriber`, when the owner
  decides to pay for it.
- An official TOEIC or IELTS scale, or a conversion table claiming to be one.
- Webcam or screen proctoring.
- Picture-description speaking tasks.
- Adaptive exams, item statistics, and `questionbank`.
- Human review of the exam pool.

---

## 8. For the owner

**Before §3.3:** read §4 when the spike is written. Choose the transcription provider, **read its
terms on audio it receives**, and decide whether that is acceptable for learners' voices.

**After the merge and deploy:**

1. Migrations, against production deliberately, with `DB_DSN` set on the command.
2. The `SPEECH_*` and `EXAM_*` keys on **both** the worker and the API.
3. If §3.0 chose rendering in the worker: confirm the Render build installs the engine. If it chose
   offline rendering: run the command regularly — listening items stay out of every sitting until
   their audio exists, so a pool with no rendered audio has no listening section to draw.
4. Budget rows for `listening_generate`, alongside work order 11's `practice_generate` and
   `practice_solve`.
5. `deploy/r2/cors.json` already allows `PUT` from both production origins. Confirm the presigned
   upload's `Content-Type` is the only header the browser sends, or add the others there.

**The exam pool fills over hours, not at once** — five items per slot per run. Wait until every slot
holds a full sitting's worth before announcing exams.

**Then check it on the live site:** start an exam, close the tab, come back after the deadline — the
result is there. Start a second sitting: none of the first sitting's items. Record a speaking answer
on a phone. Open the report.

**Every week:** the report queue, now covering the exam pool as well.

---

## 9. What keeps being found in review

**Free is a constraint, not a price.** Every free service in this order has a limit — memory, rate,
duration, terms. §3.0 exists so the design meets the limits before the code does.

**An item shown is an item spent.** Exposure is recorded when an item is served, not when it is
answered — otherwise abandoning a sitting would be a free look at the pool.

**A deadline enforced in one place is a suggestion.** The client clock, the scheduled job, the sweep
and the read all exist because each of the others can be absent.

**A file in storage is not deleted by a row's cascade.** Anything that writes to R2 on a learner's
behalf needs a line in the erasure path and a test that looks in the bucket.

**An answer key has more ways out than the lesson body.** The preview route, the lesson route, the
transcript route, a long-lived presigned URL and an exam draft are each one.

Everything in work order 11 §8 still applies.
