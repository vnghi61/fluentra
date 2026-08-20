---
doc_type: work_package
phase: 2
work_package: WP10
title: "Learner web — dashboard, learn, lesson runner, review, progress"
tasks: 5
estimate: "~12 days"
blocked_by: "WP6, plus the contract task of WP7/WP8/WP9"
status: ready
last_verified: 2026-08-20
---

# WP10 — Learner web

This is the UI plan's P3–P6 and the Phase 2 half of its P8. Speaking (its P7) and
gamification are **not here** — see
[`phase-2-ui-plan-review.md`](../phase-2-ui-plan-review.md) §0.

Each task can start as soon as the matching **contract-only** task has landed (P7.1, P8.1,
P9.1), against MSW handlers typed from `web/src/types/api.ts`. It is finished when it runs
against the real endpoint. That is the whole point of the contract-first ordering: this work
package runs *beside* WP8 and WP9, not after them.

| Task | Branch |
|---|---|
| P10.1 | `feat/web-dashboard` |
| P10.2 | `feat/web-learn` |
| P10.3 | `feat/web-lesson-runner` |
| P10.4 | `feat/web-review-session` |
| P10.5 | `feat/web-progress` |

---

## Rules for every task in this work package

1. **No hand-written DTOs.** Types come from `pnpm gen:api`. A PR that declares its own
   `interface DashboardResponse` is rejected however tidy it looks — Phase 1 shipped two
   schema mismatches into the E2E suite exactly that way.
2. **Empty, loading and error states ship in the same PR as the happy path.** During the
   alpha the empty state is the *common* state, not an edge case.
3. **Bundle figure in the PR body**, before and after. 166.4 kB of a 200 kB budget as of
   2026-08-20.
4. **Both locales.** Every new string in `en.json` and `vi.json`. Vietnamese runs 20–30 %
   longer than English, which is a 320 px layout problem, not a translation problem.
5. **Accessibility in the PR, not in a later phase.** Keyboard path, visible focus, no
   colour-only status, 44 × 44 targets.

---

## P10.1 — Dashboard `M`

| | |
|---|---|
| **Depends on** | P6.6, P8.1 |
| **Context** | The review §A1 and §D1, the UI plan §5 |
| **Files** | `web/src/features/learning/`, `web/src/routes/DashboardPage.tsx` |
| **Do** | **Three cards, in this order.** (1) *Continue Learning* — the hero, one primary action, driven by the next-activity value from `GET /me/dashboard`. (2) *Reviews Due* — count, estimated minutes, one button. (3) *Skill Progress* — per-skill mastery from the same response. That is all. No streak, no XP, no daily goal, no achievements: those read `gamification` data and `gamification` is a Phase 3 module. No Quick Practice grid — it duplicates the Practice tab; fold it into the Reviews Due **empty** state instead ("nothing due — practise something anyway"). |
| **Acceptance** | Four states render correctly and each has a test: brand-new learner with no enrolment, learner mid-course, learner with nothing due, API error. The greeting does not claim a name the API does not return. 320 px clean in `en` **and** `vi`. |
| **Trap** | The UI mock shows six blocks and it is tempting to render the other three as zeros. A card that always says "0 XP" is a card that ships a lie and then has to be un-shipped. Build three. |

## P10.2 — Learn: catalogue, unit list, lesson detail `M`

| | |
|---|---|
| **Depends on** | P6.5, P7.1 |
| **Context** | The review §D2, the UI plan §6 |
| **Files** | `web/src/features/lesson/`, `web/src/routes/LearnPage.tsx` |
| **Do** | `/learn` renders the course with a progress bar, its units, and the lessons inside each. **A unit list, not a vertical node map.** Two states carry the weight — *done* and *next* — with `available` as the default and `locked` rendered as a row with a lock icon **and the reason string the API returns** ("finish Unit 2 first"). Lesson preview on selection: title, activity count, estimated minutes. |
| **Acceptance** | A learner can answer, without scrolling twice: where am I, what is next, what is done, what is locked and why. Status is never conveyed by colour alone — icon or text accompanies every state. Empty course renders an intentional state, because content seeding runs in parallel and a partial course is the normal case for weeks. |
| **Trap** | The vertical path with rings and milestone treatments is deferred on purpose (review §D2) — it is roughly five times the build for an audience of adult exam-preppers who want a syllabus. If it turns out to be wanted after the alpha, it is built then, with a reason. |

## P10.3 — Lesson runner `L`

| | |
|---|---|
| **Depends on** | P6.4, P8.1 |
| **Context** | The UI plan §7 — adopted as written, it is the best-specified part of that plan |
| **Files** | `web/src/features/learning/components/`, `web/src/routes/LessonPage.tsx` |
| **Do** | Full-screen, distraction-free. Header: exit, lesson title. Then step counter and progress bar, the activity canvas, one primary action. Three exercise types only: **multiple choice, gap-fill, flashcard**. Exit asks for confirmation. Completion shows score, accuracy and time — no XP, and no long celebration animation. |
| **Acceptance** | The runner sends the **`Idempotency-Key` HTTP header** as defined in the WP8 contract on every submit — open the same lesson in two tabs and exactly one attempt is created. Keyboard: answer selectable and submittable without a mouse. Completion animation honours `prefers-reduced-motion`. A submit failure keeps the learner's answer on screen and offers a retry rather than dropping it. |
| **Trap** | Do not implement all the exercise types the content model can express. Three types cover the seeded content; a fourth is added when content needs it. |

## P10.4 — Review session `M`

| | |
|---|---|
| **Depends on** | P6.4, P9.1 |
| **Context** | The UI plan §8 — adopted as written |
| **Files** | `web/src/features/review/`, `web/src/routes/ReviewPage.tsx` |
| **Do** | Queue → flashcard → reveal → grade → next. The card front shows the word, IPA and a listen button; the back shows the sense and examples. Four grade buttons: Again / Hard / Good / Easy. Keyboard: Space to flip, 1–4 to grade, with a hint shown on first use and never again. Session summary at the end. |
| **Acceptance** | A full queue can be cleared with the keyboard alone. Grade buttons are ≥ 44 × 44 px and distinguishable without colour. The four labels fit at 320 px in Vietnamese — check this one specifically; it is the tightest row in the app. Audio failure degrades to a disabled button with a title, never a broken control. |
| **Trap** | Do not send the grade as an integer because the keyboard map is 1–4. The wire format is the enum the P9.1 schema defines; the digits are a UI affordance. |

## P10.5 — Progress `S`

| | |
|---|---|
| **Depends on** | P6.4, P8.1 |
| **Context** | The review §0 — the Phase 2 half of the UI plan's P8 |
| **Files** | `web/src/features/learning/components/`, `web/src/routes/ProgressPage.tsx` |
| **Do** | Total study time, words mastered, and per-skill progress across vocabulary, grammar, listening and speaking — reading `GET /me/progress`. Skills with no Phase 2 data render as "not started yet", which is honest, rather than as 0 %, which reads as failure. |
| **Acceptance** | No XP, no streak, no achievements tab, no leaderboard, no radar chart. A learner with no history sees an intentional empty state. |
| **Trap** | This is the screen most likely to grow a chart library. 33.6 kB of bundle headroom; a charting dependency can be most of it. Bars built from the existing `progress` primitive are enough for four skills. |

---

## Work-package gate

- Every screen renders empty, loading and error states
- The 320 px no-horizontal-scroll spec passes in **both** `en` and `vi`
- Two tabs on one lesson produce one attempt
- The review queue can be cleared entirely from the keyboard
- No hand-written response type anywhere under `web/src/features/`
- Initial bundle under 200 kB with all five screens shipped, figure stated
