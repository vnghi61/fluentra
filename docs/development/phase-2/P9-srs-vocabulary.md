---
doc_type: work_package_brief
phase: 2
work_package: WP9
title: "P9 — srs and vocabulary: FSRS, the due queue, and the first grader"
tasks: 5
estimate: "~11 days"
blocked_by: "P8.3 (merged); P8.5 for the due-count wiring"
last_verified: 2026-08-25
---

# P9 — `srs` + `vocabulary`

**One document, five PRs.** The work package is planned as a whole here because its five
tasks share one thesis and one set of boundary decisions; it is still implemented one branch at
a time, in the order below, and a PR that covers two of them is a PR nobody reviews.

| Order | Task | Branch | Size |
|---|---|---|---|
| 1 | P9.1 contracts and OpenAPI | `feat/srs-vocabulary-contracts` | S |
| 2 | P9.2 FSRS in pure functions | `feat/srs-fsrs-domain` | M |
| 3 | P9.3 schema, cards, due queue | `feat/srs-schema-due-queue` | L |
| 4 | P9.4 vocabulary: words, senses, decks | `feat/vocabulary-module` | M |
| 5 | P9.5 the first `ExerciseGrader` | `feat/vocabulary-grader` | M |

Read in this order:

1. `/AGENT.md` and `/CLAUDE.md`
2. **`docs/adr/ADR-0016-srs-fsrs.md`** — all of it, before P9.2
3. `docs/adr/ADR-0015-content-exercise-core.md` §Consequences — P9.5 is its proof
4. `internal/modules/srs/AGENT.md` §4, §5, §6 and `internal/modules/vocabulary/AGENT.md` §4–§6
5. `internal/modules/learning/contract/contract.go` — `ExerciseGrader`, `GradeResult`,
   `ReviewItem` as they are already frozen
6. `internal/modules/learning/DECISIONS.md` — the grader registry, `ReviewItem` ownership, the
   idempotent claim, and the dashboard cache. Four of this work package's decisions were
   already taken there
7. `db/migrations/learning/1700000210_create_learning_tables.sql` — the monthly partitioning
   and rotation pattern P9.3 copies
8. [WP9-srs-vocabulary.md](WP9-srs-vocabulary.md) for the original task table, and
   [README.md](README.md) §Handoff notes
9. This file

---

## 1. Why this work package is different

Two things land here that nothing else in Phase 2 can produce.

**The first is retention.** Everything WP8 built lets a learner finish a lesson. Nothing so far
brings them back tomorrow. `srs` is that mechanism, and its due queue is, per `srs/AGENT.md`
§5, *the hottest query in the product*.

**The second is proof.** `learning.ExerciseGrader` has one fake implementation and five real
ones planned for Phase 3. P9.5 is the first real one. If the interface is wrong, this is the
last cheap moment to find out — one implementation to change instead of six.

The bar for this work package: **a Phase 3 agent writing the grammar grader copies
`vocabulary/service/grader.go`, changes the scoring, and touches nothing else.**

---

## 2. Where things stand

Verified against `main` @ `c43e622` plus the merged P8.5 branch.

```
internal/modules/srs/           AGENT.md and friends, plus contract/doc.go.
                                No types. No service, no repository, no transport.
internal/modules/vocabulary/    the same: documentation and an empty contract package
db/migrations/                  no srs/, no vocabulary/. The `skill` schema does not exist
api/openapi/openapi.yaml        zero /reviews paths, zero /vocabulary paths
.go-arch-lint.yml
  m_srs                         in: modules/srs/**   mayDependOn:
                                [p_cache p_job p_telemetry c_content]   — no c_user (§4)
  m_vocabulary                  mayDependOn:
                                [p_cache p_media p_ai p_search p_telemetry
                                 c_content c_srs c_learning]
  m_learning_service            may NOT depend on c_srs (§7.1)
docs/knowledge/                 README.md only — ADR-0016 promises fsrs.md and it is owed
cmd/api/modules.go              learning.New(... Graders: nil, DeclaredKinds: nil ...)
                                so today the process boots with no grader at all
internal/modules/user/contract  Reader.GetByID → Summary{..., Timezone}, and GetManyByIDs
                                for the batched read the due queue needs
```

What `learning` already emits and drops on the floor: `GradeResult.ReviewItems`, from every
graded attempt, since P8.3. Nothing consumes them. That is the wire this work package connects.

What `learning` already returns as a lie-shaped truth: `due_reviews_count: 0`, hard-coded in
P8.5 with a comment naming WP9. §7.1 is how it stops being zero.

---

## 3. Trap 1 — there are two `ReviewItem` types on paper, and there must be one in code

`srs/AGENT.md` §4 lists a struct `srs.ReviewItem` with `{ContentVersionID, Skill,
InitialGrade}`. `learning/contract/contract.go` **already contains that struct**, field for
field, shipped in P8.1, with a recorded decision explaining why it lives there: all six skill
modules return it from `ExerciseGrader`, and if it came from `srs`, `reading`, `writing` and
friends would each need a dependency on `c_srs` they do not otherwise have.

If P9.1 declares the second one, every grader author from here on has to decide which to
import, and the mapping between two identical structs becomes a function somebody maintains.

**Do this instead:** `srs.CardWriter` accepts `learning.ReviewItem` (the module already has
`c_learning` — check the arch line before assuming), or it accepts its own narrow input type
that only `learning` constructs. Then **delete the loser from `srs/AGENT.md` §4** via
`tools/docgen/data/learning.json`, so the next agent is not offered a choice that no longer
exists.

---

## 4. Trap 2 — the due queue needs the learner's timezone, and `srs` is not allowed to read it

`.go-arch-lint.yml:389` gives `m_srs` exactly four edges: `p_cache`, `p_job`, `p_telemetry`,
`c_content`. The acceptance in P9.3 requires a day boundary at the learner's local midnight,
which means reading `user.Reader.GetByID(...).Timezone`.

So the config grows a `c_user` edge, in the P9.3 PR, with the reason in `srs/DECISIONS.md`.
That is legitimate: `user` is a platform-shaped module and `Summary` exists precisely so other
modules can render a person without reaching into `core.users`. What is not legitimate is
either workaround the constraint invites: a `timezone` column copied into `review_cards`, or a
join from `db/queries/srs/` into `core.users`. Neither would be caught by anything automated —
there is no SQL linter here, and `go-arch-lint` does not read SQL.

While you are in that file: `m_srs` is declared `in: modules/srs/**`, which works only while
the module is a single package. The moment P9.3 adds `domain/`, `repository/`, `service/` and
`transport/http/`, that form makes every package its own import target and `module.go` importing
its own service becomes a violation. `learning` hit this in P8.3 and split into four
components; do the same, and expect the same for `m_vocabulary` in P9.4.

---

## 5. Trap 3 — "due today" computed in UTC passes every test written in UTC

This is the bug the work package exists to prevent, and it ships silently. A learner in
`Asia/Ho_Chi_Minh` whose day rolls over at 00:00 UTC gets their new reviews at 07:00 local,
every day, and the only signal is that retention looks bad.

The test that proves it: two learners, two timezones, **one card with the same `due_at`**, and
different verdicts about whether it is due today. Write it before the query.

`user.Reader` gives you `GetManyByIDs` as well as `GetByID` — the batched one exists so five
modules do not each write a loop first and get refactored later. A due queue that reads one
timezone per card is the loop.

---

## 6. Trap 4 — `review_logs` is a partitioned table, and P8.2 already paid for the lessons

`srs/AGENT.md` §5 says `review_logs` is partitioned monthly. `learn.attempts` already is, and
P8.2's migration is the template. Three things it settled that you inherit rather than
rediscover:

- **Pre-create partitions in the migration** for the current and next two months. Monthly
  partitioning with no initial partitions fails on the first insert.
- **Ship the rotation job in the same task**, not later. No automatic partition creation is a
  production outage on the first of a month.
- **Take a new advisory lock id.** `learning.rotate_partitions` holds `1_700_000_210`, and the
  convention is the migration's timestamp. Two jobs on one lock id means one of them silently
  never runs.

And one that is specific to this table: a unique constraint on a partitioned table must include
the partition key. If any part of the answer path wants uniqueness — one log per card per
answer, say — check whether the constraint you want is even expressible before designing
around it. `learning/DECISIONS.md` has the worked example of what to do when it is not.

---

## 7. Five decisions to take in P9.1, not five times in five PRs

Record each in the owning module's `DECISIONS.md` through `tools/docgen/data/learning.json`.

**7.1 How does the dashboard learn the due count?** `m_learning_service` may not depend on
`c_srs`; only `m_learning` may. P8.5 returns a literal zero.
*Recommendation:* add the service edge and inject a small reader interface the service declares
— the same shape `lesson.Reader` already has inside `learning`, so the dashboard's assembly
stays in one place. The arch-lint line belongs to whichever PR wires it, and the field stops
being zero in P9.3, not before.

**7.2 Who writes review cards, and in which transaction?** `CardWriter.UpsertCards` is "called
by the exercise engine after grading". Inside the grading transaction, two modules' tables sit
in one `InTx`; after commit, a crash between the two leaves a graded attempt with no cards.
*Recommendation:* after commit, through the contract, failure logged, with `activity.completed`
in the outbox as the replayable backstop — and say so, because the alternative reads as an
oversight rather than a choice.

**7.3 Does answering a review invalidate the learner's dashboard cache?** The dashboard is
cached per learner for two minutes and the due count will live inside it. `learning` subscribes
to no events today.
*Recommendation:* accept two minutes of staleness for the alpha and write it into the cache
table in `learning/AGENT.md` §12, beside the staleness note already there. Cross-module
invalidation is a Phase 3 conversation with `gamification` in the room.

**7.4 One `ReviewItem` or two?** See §3. Decide, then delete the other from the module doc.

**7.5 Is `GET /reviews/forecast` implemented?** It is a 30-day projection on no Phase 2 screen.
*Recommendation:* specced because the module doc lists it, not implemented, recorded as
deferred in `srs/TODO.md`.

---

## 8. The five tasks

### P9.1 — contracts and OpenAPI, no implementation

**Do.** `srs/contract`: `CardWriter.UpsertCards(ctx, userID, items)`, `QueueReader.DueCount`
and `DueCards`, plus the event payloads §4 of the module doc names. `vocabulary/contract`:
`Reader.LookupWord` and `GetSenses`, and the `Grader` marker the doc describes. Then the seven
`srs` paths and the eight `vocabulary` paths from the two §6 tables.

The four grades travel as an enum — `again`, `hard`, `good`, `easy` — never as 1–4. The
keyboard map in P10.4 is a UI affordance; a wire format that encodes it is one a client can get
wrong silently.

**Acceptance.** `make gen` and `pnpm gen:api` clean, no handlers anywhere. A frontend agent can
build the entire review session against the generated types from this commit. The five
decisions in §7 are recorded.

**Trap.** §3. Do not ship two `ReviewItem`s.

### P9.2 — FSRS in pure functions

**Do.** FSRS with the published parameter set, as pure functions over
`(card state, grade, now) → next card state`, in `srs/domain/`. `now` is a parameter; there is
no clock call and no database handle inside. Stability and difficulty are modelled explicitly
per item and learner. Per-learner parameter optimisation is out of scope — it needs hundreds of
reviews per learner that do not exist.

Write `docs/knowledge/fsrs.md` in the same PR. ADR-0016 cites it as the reason the team can
safely change the model later, and it does not exist.

**Acceptance.** Property-based tests, not examples, because the invariants *are* the
specification: `easy` schedules further out than `good`, than `hard`, than `again`, for every
reachable state; interval is monotonic in stability; `again` reduces stability; no grade ever
produces a zero or negative interval; the function is total over its input domain. A grep
proves `srs/domain/` imports no I/O package.

**Trap.** The parameters are not independent knobs — changing one without simulation is
guessing. Take the published set, cite the source in a comment, leave them alone.

### P9.3 — schema, review cards, the due queue

**Do.** Schema `learn`: `review_cards`, `review_logs`, `srs_params`, `review_daily_stats`, per
`srs/AGENT.md` §5. Then the due queue, the answer path (write a log, reschedule through the
pure function from P9.2), suspend, reset, and the daily stats rollup as a job.

`review_cards.content_version_id` is a plain `uuid` with **no** foreign key: DB4 permits one
cross-schema key and it is `→ core.users(id)`. What keeps history intact is the authoring
workflow — a published version is archived, never deleted (P7.3) — not a constraint.

**Acceptance.** The timezone test in §5 passes. Every answer writes a `review_logs` row, with
`stability_before` and `stability_after`, because those logs are what makes tuning possible
later and nothing may be dropped. `GET /reviews/due-count` is cheap enough to call on every app
open — state what it costs, and note `idx_review_cards_user_due` is partial on
`suspended_at IS NULL`. Partitions for the current and next two months exist immediately after
migration, proven by inserting a log dated next month.

**Trap.** §4 and §5 and §6, all three of which live in this task.

### P9.4 — `vocabulary`: words, senses, decks

**Do.** Schema `skill`: `words`, `word_senses`, `word_relations`, `decks`, `deck_items`,
`user_word_state`. Dictionary lookup and search; learner decks alongside curated ones; mark
known or ignored.

A sense carries IPA, an audio reference and examples. **The flashcard in P10.4 renders exactly
these fields**, so what is stored here is what a learner sees — and what P11.1 has to author
200 of.

**Acceptance.** A lookup returns senses with IPA and an audio URL resolved through
`content.media_assets`, via `c_content` rather than by reading another module's table. Deck
membership is per learner. Marking a word known removes it from future scheduling — assert that
against the due queue, not against the column.

**Trap.** `vocabulary` owns `skill`, not `learn`, and must not create a review card table of
its own. That duplication is exactly what ADR-0015 exists to prevent.

### P9.5 — the first `ExerciseGrader`

**Do.** Implement `learning.ExerciseGrader` in `vocabulary/service/grader.go` for the
vocabulary activity kinds, and register it. A graded attempt returns a `GradeResult` whose
`ReviewItems` create or update cards through §7.2's chosen path.

**Wiring this task owns.** `cmd/api` currently constructs `learning` with no graders and no
declared kinds, which is why the process boots today. P8.3's recorded decision is
*registry-declared*: a kind in `DeclaredKinds` with no grader behind it **fails the process at
boot, naming the kind**. This task supplies both, and the activity kinds P11.1 authors must
match exactly — a mismatch is a failed deploy, which is the behaviour that was chosen
deliberately over a 500 at 22:00.

**Acceptance.** The loop is an **integration test**: an attempt on a vocabulary activity is
graded, a card appears, and its `due_at` equals what the pure function returns for that grade
and that `now`. `vocabulary` defines no attempt table — review-blocking. `cmd/api` boots with
the kinds declared, and refuses to boot with a kind declared and unregistered, proven by a
test.

**Trap, and the point of the exercise.** If grading needs anything from `learning` that its
contract does not expose, the interface is wrong. Say so and change it now, with one
implementation in existence. Finding that is a success; routing around it privately is how
Phase 3 inherits six copies of a workaround.

---

## 9. Work-package gate

- FSRS property tests pass; a grep proves `srs/domain/` performs no I/O
- `good` always schedules further out than `hard`, for every reachable card state
- Two learners in two timezones get correct and different day boundaries for one card
- Every answer writes a `review_logs` row; next month's row lands in next month's partition
- Completing a vocabulary activity schedules a card whose due date matches the pure function
- `vocabulary` has no attempt table, and `srs` has no second `ReviewItem`
- `cmd/api` fails to boot when a declared kind has no grader
- `GET /me/dashboard` reports a real due count, and the staleness is recorded
- Coverage ≥ 85 % on `srs`; `make check` green

---

## 10. Definition of Done — every task here

[phase-2-plan.md](../phase-2-plan.md) §2.3, plus what WP8 paid to learn:

1. **`make check` is not the whole check.** CI also runs
   `golangci-lint run --build-tags=integration ./...` and `make gen-check`. Twice in WP8 a
   widened contract left an integration suite that did not compile, because a stub implementing
   it sat behind a build tag. **When any `contract.Reader` or `CardWriter` grows a method, grep
   for every implementation, including `//go:build integration` files.**
2. **`make docs-check` is three commands** — `check-drift.mjs`, `generate.mjs --check` and
   `npx markdownlint-cli2`. The third is the one that fails and the one that gets skipped; a
   `|` inside generated text splits the table row and breaks the build.
3. **Generated documentation is generated**, from `tools/docgen/data/*.json`. The cache-strategy
   table in an `AGENT.md` is **outside** the generated markers and is hand-maintained; changing
   the JSON alone changes nothing there.
4. **A test that cannot fail is not a test.** For the timezone boundary, the FSRS invariants and
   the card due date, break the code and confirm the test goes red. Say so in the PR.
5. **One task, one diff.** No formatting sweeps; revert the two `web/` test files `make check`
   reformats.

Before finishing any task here:

```bash
make check
golangci-lint run --build-tags=integration ./...
make cover-check
make gen-check
make docs-check
```

If 5432 or 6379 is taken, see **Ports** in [README.md](README.md).

---

## 11. What this work package does **not** do

- **No per-learner FSRS optimisation.** It needs review history that does not exist.
- **No forecast screen**, and `GET /reviews/forecast` implemented only if §7.5 says so.
- **No grammar, reading, listening, speaking or writing grader.** Phase 3 — this proves the
  interface for them.
- **No gamification.** Streaks and XP read `gamification` tables that do not exist in Phase 2.
- **No review UI.** That is P10.4, against the contract from P9.1.
- **No cross-module cache invalidation** unless §7.3 is decided the other way.
- **No second attempt table, anywhere.**

---

## 12. Handoff prompt

```text
Read /AGENT.md and /CLAUDE.md.
Read docs/development/phase-2-plan.md §2.
Read docs/adr/ADR-0016-srs-fsrs.md before P9.2 and ADR-0015 before P9.5.
Then implement docs/development/phase-2/P9-srs-vocabulary.md, task <P9.N> only,
on the branch that task's row names.

§7 has five decisions that belong to P9.1. If you are starting a later task and
they are not recorded in DECISIONS.md, stop and settle them first — they are the
seams between this module and learning, and deciding them mid-task means
deciding them twice.

Before P9.3, read db/migrations/learning/1700000210_create_learning_tables.sql
and learning/DECISIONS.md: the monthly partitioning, the pre-created partitions,
the rotation job and the advisory lock convention are all already solved there.

If 5432 is taken, see "Ports" in docs/development/phase-2/README.md; do not stop
a container that may belong to another project.

When done:
  1. make check
  2. golangci-lint run --build-tags=integration ./...
  3. make cover-check, and quote the figure it printed
  4. make gen-check and make docs-check (all three of docs-check's commands)
  5. show me the diff, grouped by file
  6. state which acceptance criteria you verified and how; for any you could not
     verify, say so rather than assuming it holds
  7. say what you broke to prove the task's headline test fails
  8. state whether any contract changed, in either direction, and why
```
