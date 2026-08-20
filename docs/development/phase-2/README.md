---
doc_type: index
phase: 2
title: "Phase 2 work packages — handoff guide"
last_verified: 2026-08-20
---

# Phase 2 — work packages

Six files, one per work package, each sized to hand to a single agent in a single session.

| File | Tasks | Est. | Blocked by | Hand to |
|---|---|---|---|---|
| [WP6-design-system.md](WP6-design-system.md) | 6 | ~9 d | nothing | frontend agent |
| [WP7-content-lesson.md](WP7-content-lesson.md) | 5 | ~11 d | nothing | backend agent |
| [WP8-learning-engine.md](WP8-learning-engine.md) | 5 | ~12 d | WP7 | backend agent — **the careful one** |
| [WP9-srs-vocabulary.md](WP9-srs-vocabulary.md) | 5 | ~11 d | WP8 (P8.3) | backend agent |
| [WP10-learner-web.md](WP10-learner-web.md) | 5 | ~12 d | WP6 + contract tasks | frontend agent |
| [WP11-seed-e2e-ship.md](WP11-seed-e2e-ship.md) | 4 | ~8 d | all (except P11.1) | either |

Context that applies to all of them:

- [../phase-2-plan.md](../phase-2-plan.md) — scope, dependency graph, Definition of Done,
  verification gates. **Every agent reads §2 before starting.**
- [../phase-2-ui-plan-review.md](../phase-2-ui-plan-review.md) — why the UI work is shaped
  the way it is, and what was moved to Phase 3.
- [REVIEW-CHECKLIST.md](REVIEW-CHECKLIST.md) — for the reviewing agent.

---

## The handoff loop

One work package at a time. Not two, and never all six.

```
1. IMPLEMENT   → give the implementing agent exactly one WP file
                 plus phase-2-plan.md §2
                 It produces one PR per task, not one PR per WP.

2. REVIEW      → give the reviewing agent the same WP file,
                 REVIEW-CHECKLIST.md, and the diff
                 It reports findings; it does not rewrite.

3. FIX         → back to the implementing agent with the findings

4. GATE        → run the work-package gate at the bottom of the WP file
                 Do not start the next WP until it passes.
```

**Why one at a time.** WP8 defines an interface that WP9 implements and Phase 3 implements
five more times. An agent given WP8 and WP9 together will design the interface around the
one caller it can see, which is the exact failure ADR-0015 exists to prevent.

## Starting order

Two agents can run from day one:

```
Frontend agent:  WP6  ────────────────►  WP10
Backend agent:   WP7  ──►  WP8  ──►  WP9
Content:              P11.1 (starts when P7.3 lands — see the note in WP11)
```

WP10 does not wait for WP8 and WP9 to finish. It waits for their **contract-only** tasks
(P7.1, P8.1, P9.1), which are small and land early. That is deliberate, and it is what makes
the two tracks genuinely parallel rather than nominally parallel.

## What to give the implementing agent

The WP file is written to be self-contained. It still needs the repository's standing rules,
which are in `AGENT.md` and `CLAUDE.md` and are not repeated in each file. The prompt is
roughly:

```
Read /AGENT.md and /CLAUDE.md.
Read docs/development/phase-2-plan.md §2 (how to run this plan).
Then implement docs/development/phase-2/WP<N>-<name>.md, task <ID> only.

Test against the real stack: `make dev` brings it up. Do not mock the API.
If a port is taken, stop the container holding it — do not delete it.

When done:
  1. make check
  2. show me the diff, grouped by file
  3. state which acceptance criteria you verified and how
  4. for web/ changes, state the bundle figure before and after
```

One task per session. A whole work package in one session produces a diff nobody reviews.

## What to give the reviewing agent

```
Read docs/development/phase-2/WP<N>-<name>.md and REVIEW-CHECKLIST.md.
Review this diff against the acceptance criteria of task <ID>.
Report findings; do not rewrite the code.
For each finding: file, line, what is wrong, why it matters, how to verify.
If an acceptance criterion cannot be verified from the diff, say so — do not assume it holds.
```

The last line matters. Phase 1's most useful review findings were the ones that said "this
was claimed but not demonstrated", not the ones that found typos.
