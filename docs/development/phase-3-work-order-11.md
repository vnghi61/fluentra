---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-11
---

# Phase 3 — work order 11

**Purpose.** Before anything else, stop the practice generator deleting learners' attempts every
twelve hours. Then four things, in this order of risk:

1. **Finish WP20.** Reading and writing each have a grader and nothing else, and writing — which
   its own spec calls "the most expensive feature in the product per use" — is wired in a way that
   cannot survive a real AI provider. The owner has agreed to pay for one.
2. **Words typed wrong.** `schol: trường học` is refused today. It should add *school* and say so.
3. **Grade by comparison wherever an answer can be known in advance**, so the model is paid once
   per item rather than once per learner.
4. **A daily practice set, drawn per learner from a pool** the model fills a few items at a time and
   checks by machine, with a way for learners to report a bad item.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification and
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps. Then
[ADR-0015](../adr/ADR-0015-content-exercise-core.md),
[ADR-0025](../adr/ADR-0025-anonymous-curriculum-access.md), and the `DECISIONS.md` of `reading`,
`writing` and `vocabulary`. Most steps change `openapi.yaml`, so both codegen gates apply.

**Follows** [phase-3-work-order-10.md](phase-3-work-order-10.md), merged as PR #77 on
2026-09-10.

---

## 1. What is broken

Every item below was checked against `main` at `bc0ec3e`, not inferred from a document.

### The practice generator deletes learners' attempts every twelve hours

Read these three in order:

1. `vocabulary.generate_exercises` runs every twelve hours (`vocabulary/module.go:48`), and
   `buildLesson` in `vocabulary/service/generator.go` ends every lesson it generates with
   `ReplaceActivities` — whether or not anything about the lesson changed.
2. `lesson/repository/repository.go:441` implements `ReplaceActivities` as
   `DeleteActivitiesByLessonID`, then an insert per activity.
3. `learn.attempts` declares `fk_attempts_activity ... REFERENCES learn.activities (id) ON DELETE
   CASCADE` (`1700000210`, line 75).

So each run deletes every attempt every learner has made in the generated practice course, and
recreates the same exercises under new ids. `HANDOFF-PHASE3.md` §6 records XP as "idempotent on
activity id", so a new id may pay for the same exercise again.

This was established by reading the code, not by running it. §3.0 reproduces it before fixing it.
§3.11's pool depends on the fix: a pool that replaced its own lessons would forget what every
learner has seen.

### Writing is graded inside the request, which its own decision forbids

`writing/DECISIONS.md` records:

> Synchronous or asynchronous grading? **Asynchronous with streaming** — 10–30 seconds is far
> outside an acceptable request budget, and a provider outage must not surface as a failed
> submission.

`writing/service/grader.go` calls `ai.CompleteJSON` inside `Grade`, which runs inside
`POST /attempts/{id}/submit`, and `buildResult` hard-codes `Async: false`. On the mock provider this
returns in microseconds, so nothing has ever looked wrong. On a real one, every essay submission
holds an HTTP request open for as long as the model takes.

### A failed model call is recorded as a pass

When the AI call fails, `evaluate` falls through to:

```go
return 75, true, "Your writing has been received and reviewed.", body.Explanation
```

Nothing reviewed it. The attempt is stored as graded at 75, `correct` is true, the progress rollup
counts it, and `buildResult` issues an SRS card with `InitialGrade: "good"`.

### A visitor with no account can spend the AI budget

`POST /activities/{id}/grade` (`learning/transport/http/handler.go:65`) grades for callers with no
account, and dispatches to whichever grader is registered for the activity's kind. The comment on
`GradePreview` says ADR-0025 keeps metered AI graders off this route. The only thing that does so is
`if result.Async` at `learning/service/service.go:612` — which runs **after** `grader.Grade` has
returned, so the model has already been called and paid for.

### Feedback is shown once and thrown away

`learn.attempts` (`1700000210`) stores `score`, `max_score`, `grader` and `duration_ms`, and no
prose. The attempt-detail builder in `learning/service/service.go` says so itself — search for
`feedback stays nil`.

### The asynchronous path is built and never wired — the seventh time

| Built | Where |
|---|---|
| `GradeResult.Async` | `learning/contract/contract.go` |
| 202 with `status: grading` | `learning/service/service.go:333`, and the spec's `202` on submit |
| `grading` as an attempt status | `ck_attempts_status` in `1700000210` |
| An async fake grader | `learning/domain/fake_grader.go` |

| Missing | Consequence |
|---|---|
| Any job that completes a `grading` attempt | An async attempt stays `grading` for ever |
| Any client that reads a 202 | `LessonPage.handleSubmit` renders the 202 body as a verdict of nulls |
| Any sweep for attempts stuck in `grading` | A lost job is a permanently spinning screen |

The previous six: the gamification widgets, the avatar route, `AdminAIUsage`, `WordAutocomplete`, the
review session, and the account menu.

### Anyone's usage is everyone's budget

`DBBudgetChecker.CheckQuota(provider, task)` meters per provider and task, for the whole product, per
day. There is no per-learner limit. And from its own comment: "If no active budget is configured for
(provider, task), quota is considered available". **No budget row means no limit.**

### Reading asks one question per passage

`reading/service/grader.go` reads one `prompt` and one `correct_answer` and scores 0 or 100.
`reading/DECISIONS.md` says questions live in `questionbank`, and `internal/modules/questionbank`
holds `doc.go` and `contract/doc.go` and no code.

### The record contradicts itself

Both `AGENT.md` files say `status: IMPLEMENTED`; commit `89360a2` set it. Both `TODO.md` files say
"`status` stays `PLANNED` on purpose". The front matter lists eight tables and none exist; two —
`reading_attempts` and `writing_submissions` — are attempt tables, which ADR-0015 forbids.

### A misspelling with the right meaning is refused

`vocabulary/service/upload.go` `verifyItem` asks the dictionary first, and `lookupDatamuse` in
`repository/dictionary.go` accepts a result only when `strings.EqualFold(items[i].Word, term)`. So
`schol` is not found. `vocab_verify.v4` then says "a misspelling is not a word" and the item is
**rejected**. The prompt asks the model to name the word the learner probably meant — but only
inside `reason`, a sentence. Nothing reads it, and *school* is never added.

### A real word typed for another is added as the wrong word

`form: từ` finds *form* in the dictionary, the model sets `meaning_matches: false`, and the item is
**verified as *form*** with the note "Your note did not quite match the usual meaning". The learner
meant *from*.

### Upload notes are English sentences written by the server

`VocabUploadItem.reason` (`api/openapi/components/uploads.yaml`) is free text built in Go, and
`UploadList.tsx` renders it as it arrives. A Vietnamese learner reads English. Work order 10's i18n
census could not see it, because it never passes through `t()`.

### One word can be paid for twice

`VerifyUpload` counts `verified[item.UserID]++` per verified item and `publishVerified` awards XP on
that count. `school` and `schol` in one paste would become two verified items for one sense. Already
true today for a word uploaded again in a later upload.

### Example sentences are published without review, against a recorded decision

`vocabulary/DECISIONS.md` records: "Generate example sentences at request time? **No** — generate at
authoring time and review them." `republishSense` (`upload.go:1304`) calls `EnsurePublished`
directly. Work order 10 §2 said that decision "holds and this order does not overturn it". That was
wrong. The owner has decided to keep publishing directly; the record has to say so.

### Nothing the model writes has an answer key, and nothing can be reported

`vocabulary/service/generator.go` builds practice from verified dictionary data and calls no model.
There is no path by which the model authors an item with an answer, and no table, route or screen
for a learner to say an item is wrong.

---

## 2. Decisions taken

### WP20

| Question | Decision |
|---|---|
| Status label | **`PLANNED` in the first commit**; `IMPLEMENTED` only when §3.12 holds |
| Attempt tables in reading or writing | **None.** ADR-0015 holds |
| How writing is graded | **In the worker**, as a River job on the `ai` queue |
| How the learner hears back | **Polling** `GET /attempts/{id}`. No streaming |
| When the model fails | Attempt becomes **`failed`**: no score, no XP, no card, nothing counted |
| Visitors and writing | Refused **before** the grader runs, with a sign-up prompt |
| Where feedback lives | **`skill.writing_feedback`**, one row per graded attempt, owned by `writing` |
| Rubric | **Four IELTS-style criteria**, band 0–9 in half steps, plus the engine's 0–100 |
| Where reading questions live | **In the content body**, as a set. `reading/DECISIONS.md` is revised |
| Per-learner limit | **`AI_WRITING_DAILY_LIMIT`**, default **10** graded submissions a day |
| Drafts | **In the browser**, keyed by user, removed on sign-out |

### Words typed wrong

| Case | Decision |
|---|---|
| `schol: trường học` — not a word, meaning given | **Add *school***, and tell the learner what was changed |
| `form: từ` — a real word, meaning fits a close spelling | **Add *from* instead**, and tell them |
| `schol` — not a word, no meaning | **Not added**; suggest *school* |
| `cat: con chó` — meaning fits no close spelling | Unchanged: add *cat* with the existing mismatch note |
| Where a correction comes from | Close spellings, then **the dictionary must confirm** the replacement |
| What the learner reads | A **code and parameters**, rendered through `t()` |
| XP | **Once per sense newly added** to the learner's words |

### Content the model writes

| Question | Decision |
|---|---|
| Example sentences | **Published directly**, as now. `vocabulary/DECISIONS.md` records the accepted risk |
| Daily practice | **A pool of 50 items per kind per level**, topped up five at a time; each learner's set is drawn from items they have not seen |
| Practice pool and exam pool | **Separate.** Work order 12's exam items never appear in practice |
| When a learner has seen everything | Their **oldest-seen** items come back; the pool grows only when active learners run low, never past **200** per slot |
| Reviewed before learners see it? | **No** — checked by machine and published |
| The main check on an answer key | **A blind solve**: a second model call answers without the key; disagreement discards |
| Reporting | **Any learner can report any item**; reports queue for an admin |
| Writing practice with an answer | **Rewrite and error-correction items**, graded by comparison |
| Essays | Still AI-graded; the model answer is **written once per prompt** and shown after submitting |

**Why a pool and not a set a day.** A fresh set a day for three levels pays the model for 27 items
every day for ever — and exams on top — most of them for sets nobody opens. A pool pays for fifty
items per slot once, and grows only when learners actually run out. It is the shape work order 10
already gave example sentences: fifteen kept per word, five added per sweep, shown three at a time.

**Why the two pools are separate.** Visitors grade practice items on the preview route, which
returns the correct answer after a submission. An item that could also appear in an exam would hand
its answer to anyone who met it in practice first.

**Why polling and not streaming.** There is no SSE or WebSocket code in the repository, and what
would be streamed is a JSON object only useful once complete and validated. TanStack Query already
polls in this codebase — `features/vocabulary/api/uploadApi.ts:67`.

**Why a feedback table does not break ADR-0015.** The ADR forbids a skill module owning an
*attempt*. What `writing` owns is skill-specific result data — per-criterion bands, annotations, the
prompt version — which the ADR explicitly allows.

**Why reading questions stay in content.** `questionbank` has no code, and building it is larger
than this whole order. Record "revisit when `exam` is built" as the trigger.

**Why the dictionary must confirm a correction.** `verifyItem` asks the dictionary first because,
in its own words, "a model asked 'is this a word' will confidently invent an entry for a typo". A
correction the model proposes and nothing checks would be a second way in that skips the first.

**Why a spelling bound.** Without one, "the meaning fits another word" is translation, not
correction: `cat: con chó` would become *dog*. A correction is only offered within a small edit
distance of what was typed.

**Why nothing is added without a meaning.** `schol` alone is *school* or *scholar*. With no meaning
there is nothing to choose between them by.

**What replaces human review of the pool.** The owner has decided not to review it. An answer key
the model writes is the one thing a machine check cannot confirm by looking at the item, so the
blind solve carries the weight: a second call, given the item without its answer, must reach the
same answer. The report queue is what is left, and it only works if someone reads it — §7 says so.

---

## 3. The work

In this order. Each numbered step is at least one commit and is releasable on its own. §3.9 is
independent of everything after §3.0 and may be done early.

### 0. Before any code: two ways data is lost

**The practice generator deletes attempts.** Reproduce §1's first finding in the integration suite
against a real Postgres: a learner attempts a generated activity, the generator runs, count the
learner's attempts. Then fix it, in its own commit, before anything else in this order:

- A generated lesson's activities are **updated in place** by position — same row, same id — while
  the kind still matches, and **appended** when new.
- An activity is **never deleted while an attempt references it.** One whose source has gone is
  retired: left out of the lesson, kept in the table. Add a column for that only if nothing like it
  exists.
- `ReplaceActivities` keeps no caller that can delete an attempted activity. Rename or remove it
  rather than leave a trap with a reassuring comment.

**Test:** an attempt survives two generator runs, the activity keeps its id, and XP for it is
awarded once.

**A grader error on a claimed attempt.** `SubmitAttempt` claims the attempt before it calls
`grader.Grade`, and a grader error returns `fmt.Errorf("grading attempt %s: %w", ...)`. Find out,
with a test against the real repository, what state that leaves the row in. If it is left
`grading`, every error path in this order — the daily limit, a failed enqueue, a missing content
version — strands the attempt. Fix it with a test that fails before the fix.

### 1. Make the record true

- Set `status: PLANNED` for `reading` and `writing`, and remove `reading_attempts` and
  `writing_submissions` from their `tables:` lists.
- The front matter is generated into more than one file. Find where `tools/docgen/generate.mjs`
  reads `status` and `tables`, change it **there**, and run docgen. Never edit a generated copy.

### 2. Visitors cannot reach a metered grader

Refuse in `GradePreview` **before** `grader.Grade` is called. The existing `if result.Async` check
stays, but it is not the guard.

- Graders declare whether they spend money. Read `buildDeclaredKinds()` in `cmd/api/modules.go`
  first. If it carries no per-kind metadata, add an optional interface in `learning/contract`,
  checked with a type assertion, rather than widening `ExerciseGrader`.
- The refusal is `401` with code `ACCOUNT_REQUIRED`. Spec it first.
- The guest runner shows the existing `GuestNotice` **instead of** the text area.

**Test:** a preview submission for `writing_prompt` returns 401 and a counting fake AI client records
**zero** calls. Remove the guard and the count becomes one.

### 3. Grade writing in the worker

**The grader** does synchronously only what needs no model:

| Submission | Result |
|---|---|
| Empty | Graded now, 0, no model call |
| Under `min_words` | Graded now, proportional score, no model call |
| Over the learner's daily limit | `429 WRITING_DAILY_LIMIT_REACHED`; the attempt stays resubmittable |
| Otherwise | Enqueue the job, return `Async: true` |

Today's count of graded writing attempts comes through a method on `learning/contract`, not a query
from `writing` into `learn.attempts`.

**The config key.** `AI_WRITING_DAILY_LIMIT`, default 10. `config.EnvKey` splits on the **first**
underscore, so it lands in the `ai` section, and the environment allowlist works per section. The
`AI_PROVIDER_*` slots already load, which suggests `ai` is allowed — confirm it rather than assume.
Add the key to `.env.example`.

**The job.** `writing.grade_submission`, queue `ai`, `MaxAttempts: 3`. Copy the shape of
`vocabulary/job/verify.go`. Register it beside `vocabularyModule.VerifyUploadWorker()` in
`cmd/worker/main.go`; `cmd/worker/main_test.go:166` asserts the number of registered kinds and will
fail until you update it.

The job loads the response and the content version, calls `writing_grade.v2`, validates the output,
writes `skill.writing_feedback`, and completes the attempt through `learning`.

**Completion lives in `learning`.** Add `CompleteAsyncGrading(attemptID, GradeResult)` and
`FailAsyncGrading(attemptID, reason)` to `learning/contract`. Completion runs the same
`executeRollupTx` as the synchronous path — progress, outbox events, XP, SRS cards. Both update the
row only `WHERE status = 'grading'` and report whether they did.

**Wake the worker.** Hand the `httpWorkerNudger` from `cmd/api/main.go` to `writing` through `Deps`
as a consumer-defined interface, and nudge after the enqueue commits.

**The sweep.** `learning.sweep_stuck_grading`, lock `1_700_000_212`, fails attempts `grading` for
more than an hour. It races a queued job when the worker wakes; conditional completion makes the
race harmless.

**Tests:**

- Submit returns 202; running the job moves the attempt to `graded` and publishes
  `activity.completed`, as the synchronous path does.
- A provider that errors on every call leaves the attempt `failed`: no score, no progress row, no
  review card, nothing counted against the limit.
- A job run against an attempt the sweep already failed changes nothing.
- An enqueue that fails leaves the attempt resubmittable.

### 4. Keep the feedback

**`skill.writing_feedback`**

| Column | Notes |
|---|---|
| `attempt_id` | Unique. No FK: `learn.attempts` is partitioned on `(created_at, id)` |
| `user_id` | FK to `core.users(id) ON DELETE CASCADE` |
| `overall_band`, `score` | Band 0–9 in half steps; score 0–100 |
| `criteria` | jsonb: four criteria, each a band and a comment in English and Vietnamese |
| `annotations` | jsonb: quoted span, located offsets, comment in both languages |
| `feedback_en`, `feedback_vi` | The summary |
| `prompt_version`, `model` | What produced it |

**The cascade on `core.users` is not optional.** Annotations quote the learner's essay. Read
`user/job/export.go` too: if the export enumerates learner data by module, this is learner data.

**`writing_grade.v2`** is a new version, not an edit to v1 — the cache key includes the version.
Criteria: task response, coherence and cohesion, lexical resource, grammatical range and accuracy.

**Annotations are located, not trusted.** The prompt asks for quoted text; the job finds it in the
submission and stores the offsets it found. A quote not in the submission is dropped. Test it.

**Reading it back.** `GET /writing/attempts/{id}/feedback`, 404 for an attempt that is not the
caller's. Spec it first.

**Never cache an essay.** `learn.answer_explanations` is keyed by `(content_version_id,
user_answer)` and read across learners. An essay must never become a `user_answer` there.

### 5. The writing screens

- **Marking state.** On a 202 the runner says the essay is being marked and polls
  `GET /attempts/{id}` — every 2 s for 30 s, then every 5 s, stopping on unmount and after 3 minutes,
  when it says marking continues and where the result will be.
- **Feedback view.** Overall band, the four criteria, the essay with annotations highlighted, the
  summary in the interface language, and the prompt's **model answer** below it.
- **My writing.** The learner's writing attempts — status, band, date — each opening its feedback.
  Paginate with the **total count**.
- **Drafts.** `localStorage`, keyed by user id, cleared once graded and **removed on sign-out**.

New screens are lazy routes. The entry chunk is 180.0 kB of a 200 kB budget, which stands for a
largest contentful paint under 2.5 s on throttled 4G ([phase-1-plan.md](phase-1-plan.md), P5).

### 6. Reading: a passage with questions

**The body.** One `reading_comprehension` body carries a passage and a `questions` array: `id`,
`type` (`multiple_choice`, `true_false_not_given`, `gap_fill`), prompt, options where the type has
them, and the answer.

**BR-CONTENT-01.** The grader still grades the single-question body the seed already wrote. Keep a
test built from that exact shape.

**Redaction reaches into the array.** `GET /lessons/{id}` for a lesson with a question set contains
no answer, for any question, in any shape.

**Per-question results.** Add an optional `item_results` — id, correct, correct answer — to
`GradeResult`, `SubmitAttemptResult` and `PreviewGradeResult`. This widens the engine's contract
deliberately, as ADR-0015's "revisit when" asks. Spec it first.

**Reading speed.** `reading_ms`, from first render to "I have finished reading", before the
questions appear. Words per minute is computed on the server and is not part of the score. Zero,
negative or over an hour is ignored rather than rejected.

### 7. Content, authoring and a way in

- **Seed.** Six passages from A2 to B2 with four to six mixed questions each; six writing prompts —
  an email, an opinion essay, an IELTS Task 2 shape — at 60, 120 and 250 words by level, each with a
  model answer. Vietnamese explanations throughout, in published lessons.
- **Admin.** `AdminContentDetailModal.tsx:58–63` validates only `vocab_*`, `grammar_*`, `*choice*`
  and `*match*`. Add `reading_comprehension` (a passage; every question with an answer) and
  `writing_prompt` (a prompt; `min_words` above zero).
- **Practice page.** Reading and writing appear and lead to those lessons. Open the page and follow
  the links.

### 8. Writing practice with an answer

Rewrite and error-correction items use the existing `grammar_sentence_transform` kind, whose grader
already accepts a list: `Acceptable []string` in `grammar/service/grader.go`. No new kind, no new
grader.

A rewrite has more than one right answer, and a list reduces that problem without removing it. Keep
each item tightly constrained — "rewrite using *although*", "correct the one error" — so the set of
right answers is small enough to list. A learner who is marked wrong for a right answer reports it
(§3.10).

### 9. Words typed wrong

Independent of §3.1–§3.8.

**The flow in `verifyItem`,** for a **single-word** term. Phrases keep today's behaviour: an edit
distance across a phrase matches unrelated phrases.

1. **Dictionary finds the term, meaning fits** — unchanged.
2. **Dictionary finds the term, meaning given and does not fit** — look for a close spelling whose
   meaning does fit. Found and confirmed: add that word, note `meaning_corrected`. Not found:
   today's behaviour, note `meaning_mismatch`.
3. **Dictionary does not find the term, meaning given** — look for a close spelling whose meaning
   fits. Found and confirmed: add it, note `spelling_corrected`. Not found: reject, `not_a_word`.
4. **Dictionary does not find the term, no meaning** — reject. If a close spelling exists, note
   `spelling_suggestion` with the suggestion. Nothing is added.

**Close spelling.** Optimal-string-alignment distance — Damerau-Levenshtein without repeated edits —
of at most 1 for terms under five letters and at most 2 otherwise, compared case-insensitively.
*form* to *from* is one transposition; *schol* to *school* is one insertion. Plain Levenshtein counts
the transposition as two and would miss *from*.

**Where candidates come from.** Two sources, both filtered by the bound above:

- **Datamuse.** `lookupDatamuse` already calls `?sp=<term>` and discards everything that is not an
  exact match. Record the real responses for `schol`, `recieve` and `form` as test fixtures before
  relying on them — do not assume what `sp` returns for a misspelling. If it returns no near
  spellings, use Datamuse's suggestion endpoint instead, and record that response too.
- **The model.** `vocab_verify.v5` adds `intended_term`: the word the learner most likely meant,
  given their meaning, or empty. v4 stays for reproducibility.

**Confirmation.** A candidate becomes the word only if the **dictionary** finds it, and then it goes
through the **same verification** a correctly typed word does — the same `judge`, the same
`materialise`. There is no second, lighter path for corrected words.

**What is stored.** A migration adds to `skill.vocab_upload_items`:

| Column | Notes |
|---|---|
| `corrected_term` | The word actually added, when it differs from `term` |
| `suggested_term` | The suggestion, when nothing was added |
| `note_code` | Constrained to the codes below; null for a plain success |

`term` keeps what the learner typed, as its column comment already requires. `reason` stays for the
admin queue and for rows written before this migration.

**Codes.** `spelling_corrected`, `meaning_corrected`, `spelling_suggestion`, `meaning_mismatch`,
`already_in_your_words`, `not_a_word`, `proper_noun`, `queued_for_enrichment`. `VocabUploadItem`
gains `corrected_term`, `suggested_term` and `note_code`; spec them first. `UploadList.tsx` renders
`t("uploads.note." + note_code, { term, corrected, suggested })` and falls back to `reason` only when
`note_code` is null. In Vietnamese: "Bạn gõ *schol* — đã thêm *school*." and "Có phải bạn định gõ
*school*?"

**XP once per sense.** Count words whose `skill.user_word_state` row this upload **created**, not
items marked verified. `UpsertUserWordState` is `ON CONFLICT ... DO UPDATE`, so the query has to
report whether it inserted. A corrected word the learner already has is `already_in_your_words` and
earns nothing. That also closes the existing re-upload case.

**Tests,** each with the exact input:

| Input | Expect |
|---|---|
| `schol: trường học` | Verified as *school*, `spelling_corrected`, one deck item, one XP |
| `form: từ` | Verified as *from*, `meaning_corrected` |
| `schol` | Rejected, `spelling_suggestion`, suggested *school*, nothing added |
| `cat: con chó` | Verified as *cat*, `meaning_mismatch` — *dog* is not a close spelling |
| `schol: trường học` and `school` in one paste | One deck item, one XP |
| Model proposes a word the dictionary does not know | Not added |
| Model proposes a word outside the spelling bound | Ignored |
| `look aftr: chăm sóc` | Today's behaviour — phrases are not corrected |

### 10. Reporting a bad item

**`content.item_reports`**: `content_version_id`, `user_id` (FK to `core.users ON DELETE CASCADE`),
`reason` (`wrong_answer`, `unclear`, `typo`, `my_answer_was_right`, `other`), an optional note of at
most 500 characters, `created_at`, and `UNIQUE (content_version_id, user_id)` so one learner counts
once.

- `POST /content/versions/{id}/reports`, signed in only, behind the existing rate limiter.
- A report button on every activity after it is answered, and on each example sentence on the
  flashcard back — the example sentences are published unreviewed by decision, so this is their
  review.
- An admin list of reported versions, ordered by number of distinct reporters, each opening the
  item. Withdrawal uses the existing `/admin/content/{id}/archive`. **Before relying on it, test what
  archiving does to a lesson that points at the item** — §3.0 is what a cascade two files away does.
- A withdrawn pool item (§3.11) is left out of every selection from then on. Sets already built keep
  it.
- A report never changes a grade already given.

### 11. The practice pool and the daily set

**Generate a little, keep it, draw from it.** The model adds a few items at a time to a pool; each
learner's daily set is drawn from items that learner has not seen.

**Slots.** A slot is one kind at one level. The practice pool has nine:

| Kind | Levels | Target per slot |
|---|---|---|
| `reading_comprehension` — a passage with four to six questions | A2, B1, B2 | 50 |
| `grammar_tense_choice` | A2, B1, B2 | 50 |
| `grammar_sentence_transform` — rewrite and error correction (§3.8) | A2, B1, B2 | 50 |

**Where it lives.** A course `pool-practice`, a unit per level, a lesson per kind. Each item is a
published content version and an activity in its slot's lesson. **Pool lessons are append-only**:
items are added, withdrawn items are left out of selection, nothing is replaced or deleted — §3.0 is
why. The pool course never appears in the catalogue; check whether `learn.courses` already has a
status or visibility the catalogue filters on before adding one.

**Owner: `learning`.** `.go-arch-lint.yml` allows `m_learning_service` to depend on `c_lesson`,
`c_content` and `p_ai`, and forbids `content` and `lesson` from importing `p_ai`. `m_learning_job`
may depend only on `m_learning_domain`, `p_job` and `p_telemetry`, so the top-up is a **service
method** run by a cron `Task` — the shape `vocabulary.generate_exercises` already uses.

**The top-up.** `learning.top_up_practice_pool`, lock `1_700_000_211`, hourly. Each run adds **at
most five** items to each slot:

- until the slot holds 50 active items;
- after that, only when a learner active in the last 14 days has fewer than 10 unseen items left in
  that slot;
- never past **200** active items in a slot.

Five a run spreads the model's cost and rate limits over hours rather than spending them in one
burst — the same pace work order 10 set for example sentences.

**Two tasks.** `practice_generate` writes one item with its answer; `practice_solve` answers an item
it is given without the answer. Follow the existing names — `vocab_verify`, `writing_grade`.

**The checks, in order.** An item that fails any of them is regenerated at most twice, then skipped
for this run, and the run logs which check failed.

1. **It parses** into the grader's own body type.
2. **Its own answer scores full marks through the real grader** — the code that will grade learners,
   not a copy of its rules.
3. **Structure**: the correct option is one of the options; options are distinct after
   normalisation; a gap-fill answer does not appear in its own prompt; the Vietnamese explanation is
   not empty.
4. **The blind solve.** `practice_solve` receives the redacted body and must reach the same answer,
   compared the way the grader compares. Disagreement drops the item.
5. **Not a duplicate** of an item already in the slot, compared after normalisation.
6. **Redaction**: the body the learner receives contains no answer.

**What each learner has seen.** `learn.item_exposures`: `user_id` (FK to `core.users ON DELETE
CASCADE`), `activity_id`, `first_served_at`, primary key `(user_id, activity_id)`. Written when an
item is put in front of a learner — here, when their daily set is built; in work order 12, when a
sitting starts. **An item shown counts as seen whether or not it was answered.**

**The daily set, built on first open.** `GET /practice/daily` builds the caller's set the first time
they open it on a date in `Asia/Ho_Chi_Minh`, stores it in `learn.daily_sets` (`user_id`,
`local_date`, ordered `activity_ids`, `UNIQUE (user_id, local_date)`), writes the exposures in the
same transaction, and returns that set for the rest of the day.

- **Contents:** one passage, five grammar items, three rewrite items, at the learner's level. Read
  how the profile stores a level before adding one; if it stores none, the learner picks A2, B1 or
  B2 the first time and the choice is kept in preferences.
- **Selection:** at random among active items the learner has not seen. When a slot has too few,
  fill from the items they saw longest ago. A set is never short; a repeat is logged.
- **No cron builds sets in advance.** A set nobody opens costs nothing.

**A way in.** A "Today's practice" card on the dashboard and a tile on the practice page, both lazy.
The runner today loads a lesson; give it an activity list as a second source rather than writing a
second runner.

**Tests:**

- A learner's second daily set shares nothing with the first while the slot has unseen items.
- A learner who has seen every item still gets a full set, drawn from their oldest exposures.
- Two requests racing on a first open produce one set.
- A fake model whose key is wrong and whose blind solve is right adds nothing to the pool.
- A slot at 50 whose active learners all have plenty unseen gains nothing; one where a learner has
  nine unseen gains five; one at 200 gains nothing.

### 12. Close the record

- `writing`: `tables: [writing_feedback]`. `reading`: `tables: []`. Every other name those lists held
  is removed, and each `TODO.md` hand-written section says which and why.
- Revise `writing/DECISIONS.md` (polling), `reading/DECISIONS.md` (questions in content; revisit when
  `exam` is built), and `vocabulary/DECISIONS.md`: example sentences are published without review,
  **by the owner's decision on 2026-09-11**, with the report button as the compensating control. If
  those tables are generated, change them at the docgen source.
- `questionbank/DECISIONS.md` records "Allow AI-generated items to auto-publish? **Never**". The
  practice pool in §3.11 does exactly that, by the owner's decision on 2026-09-11. Record the
  revision there — scoped to generated practice, with the blind solve and the report queue named as
  the controls that replace review — rather than leaving two recorded decisions that contradict each
  other.
- **Then** set `status: IMPLEMENTED` for `reading` and `writing` at the docgen source and regenerate.

---

## 4. What must not change

- **Attempts are never deleted by a content change.** Generated and pool lessons update in place or
  append; nothing replaces an activity a learner has attempted.
- **ADR-0015.** No attempt table in `reading` or `writing`; no completion path outside `learning`.
- **ADR-0025.** No answer in any learner-facing body, nested or not. `GradePreview` writes nothing.
- **BR-CONTENT-01.** A published version is never updated; old body shapes keep grading.
- **The dictionary is authoritative on existence**, for corrected words as much as typed ones.
- **Quota on success.** A failed attempt is never counted or charged.
- **Submissions are fixed.** No editing a submitted essay.
- **i18n.** Every string a learner reads goes through `t()` — including notes that start on the
  server. `setLocale`, never `changeLanguage` directly.
- **The bundle budget.** New screens are lazy. Do not raise the budget.

---

## 5. Migration numbers

Take `1700000490` and upward. The highest on `main` is `1700000480`.

---

## 6. Not in this order

- **WP21** — `platform/media`, listening, speaking. Next, and it needs a speech provider chosen.
- **Timed mock exams** — four skills, 60–90 minutes, an exam mode and a practice mode, both
  submitting on expiry, drawn from a separate exam pool. Work order 12, after WP21.
- Human review of the pool or of example sentences — declined by the owner.
- Correcting phrases; letting a learner undo a correction in one tap.
- Streaming feedback; similarity checks; human review of AI-graded essays.
- `questionbank`, and questions shared with `exam`.
- Server-side drafts and a revision-history table.

---

## 7. For the owner: the Render checklist

Gravity has no access to Render and should not have it. These are yours, **in this order**.

**Do not put a real AI key on the API service until this order is deployed.** Before §3.2, anyone
without an account can spend it; before §3.3, every essay holds a request open for as long as the
model takes. The worker can take a key sooner.

**After the merge and deploy:**

1. **Migrations**, against production deliberately, with `DB_DSN` set explicitly on the command.
   `.env` points `DB_DSN` at the production pooler. Check status first: phase-3-next-steps §5
   recorded five pending, not re-verified since.
2. **`S3_REGION=auto`** on the API, if not already.
3. **`WORKER_URL`** on the API service, set to the worker's base URL.
4. **Budgets before keys.** Active `ai.ai_budgets` rows for `writing_grade`, `vocab_verify`,
   `vocab_enrich_examples`, `practice_generate` and `practice_solve`, with daily request and token
   limits. **With no row there is no limit at all.** Check the creating migration for required
   columns.
5. **AI provider slots** — `AI_PROVIDER_1_*` onward — on **both** the worker and the API.
6. **`AI_WRITING_DAILY_LIMIT`**, only if 10 is wrong for you.

**Then check it on the live site:**

1. Signed out, open a writing activity: a sign-up prompt, and no new row in `ai.ai_requests`.
2. Signed in, submit an essay: the marking state, then feedback within a minute.
3. Add `schol: trường học` in My Words: *school* is added, with the note in Vietnamese.
4. A day after deploying, open today's practice twice: the same set both times. The next day:
   different items.

**Every week: read the report queue.** You chose not to review the pool or the example sentences
before learners see them. The report queue is where that review now happens, and a queue nobody
opens is not a control.

---

## 8. What keeps being found in review

**A replace is a delete.** `ReplaceActivities` removed every attempt its lesson had, through a
cascade declared two files away, under a comment explaining why wholesale replacement was the
careful choice. Before any destructive write, find everything that references the rows.

**A fallback that returns success is not a fallback.** It is a false record. When the dependency
fails, say so: `failed`, not 75.

**A guard after the side effect is not a guard.** `if result.Async` in `GradePreview` runs after the
money is spent. Put the check before the call, and prove it with a counting fake.

**A correction is a second way in unless it goes through the first.** A corrected word is verified
exactly as a typed one is, by the dictionary and the same judge, or it is a bypass.

**The mock hides latency, failure and wrong answers.** Test every AI path against a fake that sleeps,
a fake that errors, and — for anything with an answer key — a fake that is confidently wrong.

**Built and never wired**, seven times now. A 202 with nothing to complete the attempt and nothing to
read the response is the same fault as the six before it.

**The gate you did not run is the gate that fails.** Before pushing: `make lint`, the integration
suite, both codegen gates, `make arch` in the Linux container, `tsc -b`, the web tests, the build,
and Playwright including `narrow-320` in both locales. After pushing, read `gh pr checks`, not
`gh run list` — work order 10's CodeQL alert was visible only there.

**Remove the fix and watch the test fail.** In work order 10 a test that shared state with the test
before it passed with the fix removed. A test that cannot fail is not a test.
