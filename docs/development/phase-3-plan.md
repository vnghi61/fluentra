---
doc_type: execution_plan
phase: "3"
status: ready
owner: "@ai-team"
last_verified: 2026-08-29
---

# Phase 3 — Skills & AI · Execution Plan

> **Deliverable at the end of this plan:** a learner types a word the dictionary
> has never seen, gets it checked and corrected before it is saved, and finds it
> already carrying a definition, a pronunciation and its own question set. When
> they answer a question they are told not just whether they were right but what
> the word means and why that answer is the answer — in the language they chose.
> They earn XP for it, keep a streak, and can see a league they opted into. None
> of this stops when the free model quota runs out: the work queues, and the
> flashcards keep working.
>
> An administrator can see what the AI is spending, approve what learners have
> contributed, and edit the curriculum without touching the database.

**Base:** `main` after Phase 2. All five CI workflows green.
**Roadmap entry:** ROADMAP.md § "Phase 3 — Skills & AI (8 weeks)".
**Gate:** ADR-0011 and ADR-0012 Accepted; speech provider chosen (Azure Speech,
plan-review Q2). Both met.

---

## 0. The rule that decides the cost of this phase

> Look in the database first. Always. Only call a model for something the
> database does not know, and write the answer back so it is never asked twice.

This is not an optimisation to add later. On free-tier providers the scarce
resource is not money, it is quota, and every model call for a word the system
already knows is quota spent on nothing. Three consequences run through every
work package below:

- **Autocomplete is a cost control, not a convenience.** Suggesting existing
  words as the learner types is the cheapest thing in this plan and the highest
  leverage: every suggestion accepted is a model call that never happens.
- **Explanations are generated once per question and stored**, not per answer.
  The first learner to reach a question pays for it; everyone after reads it.
- **The dictionary is an asset that grows.** Each validated new word makes the
  next learner cheaper. Cost per learner falls over time by construction.

---

## 1. Work packages

| WP | Content | Needs | Est. |
|---|---|---|---|
| **WP12** | Seed expansion: gap-fill and exam-shaped items | nothing | ~2 d |
| **WP13** | Dictionary autocomplete on the add-a-word field | nothing | ~2 d |
| **WP14** | `gamification`: XP, streaks, opt-in leagues, dashboard widgets | nothing | ~8 d |
| **WP15** | `platform/ai`: providers, routing, cache, quota, usage, queue | nothing | ~10 d |
| **WP16** | Learner-contributed words: validate, correct, enrich, generate | WP15 | ~5 d |
| **WP17** | Answer explanations, stored and translated | WP15 | ~3 d |
| **WP18** | Quota aggregation, warnings, queue fallback | WP15 | ~5 d |
| **WP19** | Admin: content, vocabulary, AI quota, exams | WP16 + WP18 | ~5 d |
| **WP20** | `reading` + `writing` | WP15 | ~10 d |
| **WP21** | `platform/media` + `listening` + `speaking` | WP15, Azure Speech | ~15 d |

**WP12, WP13 and WP14 need no AI, no provider and no money.** They are
deliverable immediately and in parallel with WP15, which is the gate for
everything after it.

---

## WP12 — Seed beyond vocabulary

The seeded curriculum is eight vocabulary lessons. Every activity is a word
quiz, so `learning`'s grader registry has exactly one grader in it and the
exercise engine has never been exercised by a second shape.

**Tasks**

1. Author gap-fill items that are not vocabulary recall — collocation, tense,
   preposition — so `vocab_gap_fill` stops meaning "a word quiz with a hole".
2. Add an exam-shaped lesson: a timed set with a score report, using the
   existing attempt and progress machinery and no new module.
3. Extend `cmd/seed` with the new kinds, keeping `withLearnerGloss` working for
   any activity that names a word.

**Done when** the seeded course contains at least two activity kinds the
vocabulary grader does not handle, and the grader registry refuses to start with
one unregistered — the startup validation `learning` already performs and has
never had a reason to fire.

---

## WP13 — Autocomplete from the dictionary

**Tasks**

1. A combobox on the add-a-word field, backed by `GET /vocabulary/search`, which
   already exists and is already paginated.
2. Selecting a suggestion adds the existing sense — the current
   `AddWordToDeckRequest` path, unchanged.
3. Typing something with no match is what opens the WP16 flow, and only then.

**Done when** adding a word that already exists makes **no** model call, proven
by a test that fails if one is attempted.

---

## WP14 — Gamification

Every decision here is already recorded in
`internal/modules/gamification/DECISIONS.md` and none of it is open:

- XP is **the best score ever, divided by ten, granted as the increase over what
  has already been granted**. 80/100 grants 8; retaking to 100/100 grants 2; a
  third attempt grants nothing. XP cannot be farmed by repetition and never
  falls.
- The high-water mark lives **in this module**, not in `learn.progress`, whose
  `score` column holds the most recent score and drops on a bad retake.
- Streak boundaries use **the learner's own timezone**, which `core.users`
  already stores.
- **No global leaderboard.** Opt-in leagues, showing the display name and only
  inside a league the learner joined.

**Tasks**

1. `xp_events`, `streaks`, `badges`, `badges_earned`, `leaderboard_snapshots`.
2. Consume `activity.completed` from the outbox — `ActivityCompleted` already
   carries `Score`, so the high-water delta is computable with no change to
   `learning`.
3. Idempotent XP: the award is `max(0, best_new − best_granted)`, which is
   naturally idempotent and needs no dedupe table.
4. Streak with freezes, evaluated in the learner's timezone.
5. Dashboard: an XP bar and a streak, and a league board behind opt-in.

**Done when** the dashboard shows real XP and a real streak — and the two tests
that currently assert their **absence** (`dashboard.test.tsx:217`,
`progress.test.tsx:164`) are replaced by tests asserting the real numbers. Those
guards exist to stop a fake "0 XP" shipping; retiring them is part of this work
and must be deliberate, not incidental.

---

## WP15 — `platform/ai`

The module is two lines today. Its backlog in `internal/platform/ai/TODO.md`
already specifies this work and its tables: `ai_requests`, `ai_usage`,
`prompt_versions`, `ai_cache_entries`, `ai_budgets`.

**Tasks**

1. Provider interface, registry, and the **mock provider** — which is what lets
   every later work package be tested without spending quota.
2. Two or three free-tier adapters behind the interface. ADR-0011 is absolute
   here: **business code must never know which provider answered.**
3. Task-based routing: a caller asks for `vocabulary.validate`, not for a model.
4. Prompt registry, versioned, with output schemas (ADR-0012).
5. Resilience: timeout, retry, breaker, fallback between providers.
6. Exact-hash response cache, consulted before any provider is.
7. `ai_usage` recording, per provider and per task.
8. Queue: a task that cannot run now becomes a River job rather than an error.
9. Eval harness with golden sets, gated in CI.

**Done when** every skill module can call `ai.Do(ctx, task, input)` and the mock
provider serves the whole test suite with no network.

---

## WP16 — Words a learner contributes

**Decided (ADR-0026):** a validated word enters the **shared dictionary**
immediately, flagged `unverified`, with an admin review queue behind it. The
learner is not blocked; the dictionary still grows; a mistake is correctable.

**Flow**

1. Learner types a word. WP13's autocomplete offers what exists. **Stop here for
   anything already known — no model call.**
2. No match → `vocabulary.validate` runs. If the word is misspelled or not a
   word, the learner is **shown the correction and asked**, and nothing is saved
   until they confirm.
3. Confirmed → `vocabulary.enrich` produces definition, Vietnamese gloss, IPA,
   part of speech, an example; `questionbank.generate` produces its question set.
4. Written to `skill.words` / `skill.word_senses` with `verified_at IS NULL`,
   and a content version carrying the flashcard fields — the same shape
   `cmd/seed` writes, so the review card renders it with no special case.

**When quota is exhausted**, steps 3–4 become a queued job and the word is added
**now** with what is known: the lemma the learner typed and any definition they
supplied. The flashcard works immediately; enrichment fills in behind it. A
learner never waits on a provider.

**Done when** a learner can add an unknown word with the AI provider switched
off entirely and still review it as a flashcard the same minute.

---

## WP17 — Explanations

**Generated once per question and stored on the content version**, never per
answer. Each option gets: what the word means, what the answer is, and why —
with the Vietnamese alongside, following the ordering the flashcards already use
(the learner's chosen language leads; the English is never dropped).

**Done when** answering the same question twice makes exactly one model call in
total, across all learners, proven by a usage assertion rather than by
inspection.

---

## WP18 — Quota, warnings and the queue

**Free tiers are limited on two axes, and conflating them is the trap.** There is
a **volume** ceiling (requests per day) and a **rate** ceiling (requests or
tokens per minute). They need different responses:

| Condition | Response | Who is told |
|---|---|---|
| Rate limit hit | Queue, retry in seconds | **Nobody.** It resolves itself |
| Volume nearly gone | Queue non-urgent tasks | Admin |
| Volume exhausted everywhere | Queue everything | Learner: "your answer is queued" |

Aggregate capacity is **not one percentage**. Providers count in different units
— requests per day, tokens per minute, credits — so the only honest aggregate is
per task: "vocabulary validation: about 800 left today". A single number would
be a fiction.

**Done when** the learner-facing warning fires only on the last condition, and
turning every provider off degrades the product to queued enrichment and working
flashcards rather than to errors.

---

## WP19 — Admin

Today: users and feature flags, reached from the account menu. Adding:

1. **Lesson content.** `content` already has the draft → review → published →
   archived state machine and six `/admin/content/*` endpoints with no screen.
   Cheapest item here: the backend is done.
2. **Vocabulary**, including the WP16 review queue for `unverified` words.
3. **AI quota**: remaining capacity per task, which provider is serving, what is
   queued. Built on `ai_usage`.
4. **Exams**: the WP12 sets, and the question sets WP16 generates.

---

## WP20 / WP21 — The four skills

`reading` and `writing` need WP15 and nothing else. `listening` and `speaking`
need `platform/media` (presigned upload, transcode, waveform, TTS) and Azure
Speech, and are the only part of this plan that costs real money. They are last
for that reason, not because they matter least.

---

## 2. Decisions this plan records

**ADR-0026 — Learner-contributed vocabulary enters the shared dictionary.**
Validated by a model before it is saved, flagged `unverified`, admin-reviewable.
Rejected: personal-only decks, because the dictionary then never grows; and
unvalidated writes, because one learner's typo becomes everyone's.

**ADR-0027 — Free-tier provider routing.** Two or three providers wired directly
behind ADR-0011's interface. A self-hosted gateway such as OmniRoute is **one
provider behind that same interface** if it is ever adopted, deployed somewhere
that does not sleep — its quota state is SQLite on local disk, which a free host
wipes on every restart, and quota tracking that forgets is worse than none.
Keeping the interface is what makes that choice reversible. Routing learner text
through any third party must also honour `ai_processing_opt_out`, which
`user_preferences` already carries.

---

*Part of the Fluentra knowledge base. Entry point: [/AGENT.md](../../AGENT.md) · Map: [/MODULE_INDEX.md](../../MODULE_INDEX.md)*
