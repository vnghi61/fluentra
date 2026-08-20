---
doc_type: work_package
phase: 2
work_package: WP11
title: "Content seed, E2E, alpha readiness, release"
tasks: 4
estimate: "~8 days"
blocked_by: "all — except P11.1, which starts at P7.3"
status: ready
last_verified: 2026-08-20
---

# WP11 — Seed, E2E, ship

| Task | Branch |
|---|---|
| P11.1 | `chore/content-seed-a2-b1` |
| P11.2 | `test/e2e-learning-journeys` |
| P11.3 | `chore/alpha-readiness` |
| P11.4 | `chore/release-v0.2.0` |

---

## P11.1 — Content seed: 1 course, 8 lessons, 200 words `L`

> **Start this the day P7.3 lands. Not when you reach WP11.**

ROADMAP.md names content production as "**the real bottleneck**; staff it early", and this
is the task that proves it. Eight lessons and two hundred word senses at A2–B1, each with
IPA, audio and examples, is weeks of authoring — not an afternoon of `INSERT`s. If it
starts when this file's position in the list suggests, it is the critical path to the alpha
and everything else waits on it.

| | |
|---|---|
| **Depends on** | P7.3 (the authoring workflow must accept a draft) |
| **Context** | ROADMAP.md 2.9, `content/AGENT.md` |
| **Files** | `db/seeds/content/`, or authored through the admin API — see below |
| **Do** | Author one course of eight lessons at A2–B1 with activities of the three types the runner supports, and 200 word senses with IPA, audio and examples. Prefer **authoring through the real API** over a SQL seed file: it exercises the workflow the product depends on, and ROADMAP.md's exit criterion is "no manual DB edits needed to operate content". If a SQL seed is used for developer convenience, it must produce a state the API could also have produced. |
| **Acceptance** | `make seed` (or the documented authoring path) produces a course a learner can complete end to end. Every activity has a registered grader kind. Every word sense has audio that resolves. Re-running is idempotent. |
| **Trap** | Content authored directly as SQL, bypassing the state machine, produces rows in states the API can never reach — and then the first real author hits a transition the code has never seen. Go through the workflow. |

## P11.2 — E2E journeys `M`

| | |
|---|---|
| **Depends on** | WP10, P11.1 |
| **Context** | `web/e2e/`, `web/playwright.config.ts`, `web/AGENT.md` |
| **Files** | `web/e2e/learning/` |
| **Do** | Add the Phase 2 journeys to the existing suite, using the existing helpers in `web/e2e/helpers/auth.ts` rather than new ones. The roadmap journey is one test: **complete a lesson → cards are scheduled → review them → streak**… minus the streak, which is Phase 3. So: sign in → dashboard → start the next lesson → complete every activity → see the completion screen → return to the dashboard → see reviews due → clear the review queue → see progress move. Add the empty-state journey too: a brand-new learner with no enrolment reaches a sensible dashboard and can enrol. |
| **Acceptance** | Journeys pass on all device projects at `retries: 0`, against the real stack from `make dev` — no mocked API in E2E. The 320 px project passes in `vi` as well as `en`. Total suite time stays within the CI budget the Phase 1 job established (39 tests in ~3 minutes on one runner). |
| **Trap** | The Phase 1 suite was written against an imagined interface and every journey had to be rewritten. Open each screen and read the real labels before asserting on them. `web/AGENT.md` has the table of things that cost an hour when guessed — read it first. |

## P11.3 — Alpha readiness `M`

| | |
|---|---|
| **Depends on** | P11.2 |
| **Context** | `/OBSERVABILITY_GUIDELINE.md`, `docs/operations/runbooks/`, ROADMAP.md Phase 2 exit criteria |
| **Files** | `deploy/grafana/`, `docs/operations/runbooks/` |
| **Do** | The exit criterion is "an internal alpha with 20 real learners running for two weeks; **D1 retention measurable**". D1 retention is not measurable unless the events exist and something reads them — `learning.session_completed` and `activity.completed` are published by WP8, so build the dashboard that counts them. Add a Grafana dashboard for the learning funnel (enrol → start lesson → complete activity → return next day), alerts on grading error rate and on the due-queue job failing, and a runbook for the two things that will actually go wrong: a stuck attempt in `pending`, and a missing monthly `attempts` partition. |
| **Acceptance** | D1 retention is readable from Grafana without a manual query. Both runbooks have been walked through once by a person, and the walkthrough is what proves them. |
| **Trap** | "Measurable" means someone can read it on a Tuesday, not that the raw events exist somewhere in Loki. |

## P11.4 — Release `v0.2.0` `S`

| | |
|---|---|
| **Depends on** | P11.3 |
| **Context** | `/RELEASE_GUIDE.md`, `/CHANGELOG.md` |
| **Files** | `CHANGELOG.md`, tag |
| **Do** | Follow `RELEASE_GUIDE.md`. Update every touched module's `AGENT.md` status from `PLANNED` to its real state and bump `last_verified`. Update `MODULE_INDEX.md`. Tag and deploy to staging. |
| **Acceptance** | All five CI workflows green. `make docs-check` green. No module still claims `status: PLANNED` while containing a service. |
| **Trap** | Five modules changed from documentation to code in this phase. Their `AGENT.md` files currently describe an intention; after this phase they must describe the code. Stale module docs are how the next agent gets confidently misled. |

---

## Work-package gate — and the Phase 2 exit

- The full journey passes E2E against the real stack
- D1 retention is readable from a dashboard
- No module claims `PLANNED` while shipping a service
- `v0.2.0` tagged and on staging
- Then: 20 learners, two weeks. That part is not an agent task.
