---
doc_type: handoff
phase: 4
status: planned
last_verified: 2026-09-13
---

# Phase 3 — work order 13

**Purpose.** Roadmap 4.3: a **placement test** after sign-up, and a **personal path** built from it.
A new learner is invited to an adaptive test of about fifteen minutes — vocabulary, grammar, reading
and listening — that places them between A1 and C1, followed by a short optional part in writing and
speaking that refines the per-skill picture once it is graded. The result sets where they start:
which course, which lesson, lessons below their level open without being marked done, and a **weekly
plan** sized to the minutes they said they have.

**Starts after** [phase-3-work-order-12.md](phase-3-work-order-12.md) is merged. It depends on what
that order builds: listening items with rendered audio, speaking recordings and their transcription,
the exam pool's generalised pool machinery, the generation prompts for writing and speaking items, and
the rule that a pool lesson is refused by the preview and lesson routes. It depends on work order 11
for the practice pool, `learn.item_exposures`, asynchronous grading, `practice_level`, and every fix
pull request #78 made to the pool.

**Read first.** [phase-3-plan.md](phase-3-plan.md), [ADR-0015](../adr/ADR-0015-content-exercise-core.md),
[ADR-0025](../adr/ADR-0025-anonymous-curriculum-access.md), work order 11 §8 and work order 12 §9, and
the `AGENT.md`, `API.md` and `DECISIONS.md` of `learning`, `lesson`, `user` and `admin`.

---

## 1. What exists

Checked on 2026-09-13 against `fix/wo11-review` (pull request #78). Re-check the rows marked *WO12*
against `main` once work order 12 is merged.

| Piece | State |
|---|---|
| `learn.placement_results` | Created empty in P8.2: `user_id`, `estimated_level` (A1–C2 check), `per_skill` jsonb, `taken_at`. Queries `CreatePlacementResult` and `GetLatestPlacementResult` exist; **nothing calls them** |
| `learn.skill_mastery` | Written on every graded attempt by `EstimateMastery` (BR-LEARNING-09): an EWMA projected onto CEFR bands, confidence +0.10 per attempt from 0.20. One row per user and skill |
| `placement.completed` | Declared in `learning/contract` (`EventPlacementCompleted`, `PlacementCompleted`); **published by nothing** |
| BR-LEARNING-10 | "The placement test adapts: item difficulty follows the running estimate, and it stops when the confidence interval is narrow enough or the item budget is exhausted." `learning/TESTING.md`: "Placement adaptation terminates and produces a defensible level" |
| `POST /api/v1/me/placement` | Specced in the Phase 2 plan and removed from P8.1 as Phase 4. Not in `openapi.yaml` |
| `core.learning_profiles` | `declared_level`, `target_level`, `target_exam` (`ielts`, `toeic`, `none`), `weekly_minutes_goal` (15–10080), `motivations`. Queries exist; **no service, no endpoint, no caller**, and registration creates no row |
| `core.user_preferences.practice_level` | A2, B1, B2 or null; the daily practice card asks for it when null |
| `user/contract.Reader` | `GetByID`, `GetManyByIDs`, `Exists` — summaries only. `learning` may not depend on `c_user` today |
| Courses | `learn.courses` has `cefr_from` and `cefr_to`. Seeded: `everyday-english-a2-b1`, `reading-practice` and `writing-practice` (A2–B2), plus the generated and pool courses |
| Lessons | `learn.lessons` has **no level column**: `unit_id`, `position`, `title`, `skill_focus`, `estimated_minutes`, `status` |
| Unlocking | `learning.IsUnlocked` checks `learn.lesson_prerequisites` (`min_score`) against the learner's lesson progress. Nothing else opens a lesson |
| Feature flags | `admin/contract.FlagReader.IsEnabled(ctx, key, userID)`, implemented by the admin service and exposed as `admin.Module.FlagReader()`; **consumed by nothing**. The only flag routes are `/admin/flags`, behind `system.flags` |
| Onboarding | None. `RegisterPage`, `LoginPage` and the Google callback all navigate to `/` |
| Dashboard | `DailyPracticeCard`, `ContinueLearningCard`, `ReviewsDueCard`, `SkillProgressCard` |
| Pools | `pool-practice` (A2–B2, work order 11) and `pool-exam` (A2–B2, *WO12*). Nothing at A1 or C1 |
| Generation prompts | `practice_generate.v1` and `practice_solve.v1` accept A2–B2. *WO12* adds `listening_generate` and the writing and speaking generators |

---

## 2. Decisions taken

### From the owner

| Question | Decision |
|---|---|
| Form | **Adaptive, about fifteen minutes** |
| Skills | **All four** — vocabulary and grammar, reading, listening, writing and speaking |
| Writing and speaking | **A last part the learner may skip.** The level comes from the adaptive part at once; writing and speaking refine the per-skill result when graded |
| Level range | **A1 to C1** |
| Items | **A placement pool of its own**, generated and checked like the practice pool, sharing no item with practice or exams |
| When | **Invited after sign-up, and skippable.** A learner who skips chooses a level instead |
| Retake | **After 30 days**; the new result becomes the current one, the old ones are kept |
| The path | **A starting point and a weekly plan** |

### Taken in this plan — the owner may overrule

| Question | Decision |
|---|---|
| Time limits | Adaptive part: **20 minutes** hard limit, aimed at 15. Writing and speaking: **8 minutes** |
| Item budget | Adaptive part: at most **25** scored responses |
| Stopping | The most probable band reaches **0.80** after at least **8** responses, or the budget or the time runs out |
| C2 | Not placed. A learner above C1 is placed C1 |
| Writing and speaking items | One short writing task (60–100 words) and one spoken response (45 seconds), at the placed level |
| Limits | The writing task counts toward `AI_WRITING_DAILY_LIMIT`, the recording toward `SPEECH_DAILY_RECORDINGS_LIMIT` |
| Guests | Cannot take it. The invitation follows sign-up |
| Daily practice level | A learner who has not chosen a `practice_level` gets the placed level, held to A2–B2. **A chosen preference is never overwritten** |
| Lessons below the level | **Open, not done.** A prerequisite is met when its lesson's level is below the placed level; no progress row is written, and course percentages do not move |
| Weekly plan | Stored per learner per week, built on the first visit of a week starting Monday in `Asia/Ho_Chi_Minh`; fixed for that week |
| Minutes when unset | **90 a week** until the learner sets a goal |
| Invitation switch | Flag `placement.invite`, off until the placement pool can serve a full test at every band |

**Why an estimate over bands and not a percentage.** The result has to be a CEFR band, the items carry
a CEFR band, and BR-LEARNING-10 asks for a stopping rule on confidence. A probability over five bands
gives all three directly: the next item is chosen where the estimate is least sure, and "confident
enough" is a number the tests can hold the engine to.

**Why lessons open instead of completing.** Marking a lesson done would award progress and XP the
learner did not earn, and `learn.progress` is what every dashboard, streak and course certificate
reads. Opening the lesson changes one question — may they start it — and nothing that counts.

**Why a pool of its own.** An item a learner meets while being placed would be an item they have seen
in their first exam or their first daily set, and the placement pool needs A1 and C1, which the other
pools do not hold.

**Why the flag.** An adaptive test that runs out of unseen items at a band stops estimating and starts
guessing. The invitation is off until every band can serve a full test, and the owner switches it on.

---

## 3. The work

### 0. Make the records match the design

- `learning`: add `placement_sessions` and `weekly_plans` to the tables (§3.5, §3.7), the placement,
  path and weekly plan routes to `API.md`, and `placement.completed` to what is published.
- `user`: the learning profile routes (§3.1) and a `LearningProfileReader` in the contract.
- `lesson`: the lesson level (§3.2).
- `MODULE_INDEX.md` §3 and `.go-arch-lint.yml`: `learning` gains contract edges to `c_user` and
  `c_admin`. `node tools/docgen/check-drift.mjs` compares the two, so change both together.

### 1. The learning profile, in `user`

The table and its queries exist; build the rest of the slice.

| Route | Does |
|---|---|
| `GET /api/v1/me/learning-profile` | The caller's profile, or `404 LEARNING_PROFILE_NOT_FOUND` when they have none |
| `PUT /api/v1/me/learning-profile` | Creates or replaces it, whole — the same full-replacement rule `PUT /me/preferences` follows, nullable members included |

- `declared_level` is the level a learner **says** they are; the placement result lives in `learn`
  and is never written here. The column comment already says so.
- `LearningProfileReader.GetLearningProfile(ctx, userID)` in `user/contract`, returning a DTO and
  `found=false` rather than an error when there is no row.
- An `user.learning_profile_updated` event for the audit trail, carrying the changed field names and
  not their values — the same rule `ProfileUpdated` follows.
- The erasure path already deletes the row; add the route to the export.

### 2. A level on every lesson, in `lesson`

- Migration: `learn.lessons.cefr_level core.cefr_level NULL`. Nullable: a lesson with no level is
  never opened by placement.
- Seed every curriculum lesson's level in `cmd/seed`, from the course's `cefr_from`–`cefr_to` and the
  unit it sits in, and assert in the seed test that every curriculum lesson has one.
- The admin lesson create and update routes accept it; `lesson/contract.Lesson` carries it.
- Pool lessons keep it null.

### 3. The placement pool

**The same machinery as work order 11 §3.11 and work order 12 §3.7**, pointed at a third pool:
append-only lessons under a course `pool-placement`, the same checks, the same blind solve where there
is a key, the same `learn.item_exposures`. **No item is shared** with `pool-practice` or `pool-exam`.
Test all three directions. The pool is refused by the preview grade and the lesson route exactly as
work order 12 refuses the exam pool.

**Slots.** Levels A1, A2, B1, B2 and C1.

| Kind | Per item | Target per level |
|---|---|---|
| Vocabulary multiple choice | One question, four options | 40 |
| `grammar_tense_choice` | One question, four options | 40 |
| `reading_comprehension` | A passage of 60–180 words by level, three questions | 15 |
| `listening_comprehension` | A clip of 20–60 seconds by level, three questions | 15 |
| `writing_prompt` | A 60–100 word task | 10 |
| `speaking_task` `respond` | A prompt, 45 seconds | 10 |

**Choose the vocabulary kind first.** Read the vocabulary graders' body shapes before generating
anything. Use the kind whose body is self-contained; if every vocabulary kind needs a word sense,
generate the vocabulary questions in the `grammar_tense_choice` shape with a `vocabulary` skill tag,
and record the choice in `learning/DECISIONS.md`.

**Prompts.** A2–B2 is all the existing generators accept. Add new versions — never edit a version in
place — that accept A1 to C1, or add `placement_generate`. **Every generation prompt declares
`cache: false`** and a registry test asserts it; replies are parsed through `ai.CompleteJSON`; slugs
are kebab-case. Name every new task in §7 for a budget row.

**Difficulty is the band the item was generated at.** The blind solve checks the key, not the band.
Log each item's served count and correct rate per band so a later order can recalibrate from real
responses; this order does not.

**The top-up.** `learning.top_up_placement_pool`, lock in §5, hourly: five items per slot per run
until the target, then only when a learner active in the last 30 days has fewer than five unseen items
in the slot. The generator's author is resolved when the job runs.

**An empty band is a state, not an error.** §3.5 refuses to start a test while any band is short, and
the invitation stays off (§2).

### 4. The adaptive engine

Pure Go in `learning/domain`, no database and no clock, so it can be tested exhaustively.

- **Bands.** A1, A2, B1, B2, C1, at θ = −2, −1, 0, 1, 2.
- **Response model.** The probability of a correct answer to an item of band *d* from a learner at θ
  is `c + (1 − c) · σ(1.7 · (θ − d))`, with `c` = 1/options for multiple choice and 0 otherwise. A
  question inside a passage or clip is one observation.
- **Estimate.** A probability over the five bands, uniform at the start, updated by Bayes' rule after
  every scored response.
- **Next item.** The band nearest the estimate's mean, rounded, among items this learner has not seen;
  if that band has none unseen, the nearer neighbour, and the fallback is logged.
- **Stopping.** The most probable band reaches 0.80 after at least eight responses, or the budget of
  25 responses or the time limit is reached. The placed level is the most probable band.
- **Stages.** First, vocabulary and grammar alternate until the stopping rule would stop or 14
  responses. Then one reading passage and one listening clip at the provisional level. Then, while
  the rule has not stopped and the budget allows, a second passage or clip one band towards the
  uncertainty.
- **Per skill.** Vocabulary, grammar, reading and listening each get an estimate from their own
  responses, drawn towards the overall estimate in proportion to how few they had. A skill with no
  responses is absent, never guessed.

**Tests — golden and simulated, with fixed seeds:**

- A thousand simulated learners at each band: the placed level is within one band in at least 95%, and
  exact in at least 70%.
- Every simulated test stops within 25 responses.
- A learner who answers everything correctly is placed C1; one who answers everything wrong, A1.
- A band with no unseen item never stalls the engine.
- The same responses in the same order always give the same estimate.

### 5. The placement test, on the server

**`learn.placement_sessions`**: `id`, `user_id` FK to `core.users ON DELETE CASCADE`, `status`
(`in_progress`, `completed`, `expired`), `started_at`, `deadline_at`, `stage`, `estimate` jsonb, the
served items in order with their attempt ids, `productive_status` (`offered`, `skipped`,
`in_progress`, `submitted`, `graded`), `result_id` FK to `learn.placement_results`, `completed_at`.
At most one `in_progress` session per learner, by a partial unique index. `learn.placement_results`
gains a nullable `session_id`.

| Route | Does |
|---|---|
| `GET /api/v1/me/placement` | The current result, an active session, `retake_available_at`, and `invite_available` — the flag is on, the learner has no result and no session |
| `POST /api/v1/me/placement` | Starts a session. `409 PLACEMENT_IN_PROGRESS` with the active session; `409 PLACEMENT_RETAKE_TOO_SOON` with `retake_available_at`; `409 PLACEMENT_UNAVAILABLE` when a band is short |
| `GET /api/v1/me/placement/sessions/{id}` | The current item, redacted, with `remaining_seconds` and the stage |
| `POST /api/v1/me/placement/sessions/{id}/answers` | Answers the current item with an `Idempotency-Key`; returns the next item or the result |
| `POST /api/v1/me/placement/sessions/{id}/productive` | Starts the writing and speaking part, or skips it with `{"skip": true}` |

Spec every route in `openapi.yaml` first.

- **Every answer is a `learn.attempts` row**, graded by the grader registered for its kind (ADR-0015).
  Attempts on `pool-placement` count toward no course progress, like the practice pool.
- **Exposure is recorded when an item is served**, in the transaction that serves it.
- **The clock is the server's.** An answer after `deadline_at` plus five seconds is refused. A session
  past its deadline is finished with what it has, by `learning.expire_placement_sessions` (lock in §5)
  every minute and by any read of the session. If it had fewer than eight responses, it records no
  result and the learner may start again at once.
- **Finishing the adaptive part**, in one transaction: a `placement_results` row with the placed level
  and the four measured skills; `skill_mastery` rows for those skills at the placed band with
  confidence **0.40**, raised only where the existing confidence is lower; `placement.completed` in
  the outbox.
- **Writing and speaking.** Offered after the result, never before it. They are ordinary asynchronous
  attempts. When each is graded, its band is mapped to CEFR with a table recorded in
  `learning/DECISIONS.md`, `per_skill` on the same result gains the skill, and its `skill_mastery` row
  is written the same way. `placement.completed` is not published again.
- **Retake.** 30 days after the last completed session; the new result is current, and every result
  is kept.

**Tests:**

- An answer one second after the deadline is refused; one second before, it counts.
- An abandoned session is expired by the sweep, and with the sweep removed, by the next read.
- Two answers to the same item with the same key grade once.
- A retake on day 29 is refused; on day 30 it starts.
- A session with seven responses at expiry records no result.
- A graded writing task adds `writing` to `per_skill` and publishes nothing.
- A placement pool item is never drawn into a daily set or an exam sitting, and neither pool's items
  are drawn into placement.

### 6. The starting point

**`GET /api/v1/me/path`** — the recommended courses and, for each, the lesson to start at.

- **The level used** is the current placement result; failing that, `declared_level`; failing that,
  A2.
- **Courses** are the curriculum courses whose `cefr_from`–`cefr_to` contains the level, and the nearest
  below it when none does.
- **The start lesson** is the first lesson, in course order, whose `cefr_level` is at or above the
  level.
- **Opening lessons below the level.** `IsUnlocked` treats a prerequisite as met when the required
  lesson's `cefr_level` is set and below the learner's placed level. It writes nothing. A learner with
  no placement result opens nothing this way — a declared level is a claim, not a measurement.
- **The dashboard's next activity** comes from the start lesson for a learner who has not begun the
  course, instead of the course's first lesson.
- **The daily practice set** uses the placed level, held to A2–B2, when the request names no level and
  the learner has no `practice_level`. `learning` reads the result it owns; it never writes user
  preferences.

**Tests:** a learner placed B2 in an A2–B1 course starts at its first B1 lesson, can open every A2
lesson, and the course reads 0% with no progress rows; a learner with only a declared level opens
nothing early; a chosen `practice_level` still wins.

### 7. The weekly plan

**`learn.weekly_plans`**: `user_id` FK cascade, `week_start` date, `minutes_goal`, `items` jsonb,
`created_at`, unique on the two. **`GET /api/v1/me/weekly-plan`** builds this week's plan on the first
request of the week and returns the stored one after that.

- **Minutes** come from `weekly_minutes_goal` through `LearningProfileReader`, or 90.
- **Composition,** by estimated minutes: 40% the next lessons from §3.6, 30% daily practice sets,
  20% due reviews at their measured pace, 10% one writing or speaking task. Items are whole; the plan
  rounds to whole items and never exceeds the goal by more than one item.
- **The weakest skill** — the lowest `skill_mastery` band, compared with `target_level` when set —
  gets one extra item in place of the least important one.
- **Progress** is minutes from `learn.learning_sessions` in the week, and the items done, read at
  request time — never stored in the plan.

**Tests:** a plan is identical on every request in its week and new the next Monday in
`Asia/Ho_Chi_Minh`; 90 minutes with no goal; the weakest skill gets its extra item; a learner with no
mastery and no placement still gets a plan.

### 8. In the browser

- **`/welcome`**, shown once after sign-up — email or Google — when the learner has no learning
  profile. Three steps: goal (target exam and target level), minutes a week, then the invitation.
  Starting goes to the test; skipping asks for a level, saves it as `declared_level`, and goes to the
  dashboard. Saving the profile is what ends onboarding: `/welcome` never shows again once one exists.
- **The test**, full screen: one item at a time, the clock from the server, the stage shown as
  progress, no going back. The listening player and the recorder from work order 12. At zero: "Time's
  up", then the result.
- **The result**: the placed level, the four measured skills, "writing and speaking not measured yet"
  with the option to take that part now or later, the recommended course and start lesson, and the
  weekly plan.
- **The dashboard** gains a weekly plan card with the week's progress, and an invitation card while
  `invite_available` is true.
- **Settings** gains the learning profile — goal, target level, minutes — and "Retake placement", with
  the date it becomes available.

Every screen lazy, every string through `t()` in both locales, every control at least 44 px at 320 px.

### 9. Close the record

- `learning/TODO.md`: tick adaptive placement and personalised path in the hand-written section; the
  generated list comes from `tools/docgen/data/learning.json`.
- `learning/DECISIONS.md`: the band model, the stopping rule, opening instead of completing, the
  writing and speaking CEFR table, and the vocabulary kind chosen in §3.3.
- `user/TODO.md` and `lesson/TODO.md`: what landed.
- `analytics/AGENT.md` names the funnel "signup → placement → first lesson"; leave it for analytics.

---

## 4. What must not change

- **ADR-0015.** Every placement answer is a `learn.attempts` row; no attempt table anywhere else.
- **ADR-0025.** No answer reaches the learner before they answer — not in a session read, not in the
  preview route, not in the lesson route for a pool lesson.
- **The three pools never share an item.** Pool lessons are append-only.
- **Placement completes nothing.** No progress row, XP or course percentage comes from a placement.
- **A learner's own choice wins.** A chosen `practice_level` and a saved learning profile are never
  overwritten by a placement.
- **Server time**, BR-CONTENT-01, the 200 kB bundle budget, and i18n, as before.

---

## 5. Migration numbers and lock ids

Migrations: take `1700000700` and upward. Work order 12 has `1700000600`.

Lock ids: `1_700_000_215` for the placement pool top-up and `1_700_000_701` for the session sweep —
**check first**. On 2026-09-13, 210, 211, 212 and 214 were taken; work order 12 claims 213 and 601,
and gives `speaking.purge_recordings` "the next free number", which may be 215. Take the next free one
and write it here.

---

## 6. Not in this order

- **Recalibrating item difficulty** from real responses, item statistics, and `questionbank`.
- Placing C2.
- Placement for guests.
- An adaptive daily plan that replaces the dashboard's suggestions.
- Reminders, notifications or emails about the plan — `notification` and roadmap 4.7–4.8.
- Re-placing a learner automatically when their mastery drifts.

---

## 7. For the owner

**After the merge and deploy:**

1. Migrations, against production deliberately, with `DB_DSN` set on the command.
2. Budget rows for every task §3.3 adds. With no row there is no limit at all.
3. Create the flag `placement.invite`, **off**.
4. Wait for the placement pool. At the target, a band holds 40 + 40 multiple choice items, 15
   passages and 15 clips — well over a full test's worth — and the pool fills five items per slot per
   hour. Switch the flag on when every band can serve a full test.

**Then check it on the live site:** register a new account — `/welcome` appears, once. Take the test
and close the tab halfway; come back after the time limit and start again. Finish a test, skip writing
and speaking, then do them from the result page and see the skills fill in. Check that lessons below
the placed level open and the course still reads 0%. Open the dashboard on Monday: a new weekly plan.

**Every week:** the report queue, now covering the placement pool.

---

## 8. What keeps being found in review

**A skip is not a completion.** Opening a lesson and finishing it are different facts, and only one of
them may be written down.

**An estimate is only as good as its labels.** An item's difficulty is the band it was generated for
until real responses say otherwise. Log what the responses say, even though this order does not act
on it.

**A test that can run out of items is a test that guesses.** The flag and `409 PLACEMENT_UNAVAILABLE`
exist so a thin pool is refused, not quietly served.

**A claim is not a measurement.** A declared level picks a starting course; only a placement opens
lessons early.

**Onboarding is a one-way door.** Decide what ends it — here, a saved learning profile — and test that
it never comes back.

Everything in work order 11 §8 and work order 12 §9 still applies.
