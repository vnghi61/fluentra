---
doc_type: work_package_brief
phase: 2
work_package: WP11
title: "P11 — content seed, E2E journeys, alpha readiness, release"
tasks: 4
estimate: "~8 days of engineering, plus weeks of authoring in parallel"
blocked_by: "P11.1 starts at P7.3 (already landed); P11.2 needs P10 and P11.1"
last_verified: 2026-08-25
---

# P11 — seed, E2E, ship

**One document, four PRs.** This is the work package that turns a working system into a
product twenty people can use, and its first task should have started weeks ago.

| Order | Task | Branch | Size | Starts when |
|---|---|---|---|---|
| 1 | P11.1 content seed | `chore/content-seed-a2-b1` | L | **now** — P7.3 landed long ago |
| 2 | P11.2 E2E journeys | `test/e2e-learning-journeys` | M | P10 and P11.1 |
| 3 | P11.3 alpha readiness | `chore/alpha-readiness` | M | P11.2 |
| 4 | P11.4 release v0.2.0 | `chore/release-v0.2.0` | S | P11.3 |

Read in this order:

1. `/AGENT.md`, `/CLAUDE.md`, `/RELEASE_GUIDE.md`
2. `ROADMAP.md` §2.9 and the Phase 2 exit criteria — this work package is measured against them
   and against nothing else
3. `web/AGENT.md` §6b and its hour-costing table, before writing a single selector
4. `/OBSERVABILITY_GUIDELINE.md` and `docs/operations/runbooks/` — the nine that exist show the
   shape the two new ones take
5. [WP11-seed-e2e-ship.md](WP11-seed-e2e-ship.md) for the original rows, and
   [README.md](README.md) §Handoff notes
6. This file

---

## 1. Why this work package is different

The other work packages are judged by tests they write themselves. This one is judged by
ROADMAP.md's exit criterion, which is not a test:

> an internal alpha with 20 real learners running for two weeks, with **D1 retention
> measurable**

Three of the four tasks exist to make that sentence true, and the fourth is the release that
puts it in front of people. "Measurable" means a person reads it on a Tuesday — not that the
events are somewhere in Loki and someone could write a query.

The bar: **a learner who has never seen the app can sign up, be given something to learn,
finish it, come back tomorrow to reviews that are actually due — and the team can see, without
asking anyone, how many of them did.**

---

## 2. Where things stand

Verified against `main` @ `c43e622`.

```
db/seeds/                    rbac.sql only
cmd/seed/                    seeds accounts — the two demo logins in getting-started.md.
                             No content, no lessons, no words
content + lesson             the full authoring workflow: draft, review, publish, archive,
                             with publish gated on content being published (WP7)
learning                     attempts, rollup, dashboard, progress (WP8). cmd/api boots
                             today with NO graders registered — P9.5 changes that, and the
                             seed's activity kinds must match what it declares
deploy/grafana/dashboards/   api-overview, auth-security, cache-redis, database, jobs.
                             Five dashboards about machines, none about learners
docs/operations/runbooks/    9 runbooks: pool exhaustion, error rate, job failures, job
                             backlog, lockout spike, outbox lag, refresh reuse, slow
                             queries, slow requests. None for a stuck attempt or a missing
                             monthly partition
web/e2e/                     7 auth journeys, 1 responsive spec, helpers/auth.ts,
                             helpers/stubs.ts, helpers/mailpit.ts
web/playwright.config.ts     retries: 0, five device projects: desktop, mobile-ios,
                             mobile-android, tablet, narrow-320
CHANGELOG.md                 the Unreleased section is where this phase's entries go
```

Events already published, which P11.3 has to turn into a funnel: `activity.completed`,
`lesson.completed`, `course.completed`, `learning.session_completed` — all from WP8, all
through the outbox.

---

## 3. Trap 1 — content authored as SQL produces states the API cannot reach

The temptation is a `db/seeds/content.sql` with two hundred inserts, because it is fast and it
is idempotent by construction.

What it produces is rows in states the state machine has never emitted: a published version
with no reviewer, a lesson published while its content is still draft, an activity whose kind no
grader claims. Then the first real author hits a transition the code has never seen, and the
bug is in the workflow rather than in the seed.

ROADMAP.md's exit criterion for content is "no manual DB edits needed to operate content", and
the way to prove that is to author through the API the product ships. If a SQL seed is kept for
developer convenience, it must produce a state the API could also have produced — and something
should check that claim rather than assert it.

---

## 4. Trap 2 — the seed and the grader registry must agree, or nothing boots

P8.3's recorded decision is *registry-declared*: `cmd/api` is handed the kinds a deployment
claims to grade, and a declared kind with no grader **fails the process at boot, naming the
kind**. P9.5 supplies the first real registration.

That makes the seed's activity kinds part of a deployment contract:

- an activity kind in the seed that the registry does not know fails **the learner's request**
  with `UNSUPPORTED_ACTIVITY_KIND` (422, kind named) — one request, not the process;
- a kind declared in `cmd/api` with no grader behind it fails **the boot**.

Both behaviours were chosen deliberately over a 500 at 22:00. What this task owes is that the
three lists agree: what the seed authors, what `vocabulary` registers, and what `DeclaredKinds`
claims. Say in the PR which kinds those are.

The runner only supports three exercise types — multiple choice, gap-fill, flashcard — so the
seed authors those three and no others.

---

## 5. Trap 3 — a journey written against an imagined screen

Phase 1's E2E suite was written before the screens and every journey had to be rewritten. The
cause is in `web/AGENT.md`'s hour-costing table: the sign-out control says "Sign out" on desktop
and "Logout" on mobile; a required field's accessible name carries a marker; the OTP screen
submits itself on the sixth digit and a click races a detached element; one Mailpit inbox serves
every parallel worker, so clearing it deletes another journey's code.

Open each screen, read the real labels, and use the helpers that already exist —
`helpers/auth.ts` has `newLearner()`, `newPassword()`, `enterOtp()` and `expectSignedIn()`
precisely because each of those was a red test once.

And when a journey does go red: read `error-context.md` in the Playwright artifact before
calling it a flake. It carries the page snapshot, and it has named the cause in one line before.

---

## 6. Trap 4 — "measurable" means someone reads it, not that the data exists

D1 retention is a number about people: of the learners who did something on day zero, how many
came back on day one. The events for it are already published. The failure mode is a dashboard
of raw event counts that nobody can turn into that number without writing SQL, which means it is
not measured — it is measurable in principle, which is the state the criterion was written to
rule out.

Build the funnel as steps a person recognises: enrolled, started a lesson, completed an
activity, returned the next day. Then the retention number is a ratio of two panels that are
already on the screen.

---

## 7. The four tasks

### P11.1 — content seed: one course, eight lessons, two hundred word senses

> **Start this the day P7.3 lands. It landed in WP7. Start it now.**

ROADMAP.md names content production as *the real bottleneck; staff it early*. Eight lessons and
two hundred senses with IPA, audio and examples is weeks of authoring, not an afternoon of
inserts. Scheduled by its position in this list, it becomes the critical path and E2E waits on
it.

**Do.** One course at A2–B1, eight lessons, activities of the three types the runner supports,
and 200 word senses with IPA, audio and examples. Author through the real API (§3). Word senses
need `vocabulary` (P9.4) to exist; the course, lessons and activities do not — start there.

**Acceptance.** `make seed`, or the documented authoring path, produces a course a learner can
complete end to end. Re-running is idempotent. Every word sense has audio that resolves. Every
activity kind has a registered grader (§4). No manual DB edit is needed at any point, and the
PR says so because someone checked.

### P11.2 — E2E journeys

**Do.** Two journeys in `web/e2e/learning/`, using the existing helpers rather than new ones.

1. **The roadmap journey, minus the streak** (that is Phase 3): sign in, dashboard, start the
   next lesson, complete every activity, see the completion screen, return to the dashboard, see
   reviews due, clear the review queue, see progress move.
2. **The empty-state journey**: a brand-new learner with no enrolment reaches a sensible
   dashboard and can enrol from it.

**Acceptance.** Green on every device project at `retries: 0`, against the real stack from
`make dev` — no mocked API in E2E. The `narrow-320` project passes in `vi` as well as `en`.
Total suite time stays inside the budget Phase 1 established: 39 tests in about three minutes on
one runner. State the new total.

**Trap.** §5.

### P11.3 — alpha readiness

**Do.** Three things, all of them in service of the exit criterion.

- **The learning funnel dashboard** in `deploy/grafana/dashboards/`, from the events WP8
  publishes: enrolled, started a lesson, completed an activity, returned the next day. It is the
  sixth dashboard and the first one about learners.
- **Two alerts**: grading error rate, and the due-queue job failing. Both have a clear owner
  action, or they are noise.
- **Two runbooks** in `docs/operations/runbooks/`, matching the nine that exist in shape: an
  attempt stuck in `grading` — which is the state an async grader leaves and Phase 2 has no
  consumer to clear — and a missing monthly partition, now for two tables, `learn.attempts` and
  `learn.review_logs`.

**Acceptance.** D1 retention is readable from Grafana without a manual query (§6). Both runbooks
have been walked through once by a person, and that walkthrough is what proves them — a runbook
nobody has followed is a document, not a procedure.

### P11.4 — release `v0.2.0`

**Do.** Follow `RELEASE_GUIDE.md`. Update `CHANGELOG.md` under Unreleased. Then the part that is
easy to skip: five modules changed from documentation to code this phase — `content`, `lesson`,
`learning`, `srs`, `vocabulary` — and their `AGENT.md` files still say `status: PLANNED` in
places while shipping services. Fix the status, bump `last_verified`, and make `MODULE_INDEX.md`
match. Tag and deploy to staging.

**Acceptance.** All five CI workflows green — `build`, `ci-backend`, `ci-frontend`, `docs`,
`security`. `make docs-check` green, all three of its commands. No module claims `PLANNED` while
containing a service. `v0.2.0` tagged and on staging.

**Trap.** Stale module docs are how the next agent gets confidently misled, and after this phase
five of them describe an intention rather than the code.

---

## 8. Work-package gate — and the Phase 2 exit

- The full journey passes E2E against the real stack, on every device project, at `retries: 0`
- A learner can complete the seeded course without a manual database edit anywhere
- D1 retention is readable from a dashboard by someone who did not build it
- Both new runbooks have been walked through once
- No module claims `PLANNED` while shipping a service; `MODULE_INDEX.md` agrees
- `v0.2.0` tagged and on staging, all five workflows green

Then: twenty learners, two weeks. That part is not an agent task.

---

## 9. Definition of Done — every task here

[phase-2-plan.md](../phase-2-plan.md) §2.3, plus:

1. **`make docs-check` is three commands** — `check-drift.mjs`, `generate.mjs --check` and
   `npx markdownlint-cli2`. P11.4 touches more Markdown than any other task in the phase, and
   the third command is the one that gets skipped.
2. **Generated documentation is generated**, from `tools/docgen/data/*.json`. The status field
   P11.4 updates lives in front matter, which docgen merges rather than owns — but the endpoint,
   contract, decision and TODO blocks are generated, and hand-editing them fails CI.
3. **A test that cannot fail is not a test.** For the E2E journey, break one screen's contract
   and confirm the journey goes red rather than passing on a stale selector.
4. **One task, one diff.**

---

## 10. What this work package does **not** do

- **No new features.** If a journey needs a screen that does not exist, that is a P10 task and
  this one waits.
- **No streak in the journey.** The roadmap sentence includes it; Phase 2 does not.
- **No Phase 3 observability.** One funnel dashboard, two alerts, two runbooks.
- **No production deploy.** Staging, then twenty people, then a decision.
- **No content beyond one course and two hundred senses.** That is what the alpha needs; more is
  content debt with no reader.

---

## 11. Handoff prompt

```text
Read /AGENT.md, /CLAUDE.md and RELEASE_GUIDE.md for P11.4.
Read ROADMAP.md's Phase 2 exit criteria — this work package is measured against
them.
Then implement docs/development/phase-2/P11-seed-e2e-ship.md, task <P11.N> only,
on the branch that task's row names.

Before P11.2, read web/AGENT.md's "Things that will cost you an hour if you
guess" table and open every screen the journey touches. Assert on labels you
have read, not on labels you expect.

Before P11.1, confirm which activity kinds vocabulary registers and cmd/api
declares. The seed, the registry and DeclaredKinds must name the same set.

When done:
  1. make check and, for web work, pnpm lint / pnpm test / pnpm run build
  2. pnpm test:e2e against the stack from make dev, and quote the suite time
  3. make docs-check (all three of its commands)
  4. show me the diff, grouped by file
  5. state which acceptance criteria you verified and how; for any you could not
     verify, say so rather than assuming it holds
  6. for P11.3, say who walked the runbooks and what they found
```
