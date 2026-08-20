---
doc_type: plan_review
subject: "Fluentra UI/UX Final Implementation Plan"
reviewer: Claude
status: complete
verdict: "adopt with changes — 7 of 10 UI phases belong in Phase 2, 2 belong in Phase 3, 1 is not a phase"
last_verified: 2026-08-20
---

# Review — Fluentra UI/UX Final Implementation Plan

The UI plan asked for a review in six areas (its §17 A–F). This answers all six against
what is actually in the repository on 2026-08-20, then places each of its ten phases on
the roadmap.

Every claim below was checked against a real file. Where the UI plan is wrong or
unverifiable, that is said plainly.

---

## 0. Verdict in one table

| UI plan phase | Verdict | Lands in | Why |
|---|---|---|---|
| P0 Foundation verification | **Not a phase** | precondition | Four commands. The first 20 minutes of P1, not a work package. |
| P1 Design System | **Adopt, do first** | Phase 2 · WP6 | Zero backend dependency, and it fixes a live WCAG failure. Can start today. |
| P2 App Shell & Navigation | **Adopt with a change** | Phase 2 · WP6 | Nav must not link to routes that 404 for six weeks. See §D3. |
| P3 Dashboard | **Adopt, cut in half** | Phase 2 · WP10 | Three of its six cards read data owned by `gamification`, which is **Phase 3**. |
| P4 Learning Path | **Adopt, de-gamify** | Phase 2 · WP10 | See §D2 — the node path is the wrong shape for the stated audience. |
| P5 Lesson Runner | **Adopt as written** | Phase 2 · WP10 | Correctly scoped. The strongest section of the plan. |
| P6 Vocabulary SRS | **Adopt as written** | Phase 2 · WP10 | Correctly scoped. |
| P7 Speaking AI | **Defer** | **Phase 3** | Needs `platform/ai`, `platform/media` and `speaking` — all Phase 3, all empty. |
| P8 Progress & Gamification | **Split** | Phase 2 + **Phase 3** | Progress and skill mastery are Phase 2. XP, streak, badges, achievements are Phase 3. |
| P9 Responsive, A11y & QA | **Not a phase** | per-task DoD | A terminal QA phase means building nine screens wrong and retrofitting. Phase 1 already proved the per-task approach. |

**Net:** this is a good plan pointed at the wrong calendar. Its own §14 readiness table
says Learning, Lesson, Vocabulary, Speaking and Gamification are all "Planned" — and then
its implementation order builds UI for all five anyway. Phase 2 builds the backend for the
first three. Speaking and gamification stay planned.

---

## A. Scope

### A1. Which phase is too large

**P3 Dashboard.** Six information blocks, three of which have no data source in Phase 2:

| Block | Data owner | Phase | Available in Phase 2? |
|---|---|---|---|
| Continue Learning | `learning.progress` | 2 | Yes |
| Today's Goal (XP) | `gamification.xp_events` | **3** | **No** |
| Reviews Due | `srs.review_cards` | 2 | Yes |
| Quick Practice | — (navigation only) | — | Yes |
| Skill Progress | `learning.skill_mastery` | 2 | Yes |
| Achievement Preview | `gamification.badges_earned` | **3** | **No** |
| Streak (sidebar + dashboard) | `gamification.streaks` | **3** | **No** |

Source: the `tables:` and `phase:` frontmatter in each module's `AGENT.md`.

The Phase 2 dashboard is therefore **three cards**: Continue Learning (hero), Reviews Due,
Skill Progress. That is not a downgrade — it is the whole of the plan's own §1 goal
("Tôi nên học gì tiếp theo? / Hôm nay tôi cần hoàn thành gì?") with nothing on screen that
lies.

There is a trap here worth naming, because an agent will walk into it. The `learning`
module's own endpoint table already promises:

> `GET /api/v1/me/dashboard` — Today's plan, **streak**, due reviews,
> continue-where-you-left-off

That word `streak` is a Phase 3 field sitting on a Phase 2 endpoint. Ship the endpoint
without it. Do **not** add a `streaks` table to the `learn` schema to make the dashboard
look complete — that puts a `gamification` table in another module's schema and fails
rule DB1.

### A2. What is not needed for MVP

- **Quick Practice grid.** A second route to the four things the Practice tab already
  reaches, occupying prime space above the fold. Fold it into the Reviews Due card's
  *empty* state ("nothing due — practise something anyway") and delete the grid.
- **Milestone reward treatment** on the learning path. Two node states are needed
  (done / next). Five is decoration.
- **Achievements tab.** Phase 3.
- **Twelve UI primitives up front.** See §B2.

### A3. What should be deferred

Speaking AI (P7) and the gamification half of P8, to Phase 3. Leaderboard, league,
retention forecast and radar analytics stay deferred as the plan already says — the plan
is right about those, and should apply the same test to XP and streaks.

---

## B. Architecture

### B1. Feature folders — adopt, with renaming

`web/src/features/` currently holds `auth`, `account` and `admin`, each with
`api/ · components/ · model/ · index.ts`. The plan's proposal matches that shape. Good.

But the plan names folders after *screens* (`features/learning`, `features/vocabulary`,
`features/speaking`, `features/progress`) while routes are named after *verbs* (`/learn`,
`/practice/vocabulary`) and backend modules after *domains* (`lesson`, `learning`, `srs`,
`vocabulary`). Three naming systems for one concept is how a codebase acquires a folder
nobody can find.

**Use the backend module names** — one-to-one with whatever owns the data:

```
features/lesson       → course catalogue, unit list, lesson detail   (backend: lesson)
features/learning     → dashboard, attempts, lesson runner, progress (backend: learning)
features/review       → review session, flashcards, grading          (backend: srs)
features/vocabulary   → dictionary, decks                            (backend: vocabulary)
```

Routes stay as the IA specifies: `/learn`, `/practice/vocabulary`. A route is a URL, a
folder is an owner. They do not have to match, and pretending they must is what produced
the three-way split above.

### B2. Over-engineering — one real instance

The plan's P1 builds twelve primitives before any screen exists. Six exist already
(`button`, `input`, `checkbox`, `form`, `label`, `otp-input`). Of the six missing, the
shell and a three-card dashboard need exactly four: `card`, `badge`, `progress`,
`skeleton`.

`dialog`, `tabs`, `tooltip`, `toast` and `empty-state` should be built by the first screen
that needs one. Building a `tooltip` in week one for a screen designed in week five
produces an API fitted to an imagined caller.

This is not pedantry. It is the bundle — see §E3.

### B3. Component ownership rule — adopt as written

"Không tạo global component cho business-specific UI nếu nó chỉ được dùng trong một
feature" is exactly right, and matches how `features/admin` is already built.

---

## C. API readiness — the most important change in this review

The plan says:

> Không tạo fake production API. Chỉ dùng: Typed mock adapter / MSW / Explicit fallback
> state.

The intent is right. The mechanism is not, and it will reproduce a bug this project has
already paid for once.

**A hand-written TypeScript mock type is an invented DTO.** In Phase 1 the E2E suite was
written against an imagined interface, and two schema mismatches survived until the stubs
were typed against the generated schema — `TrustedDeviceList` is `{devices: []}`, not
`{items: []}`, and `SessionList` uses `device_label`, not what the test assumed. Nothing
caught those except generation. A mock adapter authored by hand is the same failure mode
with a friendlier name.

The repository already has the answer, and it is a project rule (`CLAUDE.md` rule 2,
ADR-0005): **the spec is written first.**

```
api/openapi/openapi.yaml          ← the contract is authored HERE, before any code
        |  pnpm gen:api  (openapi-typescript)
        v
web/src/types/api.ts              ← generated; never edited by hand
        v
MSW handlers typed as components["schemas"]["DashboardResponse"]
        v
Component tests
        v
Real Go handler (oapi-codegen, from the same file)
```

Mock and real handler cannot diverge, because both are generated from one file. The
frontend never invents a type; it can only be wrong in a way the compiler catches.

**Sequencing consequence, and it is why WP7–WP9 are ordered the way they are:** each
backend work package opens with a *contract-only* task that lands the OpenAPI paths and
schemas with no implementation behind them. That one task unblocks the frontend for that
surface, so WP10 runs in parallel with WP8/WP9 instead of behind them.

### C1. DTOs the UI plan assumes that do not exist

| Plan assumes | Reality |
|---|---|
| Dashboard has `streak` and `xp` | `gamification`, Phase 3. Not in the Phase 2 response. |
| "140 / 200 XP" daily goal | No daily-goal concept exists in any module doc. It is a new `gamification` feature, not a field. |
| Flashcard shows IPA + audio | Real — `vocabulary.word_senses`. Available in Phase 2. |
| Speaking returns Pronunciation / Fluency / Grammar / Vocabulary sub-scores | Phase 3. No contract exists to build against. |
| Lesson node "Milestone" state | Not in `lesson.lessons` or `lesson.activities`. Would need a schema field; do not add one for decoration. |

---

## D. UX

### D1. Dashboard hierarchy — too much, see §A1

Three cards. And the empty state matters more than the full state, because in the first
two weeks of the alpha most learners have no progress and no due reviews. The plan's §17F
lists "New user" and "No reviews due" as edge cases; in Phase 2 they are the **common**
case. Design them first, not last.

### D2. Learning path for IELTS/TOEIC 18+ — the shape is wrong

The plan says "Không làm path quá giống game", then specifies five node states, blue rings,
lock icons and "milestone reward treatment". Those are the same thing.

For adults preparing for an exam the question is "where am I, and what is next", and a
**unit list with a progress bar and one highlighted next lesson** answers it completely, at
roughly a fifth of the build cost, with no new schema. A locked lesson renders as a row
with a lock and a *reason* ("finish Unit 2 first") — more informative than a greyed circle,
and a string the `lesson` module can actually supply from `lesson_prerequisites`.

Revisit the vertical node path after the 20-learner alpha, with a reason to build it.

### D3. Navigation to routes that do not exist yet

The IA puts Learn, Practice and Progress in the primary nav from WP6, but the screens
behind them land in WP10 — weeks later. Four dead tabs is worse than three live ones.

Register the routes in WP6 with a deliberate "coming in this release" state, or gate the
nav items behind a feature flag. The `admin` module already ships feature flags (P4.2,
delivered in Phase 1) — use that rather than inventing a mechanism.

### D4. Lesson runner — correct as specified

Distraction-free, step counter, one primary action, exit confirmation, three exercise
types. No changes.

### D5. Flashcard keyboard map — keep

Space to flip, 1–4 to grade. Cheap, and it is what a returning daily user actually uses.
Show a visible hint on first use and never again.

---

## E. Technical risks

### E1. Tailwind v4 tokens — low risk, one gotcha

`web/src/index.css` is sixteen lines: `@import "tailwindcss"`, a `@custom-variant dark`
bound to the `.dark` class, and a `body` rule. There is **no** `tailwind.config.js` and
**no** token layer at all. Tailwind v4 defines tokens in CSS via `@theme`, not in a JS
config — an agent reaching for `tailwind.config.js` is working from v3 memory and will
produce a file the build ignores in silence. Say so in the task.

The dark-mode mechanism (class on `<html>`, set by an inline script before first paint) is
already correct. Keep it. Do not switch to a media query — it cannot honour an explicit
user choice.

### E2. Light mode is not "no regression" — it is currently **broken**

The plan's P1 acceptance says "Auth pages không bị regression". Too weak, because today's
state is already wrong. From `web/src/components/ui/button.tsx`:

```
outline: "border border-slate-700 bg-transparent text-slate-200 …"
ghost:   "bg-transparent text-slate-300 …"
```

Neither carries a `dark:` prefix, and `body` is `bg-white` in light mode. `text-slate-200`
on white is roughly **1.3:1** — an outright WCAG 1.4.3 failure, shipped today. `secondary`
is a dark-grey button on a white page: legible, but visibly a dark-theme component in a
light theme.

So the design-system task's acceptance is **"light mode is fixed"**, with a contrast
assertion — not "light mode is unchanged".

### E3. Bundle — the binding constraint, again

Measured 2026-08-20 with `node web/scripts/check-bundle.mjs`:

```
initial download: 166.4 kB gzipped (budget 200 kB)
```

**33.6 kB of headroom** for a design system, a sidebar, a bottom nav, a dashboard, a lesson
runner and a flashcard deck.

Routes are already lazy (`lazyRouteComponent` in `web/src/app/router.tsx`), so screen code
is not charged to the first visit. But **the shell and every primitive it imports are
eager**, and the shell is exactly what WP6 grows. A headless-UI dependency pulled into the
shell to get a `tooltip` costs first-visit bytes for every learner, forever.

Rule for Phase 2: every PR touching `web/` states the before and after figure in its body.
Same discipline as Phase 1's Stage 0.

### E4. i18n — the 320 px matrix must run in Vietnamese

`en.json` and `vi.json` both exist. Vietnamese UI strings run roughly 20–30 % longer than
English. The Phase 1 responsive spec (`web/e2e/responsive/no-horizontal-scroll.spec.ts`)
runs at 320 px — in English. A dashboard card that fits at 320 px in English can overflow
in Vietnamese. Parameterise that spec by locale in WP10.

### E5. Audio recording and microphone permission — deferred with Speaking, to Phase 3

No Phase 2 screen records audio. Playing pre-generated TTS in a flashcard is an `<audio>`
element pointed at a `content.media_assets` URL, and carries none of the permission, codec
or upload risk the plan lists.

---

## F. Missing edge cases

The plan's §17F list is good. Five it does not cover, ordered by how much they will hurt:

1. **"Due today" is timezone-dependent.** Users already have a `timezone` preference
   (Phase 1, `user` module). If the SRS due queue is computed in UTC, a learner in
   `Asia/Ho_Chi_Minh` gets their day rolling over at 07:00 local. This is a backend
   correctness bug that surfaces as a UI complaint, and it must be decided in WP9's schema
   task rather than discovered during the alpha.
2. **Double submit / two open tabs.** The attempt lifecycle is specified as idempotent
   (ADR-0015). The *idempotency key* therefore has to be part of the HTTP contract, and
   the UI has to send it. If it stays a backend detail, the second tab silently creates a
   second attempt.
3. **Content archived mid-session.** A lesson references an immutable `content_versions`
   row by design. A learner who opened a lesson yesterday must keep the version they
   started, not get a 404 because an author archived the item. Decide in WP7.
4. **Empty or partial course.** Content seeding (WP11) is the roadmap's own stated
   bottleneck. Every learner screen will meet an empty course during the alpha.
5. **Reduced motion, on the one animation that matters.** The lesson completion screen is
   the only place Phase 2 animates. Honour `prefers-reduced-motion` there; there is
   nowhere else to worry about.

---

## Summary of changes requested

| # | Change | Where it lands |
|---|---|---|
| 1 | Move Speaking AI (P7) to Phase 3 | out of this plan |
| 2 | Move XP, streak, badges and achievements to Phase 3; keep skill mastery and study time | WP10 |
| 3 | Dashboard drops to three cards; design the empty state first | WP10 |
| 4 | Dissolve P9 into the per-task Definition of Done | plan §1.4 |
| 5 | Contract-first: OpenAPI → `pnpm gen:api` → MSW typed from generated schemas | opening task of WP7/8/9 |
| 6 | Four primitives now, the rest on demand | WP6 |
| 7 | Design-system acceptance is "light mode fixed", with a contrast assertion | WP6 |
| 8 | Feature folders named after backend modules | WP6 |
| 9 | Learning path renders as a unit list plus next lesson, not a node map | WP10 |
| 10 | Nav items for unbuilt screens go behind the existing feature flags | WP6 |
| 11 | Timezone-aware due queue; idempotency key in the attempt contract | WP9, WP8 |
| 12 | Every `web/` PR reports the bundle figure | plan §1.4 |

---

*Execution plan that adopts this review: [phase-2-plan.md](phase-2-plan.md).*
