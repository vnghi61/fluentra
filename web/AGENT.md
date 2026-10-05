---
doc_type: agent_entrypoint
scope: frontend
last_verified: 2026-10-05
---

# web/AGENT.md — Frontend Entry Point

Read [`/AGENT.md`](../AGENT.md) first. This file covers the React SPA only.

---

## 1. Stack

React 19 · TypeScript 5.9 (strict) · Vite 7 · TanStack Router · TanStack Query v5 ·
Zustand · Tailwind 4 + shadcn/ui + Radix · React Hook Form + Zod · Vitest + Testing Library +
MSW · Playwright · Storybook.

Rationale for each: [`/DEPENDENCIES.md`](../DEPENDENCIES.md) §2 and
[ADR-0014](../docs/adr/ADR-0014-frontend-stack.md).

## 2. Folder map

| Path | Contains | Import rule |
|---|---|---|
| `src/app/` | Providers, router, error boundary, theme, bootstrap | May import anything |
| `src/pages/` | Route components — thin, compose features | May import feature roots |
| `src/features/<name>/` | A vertical slice: `api/`, `components/`, `hooks/`, `model/`, `index.ts` | **May import another slice only via its `index.ts`** |
| `src/components/ui/` | Design system primitives (shadcn) | Imported by everything |
| `src/components/layout/` | Shells, navigation | Imported by pages |
| `src/api/` | Generated OpenAPI client, fetch wrapper, interceptors | Imported by feature `api/` only |
| `src/hooks/` | Cross-feature hooks | — |
| `src/lib/` | Pure utilities (date, cefr, format, audio) | No React, no network |
| `src/stores/` | Zustand stores — UI/session state only | — |
| `src/types/` | Generated + shared types | — |
| `src/test/` | Setup, MSW handlers, factories | Test files only |
| `e2e/` | Playwright specs and page objects | — |

`eslint-plugin-boundaries` enforces the slice rule. A cross-slice deep import fails lint.

## 3. Feature slices (one per learner-facing capability)

`auth` · `onboarding` · `dashboard` · `courses` · `lesson` · `review` · `vocabulary` ·
`grammar` · `reading` · `listening` · `speaking` · `writing` · `exam` · `progress` ·
`gamification` · `notifications` · `settings` · `billing` · `admin`

## 4. State — pick the right tool

| Kind | Tool | Example |
|---|---|---|
| Server data | TanStack Query | lessons, progress, submissions |
| URL state | Router search params | filters, tab, page |
| Global UI | Zustand | theme, sidebar, audio player |
| Form | React Hook Form + Zod | every form |
| Local ephemeral | `useState` | open/closed, hover |

**Never copy server data into a Zustand store.** The query cache is the source of truth; a
duplicate is a stale copy waiting to be shown to a learner.

## 5. Data fetching rules

| # | Rule |
|---|---|
| F1 | Never hand-write an API type. Run `pnpm run gen:api` — types come from `api/openapi/openapi.yaml` |
| F2 | Never call `fetch` in a component. Use a hook in the feature's `api/` folder |
| F3 | Query keys are centralised per feature (`api/keys.ts`) so invalidation is greppable |
| F4 | Mutations invalidate explicitly; no blanket `invalidateQueries()` |
| F5 | Optimistic updates only where a rollback is genuinely safe (review grading, deck edits) |
| F6 | A 401 triggers a single-flight silent refresh, then one retry |
| F7 | Every list uses cursor pagination; never assume page numbers |
| F8 | Long operations (writing, speaking) use the SSE stream, with polling as a fallback |

## 6. Error handling

The mock-test hub keeps an open-sitting resume link visible while browsing exam
versions, fixed tests and the custom composer. A start refused with
`ATTEMPT_IN_PROGRESS` explains that the existing sitting must be submitted first.
The lesson runner treats zero activities as an empty state with retry/back actions,
not an unsupported activity or successful completion.
Answer feedback and explanations appear only in the learner's chosen interface
language via `lib/localized.ts` `pickLocalized`, falling back to the other
language when empty. Server grader feedback is English-only and remains hidden
in Vietnamese or when repeating the verdict, while authored learning content
(prompts, options, passages, flashcard glosses, sample answers) is never translated.

`Problem Details` → typed error → message. Every `code` has an entry in
`src/lib/errors/catalogue.ts`; a missing entry falls back to `title` **and logs a warning**, so
gaps are visible rather than silent.

| Status | UI |
|---|---|
| 422 | Map `errors[]` onto form fields; focus the first |
| 401 expired | Silent refresh, retry once |
| 401 revoked | Clear state, redirect to login with an explanation |
| 403 | Inline "no access" panel — never a blank page |
| 404 | Route-level not-found view |
| 409 | Contextual toast + refetch the affected query |
| 429 | Disable the action, count down from `Retry-After` |
| 5xx | Error boundary with retry, showing the `request_id` for support |

## 6b. Responsive & mobile — the baseline (ADR-0024)

### Product UI redesign

See `docs/ui-review/IMPLEMENTATION_PLAN.md` and `UX_AUDIT.md` for the current
source-adapted design brief and verified limitations. Reuse the shell, the
lazy-loaded `MoreSheet`/`BottomSheet`, `ErrorState`/`EmptyState` and the
route-level 404/error/loading views. `BottomSheet` portals surround
`AnimatePresence` (not the reverse). Authenticated mobile headers keep global
theme/language controls in Settings; desktop keeps them in the header.
Card radius/shadow, raised surface and XP/mission colors live in `index.css`.
Every animation respects reduced motion; no fabricated learner metrics or
unimplemented controls may be added for visual fidelity.

Dashboard refinement uses a max-1240px container with dynamic time-of-day greeting.
The user-approved initial-load bundle cap is 500 kB gzip (including static JS/CSS
dependencies); `scripts/check-bundle.mjs` still enforces it. Studio submission
status reads `latest_submission` from the owned draft detail, not a nonexistent
history endpoint, and polls while submitted/verifying. Gate 1 reports use the
generated `VerificationReport` shape with `failures` and `items_checked`.
Row 1 pairs the lesson hero (2fr) with TodayProgressCard (1fr), Row 2 pairs DailyPractice
with ReviewsDue, Row 3 pairs FoundationNext with SkillProgress, and Row 4 renders WeeklyPlan
full width (multi-column on lg). On mobile, items order: greeting, continue, today progress,
daily practice, reviews, foundation, weekly plan, CEFR skills. Data-driven omissions are
strictly preserved: no per-day weekly strip (WeeklyPlan lacks per-day data) and no daily-set
done/total or estimated minutes (LessonActivity lacks estimated_minutes and DailyPracticeSet
lacks done flags). Compact statistics retain freeze use and league opt-in without duplicate
panels. `playwright.ui-review.config.ts` captures the fixture-only Dashboard matrix against an
already-running app, with server startup disabled. Auth pages use the same lazy route
loading as learner pages; URLs and guards are unchanged.

Exam practice options below md open in a `BottomSheet` from a 'Practice options'
button, keeping the unfinished-sitting resume link visible on the page; desktop
(md+) retains the inline layout. Placement intro leads with a summary (duration
and item count only when present in data/props, followed by post-test outcome)
above collapsible test details, keeping the primary start action visible without
scrolling on 375px screens. Functional tests cover mobile sheet entry and placement
summary hierarchy; visual captures (screenshots) were not produced.


**Mobile-first is the default, not an adaptation.** Build every component at 375 px first and
enhance upward. Most learners study on a phone; the layout they see should not be the one that
was checked last.

### Breakpoints

| Token | Width | Layout |
|---|---|---|
| base | 0–639 px | Single column, bottom navigation, sheets instead of modals, tables become card lists |
| `sm` | 640 px | Two-column forms where it helps |
| `md` | 768 px | Sidebar replaces bottom navigation |
| `lg` | 1024 px | Content + sidebar + detail pane |
| `xl` | 1280 px | Max content width; do not stretch text lines further |

### Enforced rules

| # | Rule | Why it matters |
|---|---|---|
| R1 | Touch targets ≥ **44×44 CSS px**, with ≥ 8 px between adjacent targets | Below this, mis-taps dominate; it is the single most common mobile defect |
| R2 | Input `font-size` ≥ **16 px** | Below 16 px, iOS Safari zooms the viewport on focus and the learner must pinch back out |
| R3 | Honour `env(safe-area-inset-*)` | Otherwise the bottom navigation sits under the home indicator and the header under the notch |
| R4 | No hover-only affordance | There is no hover on touch. Anything revealed by hover must also be reachable by tap or be always visible |
| R5 | Layout is `visualViewport`-aware | The virtual keyboard must never cover the field being typed into — most visible on the OTP screen |
| R6 | No horizontal scroll at **320 px** | The narrowest device still in real use |
| R7 | `inputmode` and `autocomplete` set correctly on every input | `autocomplete="one-time-code"` is what lets the OS offer the OTP above the keyboard |
| R8 | Tap feedback within 100 ms | Optimistic state or a skeleton; a dead-feeling tap reads as a broken app |

R1 and R2 are enforced by lint rules (`web/eslint-local-rules/mobile-baseline.js`) and by the
design-system primitives. R6 and R1-at-320 are enforced by the `narrow-320` Playwright project,
which is the only one at 320 px — the device matrix below starts at 390 px and never checked it.

### Device matrix (Playwright projects)

| Project | Viewport | Runs |
|---|---|---|
| `mobile-ios` | 390×844, WebKit | every PR |
| `mobile-android` | 412×915, Chromium | every PR |
| `tablet` | 768×1024 | every PR |
| `desktop` | 1280×800 | every PR |
| `narrow-320` | 320×640 | every PR — runs `e2e/responsive/` only |
| `google-manual` | 1280×800, headed | manual, opt-in via `E2E_GOOGLE=1` — see `web/e2e/google/README.md` |

### Running the suite

Bring the stack up first — the journeys drive the real API, and a mocked journey is a test
of the mock (root `AGENT.md` §9, "Testing against the real stack").

```
make dev                                   # from the repository root
cd web && pnpm exec playwright test        # all projects
pnpm exec playwright test --project=desktop -g "Journey 6"   # one of them
```

Playwright serves the app itself on **5174**, beside the `make dev` container on 5173, so
the two never fight over a port. It serves the **built bundle** through `vite preview`,
not the dev server: `vite dev` transforms modules on demand and WebKit pays about 25
seconds for the first navigation of every test — most of a 30 s timeout before the journey
has done anything. `E2E_BASE_URL` points the suite somewhere else; `E2E_PORT` moves it.

Retries are **0**, on purpose. P5.4's acceptance is zero flakes, and a retry hides exactly
the flake that acceptance is about. A journey that genuinely needs longer — two accounts,
several round trips — says `test.slow()` and says why.

### Things that will cost you an hour if you guess

Each of these was found by a journey failing for a reason that had nothing to do with what
it was testing:

| Guess | What the product actually does |
|---|---|
| `Password123!@#` as a test password | Refused: it is in the breach corpus. Generate one — see `newPassword()`. |
| A display name containing the journey's tag | "admin", "support", "fluentra" and friends are refused as impersonation (BR-USER-02). `newLearner()` keeps the tag out of the name. |
| A "Confirm password" field on registration | There isn't one. |
| `getByLabel(/^Password$/)` | Labels render a required marker, so the accessible name is `Password *`. |
| Clicking "Verify & continue" after typing the code | The screen submits itself on the sixth digit; the click races a detached element. |
| Re-typing a code over filled boxes | They are not cleared after a refusal, so `onComplete` fires on the first keystroke with five stale digits. `enterOtp()` clears first. |
| `getByRole("button", { name: /Sign out/i })` | Desktop says "Sign out", mobile says "Logout". Use `expectSignedIn()`. |
| `clearMailbox()` between steps | One inbox serves every parallel worker; clearing it deletes another journey's code. Match on recipient, subject and a time bound instead. |

The per-IP OTP cap and `RATE_LIMIT_AUTH_PER_MIN` are raised for the test stack only, in
`compose.dev.yaml` and the E2E job. At the shipped values the suite refuses its own
registrations and logins from the runner's single address, and the failure reads as a
timeout rather than as a rate limit.

### Performance budget — stated for the device learners actually use

Measured on a **mid-tier Android under 4G throttling**, not a developer laptop:
LCP < 2.5 s · INP < 200 ms · CLS < 0.1 · initial download ≤ 500 kB gzip (user-approved cap).

### Mobile-specific patterns

| Situation | Pattern |
|---|---|
| Modal | Bottom sheet below `md`, centred dialog above |
| Data table | Card list below `md`, table above |
| Long form | One step per screen below `md` |
| Navigation | Bottom bar (≤ 5 items) below `md`, sidebar above |
| OTP entry | Six segmented inputs, numeric keypad, paste-fills-all, `autocomplete="one-time-code"` |
| Audio recording | Large primary control, waveform above, thumb-reachable placement |
| Destructive action | Confirmation sheet — never a hover-revealed control |

## 7. Accessibility (non-negotiable)

WCAG 2.2 AA. Every exercise is completable by keyboard alone. All audio has a transcript
available after the activity's reveal point. Focus is managed on route change and on modal
open/close. `vitest-axe` runs on every page-level component. Colour is never the only signal —
correct/incorrect states carry an icon and text.

## 8. Performance budget (enforced in CI)

| Metric | Budget |
|---|---|
| Initial static JS + CSS | ≤ 500 kB gzip |
| Route chunk | < 120 KB gzip |
| LCP | < 2.0 s |
| CLS | < 0.1 |
| INP | < 200 ms |

Tiptap (writing editor) and wavesurfer (audio) are lazy-loaded on the routes that need them.

## 9. Common tasks

| Task | Steps |
|---|---|
| New page | Route in `app/router.tsx` → page in `pages/` → compose feature components → add the E2E path |
| New feature slice | Create the folder with `api/ components/ hooks/ model/ index.ts`; export only through `index.ts` |
| Consume a new endpoint | `pnpm run gen:api` → add a hook in the slice's `api/` → add a query key → add an MSW handler |
| New form | Zod schema in `model/` → RHF → map server field errors → test the invalid path |
| New design-system component | Add under `components/ui/` with a story and an a11y test |

## 10. Testing

Practice/resource recovery (2026-10-01): practice skill links use the published
`*-foundations` courses, not archived `*-practice` slugs. Daily-level selection
explains that composition stays constant but actual difficulty changes. Resource
lists poll explicit `processing_status`; details offer content revalidation and
open ready exercises instead of posting another generation request. Exercise
generation is disabled unless the server reports `learning_suitable=true`.

| Level | Rule |
|---|---|
| Component | Query by role and accessible name. `getByTestId` only when nothing accessible exists |
| Hook | Test through a component or `renderHook`; never assert internal state |
| Network | MSW handlers generated from the OpenAPI examples — mocks cannot drift from the API |
| Async | `findBy*` / `waitFor`; never a fixed timeout |
| a11y | `vitest-axe` on page-level components |
| E2E | Only the journeys in `ARCHITECTURE.md` §16.3; zero tolerance for flakes |

## 11. Do NOT

- ❌ Hand-write an API type or response shape.
- ❌ `fetch` inside a component.
- ❌ Duplicate server state into Zustand.
- ❌ Put business rules in the frontend — the server decides scores, permissions and validity.
- ❌ Compute or send a score. Ever.
- ❌ Store an access token in `localStorage`.
- ❌ Use `dangerouslySetInnerHTML` outside the sanitised lesson-content renderer.
- ❌ Reach into another feature slice's internals.
- ❌ Add a UI library that duplicates shadcn/ui.
- ❌ Ship a component that cannot be operated by keyboard.
