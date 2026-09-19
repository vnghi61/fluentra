---
doc_type: handoff
phase: 3
status: planned
last_verified: 2026-09-18
---

# Phase 3 — work order 14

**Purpose.** Speaking practice a learner can actually do. The grading pipeline for
`speaking_task` is built, tested and wired into the worker; 229 speaking items sit in
`content.content_items`. What is missing is every screen in front of it. A learner who
meets one of those items in a lesson is shown "This activity cannot be shown", and the one
row in `skill.speaking_feedback` is the measure of how often the pipeline has run for a
real learner.

This work order gives speaking the same three surfaces writing already has: an exercise
that records, a feedback view that says what went wrong, and a history page.

---

## 1. What exists today

Read this section before changing anything. Two of the documents describing this area are
ahead of the code, and building against them produces a feature the backend cannot serve.

### 1.1 The grading pipeline (built)

`speaking.Grader` implements `learningcontract.ExerciseGrader`, and it grades in two
phases because transcription is too slow to hold a request open.

**Phase one — `Grade`, synchronous, inside the submit request**
([`grader.go`](../../internal/modules/speaking/service/grader.go)):

1. Pull `audio_object_key` out of the attempt response.
2. `domain.ValidateRecordingKey` — the key must belong to this learner.
3. `storage.Stat` — the object must exist. The key is a string the client wrote, and
   nothing else proves the upload happened.
4. Daily cap: `CountAttemptsTowardLimitSince` against `SPEECH_DAILY_RECORDINGS_LIMIT`.
5. Enqueue `speaking.grade_recording`, nudge the worker, return `GradeResult{Async: true}`.

**Phase two — `GradeSubmission`, in the worker**
([`job/grade.go`](../../internal/modules/speaking/job/grade.go)):

1. Download the recording from the private bucket.
2. Transcribe it (see 1.2).
3. Compute the numbers in Go (see 1.3).
4. Ask the model to score the **transcript** (see 1.4).
5. Write `skill.speaking_feedback`, then `CompleteAsyncGrading`.

### 1.2 How the audio becomes text

[`platform/media.HTTPTranscriber`](../../internal/platform/media/transcriber.go) posts the
audio to an OpenAI-compatible `/audio/transcriptions` endpoint (Whisper-shaped). The
response carries the text, the duration, and per-word timings — `{word, start, end}`.

**The word timings are parsed and then discarded.** Only `Text` and `Duration` reach the
grader. They are the raw material for most of section 3's feedback, and they are already
being paid for.

### 1.3 The numbers computed in Go

`speechMetrics` produces exactly two, in
[`domain/speaking.go`](../../internal/modules/speaking/domain/speaking.go):

| Number | How | When |
|---|---|---|
| `read_aloud_accuracy` | word-level Levenshtein of transcript against `reference_text`, as a percentage | read-aloud tasks only |
| `words_per_minute` | word count ÷ duration | whenever duration > 0 |

For a read-aloud task the final score is `0.3 × model score + 0.7 × word accuracy`. The
deterministic number carries the grade; the model mostly writes the prose. For an open
task there is no reference text, so the model's score stands alone.

### 1.4 What the model is asked

`ai.TaskGradeSpeaking` receives `{TaskType, Prompt, Transcript}` and returns an overall
band, a score, criteria with bilingual comments, and feedback text. **It never receives
the audio** (BR-SPEAKING-08), so every judgement it makes is a judgement about wording —
grammar, vocabulary, relevance, coherence.

### 1.5 The honest answer to "how can you grade speech"

Three different things, and only the first two touch the sound:

1. **What was said** — ASR. Reliable, and the basis of everything else.
2. **How fast** — words per minute from the duration. Arithmetic, not judgement.
3. **How well** — the model reading a transcript.

Nothing measures **how it sounded**. Pronunciation, stress and intonation are not
assessed, and the OpenAPI description says so in as many words. A learner who says every
word clearly with the wrong vowels scores the same as one who says them correctly, because
Whisper normalises both to the same string.

> **Documentation drift, deliberately not acted on here.**
> [`speaking/AGENT.md`](../../internal/modules/speaking/AGENT.md) advertises "phoneme-level
> pronunciation assessment", a "heat map", filler-word counts and pause metrics, and lists
> `criteria` accordingly. None of that is implemented. The AGENT.md is a specification that
> the build did not reach, and the API description is the truthful one. Fixing that document
> is its own task; this work order builds on what runs, and section 6 says what it would
> take to make the document true.

### 1.6 The two frontends

| Surface | State |
|---|---|
| Exam | [`SpeakingRecorder.tsx`](../../web/src/features/exam/components/SpeakingRecorder.tsx) records with `MediaRecorder`, picking `audio/webm;codecs=opus` and falling back to `audio/mp4` for Safari, and PUTs to the presigned URL. It works. |
| Lesson runner | Nothing. `speaking_task` is absent from the `canRender*` chain in [`LessonPage.tsx`](../../web/src/routes/LessonPage.tsx), so it falls through to `ActivityUnavailable`. |
| Feedback view | Nothing. |
| History | Nothing. `/speaking/submissions` does not exist; writing's equivalent does. |

### 1.7 The gap against writing, endpoint by endpoint

| Writing | Speaking | |
|---|---|---|
| `GET /writing/attempts/{id}/feedback` | `GET /speaking/attempts/{id}/feedback` | exists both sides |
| `GET /writing/submissions` | — | **missing** |
| — | `POST /speaking/upload-intent` | speaking only |
| — | `DELETE /speaking/attempts/{id}/recording` | speaking only |
| `ExerciseWriting.tsx` | — | **missing** |
| `WritingFeedbackView.tsx` | — | **missing** |
| `MyWritingPage.tsx` | — | **missing** |

---

## 2. Scope

**In.** The speaking exercise in the lesson runner, a feedback view that shows where the
learner went wrong, a submissions list endpoint, a history page, and the i18n and tests
that go with them.

**Out.** Pronunciation scoring, the exam recorder (it works), any change to the grading
pipeline's shape, and repairing `speaking/AGENT.md`.

---

## 3. Steps

### Step 1 — `ExerciseSpeaking` in the lesson runner

**Files.** New `web/src/features/learning/components/Runner/ExerciseSpeaking.tsx`;
`LessonPage.tsx`; `web/src/features/speaking/` (new slice: `api/speakingApi.ts`).

The recorder logic is solved in `SpeakingRecorder.tsx` — mime-type selection, the secure
context check, the `getUserMedia` failure paths. Lift the reusable part into
`web/src/features/speaking/` and have both the exam and the runner import it, rather than
writing a second recorder that handles Safari differently from the first.

The submit sequence:

1. `POST /speaking/upload-intent` with the recorder's real mime type → `{upload_url,
   object_key, daily_recordings_used, daily_recordings_limit}`.
2. `PUT` the blob to `upload_url`. Audio never passes through the API (BR-SPEAKING-01).
3. Submit the attempt with `{"audio_object_key": "<object_key>"}`.
4. The response is `status: "grading"`. `LessonPage` already has that branch — it calls
   `setPollingAttemptId` — and it is kind-agnostic, so it needs no change.
5. Poll for feedback the way `ExerciseWriting` does.

Show `daily_recordings_used / daily_recordings_limit` **before** recording. The cap is
enforced in `Grade`, after the upload, so a learner at the limit otherwise records, waits
for the upload, and is then refused.

**Consent.** BR-SPEAKING-03 requires explicit consent before the first recording, with a
timestamp. Check whether a consent surface exists; if it does not, this step blocks on one
and that is a finding to report, not a rule to skip.

### Step 2 — `SpeakingFeedbackView`

**Files.** New `web/src/features/speaking/components/SpeakingFeedbackView.tsx`.

`WritingFeedbackView.tsx` is the model. Four blocks, ordered by how much the learner can
act on them:

1. **Transcript, as a diff** — for a read-aloud task, align the transcript against
   `reference_text` and mark words missed, added and substituted. This is the "where did I
   go wrong" the work order is named for, and the same word-level Levenshtein that produced
   the score produces the alignment; today it returns only a number and throws the path
   away. For an open task, show the transcript plain.
2. **Criteria bands** with their bilingual comments, as writing renders them.
3. **The numbers** — words per minute against a stated comfortable range, and read-aloud
   accuracy when present. Label them as measurements, not verdicts.
4. **A plain statement that pronunciation was not assessed.** Otherwise a learner reads a
   high accuracy score as "I pronounced it correctly", which is exactly what it does not
   mean. BR-SPEAKING-09 and the module's own note on wording point the same way.

Play the recording back beside the transcript when it still exists, and say so when it does
not — recordings are purged at 90 days while scores outlive them (BR-SPEAKING-04/05).

### Step 3 — `GET /speaking/submissions`

**Files.** `api/openapi/openapi.yaml` **first** — the spec is the contract, and no handler
is written before it. Then `transport/http`, `service`, `repository`,
`db/queries/speaking/`, and the regenerated client.

Mirror `listWritingSubmissions` exactly: `page` and `page_size`, returning
`{items, total, page, page_size}`. Per item: `attempt_id`, `status`, `overall_band`,
`score`, `created_at`, plus two speaking-specific fields — `has_recording` (false once
purged) and `task_type`.

No new table. `skill.speaking_feedback` has `attempt_id`, `user_id`, `criteria` and
`recording_deleted_at` already.

### Step 4 — `MySpeakingPage`

**Files.** New `web/src/routes/MySpeakingPage.tsx`; `router.tsx`; `AppShell.tsx`.

`MyWritingPage.tsx` is the model, at `/my-speaking`, reachable from the same place
`/my-writing` is. A row opens the feedback view from step 2. Rows whose recording has been
purged say so rather than offering a dead player.

### Step 5 — i18n and tests

Every string through `t()` in both `en.json` and `vi.json`; `src/test/i18n-keys.test.ts`
enforces it for literal keys.

Tests, in the order they are worth writing:

| Test | Why |
|---|---|
| Domain: the read-aloud alignment step 2 renders | New logic, pure, cheap to test, and it is what the learner reads |
| Integration: `GET /speaking/submissions` | New endpoint against real Postgres |
| Component: `ExerciseSpeaking` upload sequence, with `MediaRecorder` mocked | Three network calls in order, and the failure path at each |
| Component: `SpeakingFeedbackView` renders a purged recording without a player | The state that only appears after 90 days, so nobody meets it by accident |
| E2E: record → grade → feedback → history | Needs a fixture recording; `test/fixtures/corpus/speech/` is where the module's AGENT.md says it goes |

The E2E suite runs against `AI_PROVIDER_*` unset, which resolves to the mock provider, and
`provider_mock.go` already answers `speaking_task`. The transcriber needs the same
treatment — check whether `MockTranscriber` is wired when no speech endpoint is configured,
and wire it if not, or the journey fails on a network call CI cannot make.

---

## 4. Order of work

Steps 3 and 4 are independent of 1 and 2 and can be built in parallel. Step 2 depends on
step 1 only for a real attempt to read.

A useful first commit is step 3 alone: it is small, it is server-side, and it makes the one
existing `speaking_feedback` row visible, which is the fastest way to confirm the pipeline
produces what these screens will render.

---

## 5. Risks

| Risk | Why it matters | Answer |
|---|---|---|
| The learner reads "92% accuracy" as "my pronunciation is good" | It measures word identity after ASR normalisation, nothing about sound | Step 2's fourth block, in plain words, not a tooltip |
| ASR mishears an accent and the score falls | Whisper's error rate is not uniform across accents | BR-SPEAKING-06 already says a low-confidence transcript must not be scored — confirm the threshold is actually enforced before shipping the screens, because the screens make the score visible for the first time |
| Recording upload fails after a long take | The learner loses the take and the daily quota feels spent | Keep the blob in memory and offer retry before discarding; do not re-request an upload intent on retry |
| `MediaRecorder` mime support varies | Safari has no webm | Already handled in the exam recorder — which is the argument for lifting it rather than rewriting it |

---

## 6. If pronunciation scoring is wanted later

Not in this work order, recorded so the decision is not re-derived from scratch:

- Whisper's per-word timings are already parsed and dropped. Keeping them buys pause
  detection, per-word timing and a filler-word count with **no new provider** — most of
  what `AGENT.md` calls fluency metrics.
- Phoneme-level scoring needs a provider that returns it (Azure Pronunciation Assessment
  and Speechmatics do; a plain Whisper endpoint does not). That is a new adapter in
  `platform/media`, a new config key, and a cost per minute.
- The schema is ready either way: `skill.speaking_feedback.criteria` is `jsonb`.

---

*Previous work order: [phase-3-work-order-13.md](phase-3-work-order-13.md) ·
Module: [`speaking/AGENT.md`](../../internal/modules/speaking/AGENT.md)*
