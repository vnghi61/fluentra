---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-04
---

# Phase 3 — work order 6

**Purpose.** The work after PR #72, with three decisions taken. It replaces §2 of
[phase-3-work-order-5.md](phase-3-work-order-5.md).

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps — the two codegen gates, the
stale TODO checkboxes, the migration numbering — and they are all still current, except that
the numbers are now taken up to `1700000450`.

---

## 1. Where this leaves off

`main` is green at `b20cd92` (PR #72): all five workflows passed.

| WP | State |
|---|---|
| WP12 — seed beyond vocabulary | **partly done, and further along than the plan records — this order** |
| WP13 — dictionary autocomplete | done |
| WP14 — gamification | done |
| WP15 — `platform/ai` | done |
| WP16 — learner words | done |
| WP17 — explanations | done: lazy, cached by question and answer, both languages |
| WP18 — quota, warnings, queue | done |
| WP19 — admin | users, feature flags and AI usage done; content, exams and vocabulary not |
| WP20 / WP21 — four skills | not started |

---

## 2. Decisions taken

| Question | Decision |
|---|---|
| What comes next | WP12 — seed beyond vocabulary, and the exam shape |
| AI provider | Will be configured **before** this work starts; see §6 |
| Production migrations | Ran clean through `1700000450`; the ownership hazard did not fire |

The ownership warning that led every work order since #3 — `must be owner of table
content_versions` — is retired. It was real and it was reproduced locally, but this database
did not hit it. Nothing below repeats it.

---

## 3. What WP12 already has

The plan's premise has expired, and reading it without checking would waste a day.

> The seeded curriculum is eight vocabulary lessons. Every activity is a word quiz, so
> `learning`'s grader registry has exactly one grader in it.

That was true when it was written. Today `vocabularycontract.GradedKinds()` returns **seven**
kinds, and the seed authors and the runner renders all seven:

| Kind | Renderer |
|---|---|
| `vocabulary_quiz` | — (declared, not seeded) |
| `vocab_multiple_choice` | `ExerciseMultipleChoice.tsx` |
| `vocab_gap_fill` | `ExerciseGapFill.tsx` |
| `vocab_flashcard` | `ExerciseFlashcard.tsx` |
| `vocab_listen_type` | `ExerciseListenType.tsx` |
| `vocab_match` | `ExerciseMatch.tsx` |
| `vocab_reorder` | `ExerciseReorder.tsx` |
| `vocab_context_choice` | `ExerciseContextChoice.tsx` |

So tasks 1 and 3 of WP12 are substantially served. What is **not** served is the condition the
plan actually set:

> **Done when** the seeded course contains at least two activity kinds the vocabulary grader
> does not handle.

All seven are vocabulary's. One grader is still the only grader, and the startup validation
ADR-0015 requires has still never had a reason to fire. That is this work order.

---

## 4. The work

### 1. A second grader, and the first one that is not vocabulary's

ADR-0015 settles the shape and it is worth quoting because it is easy to over-build here:

> A seventh skill is a grader, not a module rebuild.

and, under Compliance:

> A skill module that defines its own attempt table fails review.

So: a `grammar` module that implements `learning.ExerciseGrader`, exposes its own
`GradedKinds()` beside vocabulary's, and owns **no** attempt table, no progress table and no
score. The attempt lifecycle stays in `learning`.

It likely needs no table at all. `content` already carries a `grammar_rule` content type —
`db/migrations/content/schema_integration_test.go` exercises it — so a grammar item is a
content version with an authored body, exactly as a vocabulary item is. Add a migration only
if something is genuinely missing, and take `1700000460` or above; `1700000450` is used.

Two kinds is the requirement, and the plan names the ground: collocation, tense, preposition.
Pick two that are honestly *not* word knowledge, because a "gap-fill with a hole where a word
goes" graded by a grammar grader is the same exercise with a new label, and it satisfies the
letter of the done-condition while leaving the registry exactly as untested as it is now.

**Wiring is where this is proved, not the grader.** `cmd/api/modules.go` builds both the
grader map and `DeclaredKinds` from `vocabularycontract.GradedKinds()`. Two lists now have to
merge into one, and the comment there — *"The map and DeclaredKinds are built from the same
list on purpose"* — is the invariant to preserve, not to work around.

**Done means:** a kind removed from the grader map fails `cmd/api` at boot with that kind
named, proved by a test that asserts the panic. Not asserted from the registry's own unit
test, which has always passed — asserted through the composition root, which is the thing that
has never been checked.

### 2. `skill_focus` must be exactly `grammar`

This one is silent and it will not be caught by any test that does not look for it.

`learn.lessons.skill_focus` is free text (`char_length BETWEEN 1 AND 50`).
`learn.skill_mastery.skill` is a CHECK over six values. `updateSkillMastery` bridges them and
**skips an unrecognised skill without an error**, deliberately, so that a content author's
typo cannot abort a learner's submission mid-transaction.

The cost is that a grammar lesson seeded as `"Grammar"`, `"grammar_basics"` or anything else
records no mastery at all, for ever, and every test still passes. The six accepted values are
in `internal/modules/learning/domain/mastery.go`.

### 3. The exam-shaped lesson

The plan asks for "a timed set with a score report, using the existing attempt and progress
machinery and no new module". Most of that machinery is already there, and knowing which part
is the difference between two days and a week.

**Server-side timing already exists.** `learn.attempts.duration_ms` is computed in
`learning/service` from the attempt's `created_at` to submission — `safeDurationMs`. It is
written on every attempt today. Nothing in the web reads it.

**A server-side lesson score already exists.** `rollupLessonAndAbove` writes
`learn.progress` at `scope = 'lesson'` with the average of the activity scores, when and only
when every activity in the lesson is complete.

**And the completion screen shows neither of them.** `CompletionScreen` is handed `scoreCount`
and `elapsedSeconds`, both counted in the browser: one incremented per correct answer, the
other a single `Date.now()` subtraction at the end. Two numbers for the same thing, from two
sources, that nothing forces to agree. An exam is exactly where they stop agreeing — the
learner who closes the tab and comes back has a client count of zero and a server score that
is correct.

**Decide which number the exam reports, and make the other one stop existing or stop being
authoritative.** Record that in `learning/DECISIONS.md`. The recommendation is the server's,
because it is the one that survives a refresh and the one every other screen already trusts.

A time limit is the genuinely new part. It has to be enforced somewhere the learner cannot
edit, which means the server, which means the deadline is a property of the lesson or the
attempt rather than a `setTimeout`. Keep it in the existing tables if it fits.

**Done means:** a learner opens the exam lesson, sees a countdown, and gets a score report
that is the same number after a refresh as before it; a learner who runs out of time gets the
report for what they answered rather than an error; and the report's score comes from
`learn.progress`, not from a counter in the tab.

### 4. Seeding it

`cmd/seed/main_test.go` already enforces a three-way agreement, and it is stricter than it
looks. Every seeded kind must appear in `GradedKinds()`, and seeded kinds and `runnerKinds`
must match **in both directions** — a renderer with nothing seeded fails the test just as a
seeded kind with no renderer does.

So a new kind is four edits landing together: the grader, the `GradedKinds()` entry, the seed
activity, and the `LessonPage.tsx` dispatch plus its `Exercise*.tsx`. Three of the four make
the suite red on their own. That is the test working.

`withLearnerGloss` keys off `target_word` and returns the config untouched when there is
none, so a grammar activity passes through it unchanged and nothing breaks. **But the
Vietnamese gloss is how the learner reads the exercise**, and a grammar item has no target
word to gloss. Decide where its Vietnamese comes from — authored in the seed beside the
English, most likely — and do not leave it English-only by default because the mechanism
happened not to apply.

---

## 5. Two small things found while checking this

Both are in `web/src/routes/LessonPage.tsx` and both are in WP12's territory, which is why
they are here rather than in a list nobody reads.

**The retry timer never resets.** `startTime` is `useState(() => Date.now())` and
`onRetryLesson` resets seven pieces of state without touching it. A learner who retries a
lesson is told they took the first run plus the second. Harmless today; it is a score report,
which is the thing this work order is about.

**`vocabulary_quiz` is declared and seeded by nothing.** It sits in `GradedKinds()` and the
seed authors none of it. The bidirectional check covers the seed against the *runner*, so this
one slips through. Either seed it or drop it — a declared kind nobody has ever exercised is
the same untested path the registry validation exists to catch.

---

## 6. Configuring the provider

Being done before this work starts, per the decision above. Recorded here so the verification
is not skipped.

```bash
AI_PROVIDER=openai_compatible
AI_BASE_URL=<the provider's OpenAI-compatible endpoint>
AI_MODEL=<model id>
AI_API_KEY=<key>
AI_TIMEOUT=120s
```

A second provider is now wired as the router's fallback, and all five keys have the same
shape:

```bash
AI_FALLBACK_PROVIDER=openai_compatible
AI_FALLBACK_BASE_URL=<second endpoint>
AI_FALLBACK_MODEL=<model id>
AI_FALLBACK_API_KEY=<key>
AI_FALLBACK_TIMEOUT=120s
```

**These go on the worker.** `config.Load` drops an environment variable whose key appears in
no defaults, requirements or env sections — silently, with no warning — so a key not on this
list is a key that does nothing. All ten are declared; `.env.example` documents them, and a
test fails if one is added without documentation.

**Seed `ai.ai_budgets` before the first real call.** `CheckQuota` returns true when it finds
no row, which is the right default for a table nobody has filled in and means the ceiling does
not exist until a row says so. One row per `(provider, task)` — and there are now two tasks.
The values are the **task strings**, not the Go constant names: `vocab_verify` and
`explain_answer`, from `internal/platform/ai/ai.go`. A row naming `verify_vocabulary` matches
nothing and silently leaves that task uncapped.

**And `provider` is the adapter name, not the deployment.** `OpenAICompatibleProvider.Name()`
returns the constant `"openai_compatible"` whatever base URL it was built with, so a Groq
primary and an OpenRouter fallback are one provider to `CheckQuota`, to `DBUsageRecorder` and
to the admin usage screen. One row covers both, their usage is summed, and the sentence in
`.env.example` telling you to give the fallback its own budget rows describes something the
code cannot currently express. Worth fixing — probably by naming a provider at configuration
time rather than in the adapter — but not in this work order unless the exam work stalls.

**Verify the mock is gone.** The worker says so at start-up when it falls back:

```text
ai: using the mock provider; verification accepts every word it is given
```

If that line is in the production log, nothing is being checked by anything.

---

## 7. What keeps being found in review

Four rounds, the same faults, each caught after it was written:

**Built and never wired.** The gamification widgets, the avatar route, `AdminAIUsage` — each
complete, tested and reachable from nothing. This order's version of that trap is a grader
that exists and a `cmd/api` that never registers it, which is why the done-condition above is
a boot failure rather than a green unit test.

**A fabricated value is worse than a missing one.** A score invented as a perfect 100, a word
marked valid by nobody, a CEFR level assigned by no model. All three compiled and passed. When
a value is not known, leave it empty and let the state say why.

**Raw SQL that no test executes is untested.** Add the integration test in the same commit as
the query, and note that `go test` caches: a schema change with no Go change is "verified" by
a stale result. Use `-count=1` after touching a migration, and run a new test twice before
believing it.

**Check the premise before building on it.** WP16 was further along than three work orders
recorded, and §3 above is the same discovery again. The plan is a specification, not a
status — read the code for status.
