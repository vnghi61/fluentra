---
doc_type: work_package
phase: 2
work_package: WP6
title: "Design system, app shell and navigation"
tasks: 6
estimate: "~9 days"
blocked_by: nothing
status: ready
last_verified: 2026-08-20
---

# WP6 — Design system, app shell and navigation

**No backend dependency. This can start today, in parallel with WP7.**

Read [`/docs/development/phase-2-plan.md`](../phase-2-plan.md) §2 before starting — the
Definition of Done and the bundle rule are there and are not repeated here.

| Task | Branch |
|---|---|
| P6.1 | `feat/web-design-tokens` |
| P6.2 | `refactor/web-primitives-tokens` |
| P6.3 | `feat/web-ui-primitives` |
| P6.4 | `feat/web-app-shell` |
| P6.5 | `feat/web-route-registration` |
| P6.6 | `chore/web-feature-scaffold` |

---

## Before you start: what is actually there

Verified 2026-08-20. Do not re-derive this; do check it is still true.

```
web/src/index.css                 @theme tokens (P6.1), @custom-variant dark, per-mode :root/.dark.
                                  No tailwind.config.js — and there must not be one.
web/src/test/tokens.test.ts       parses index.css and asserts the contrast of every pair (P6.1)
web/src/components/ui/            button, checkbox, form, input, label, otp-input
                                  — still on hardcoded indigo/slate. That is P6.2.
web/src/components/layout/        AppShell.tsx  (one file)
web/src/features/                 auth, account, admin — each api/ components/ model/ index.ts
web/src/app/router.tsx            routes declared in code, lazy via lazyRouteComponent
web/src/routes/                   HomePage.tsx (a trace-proof harness), PracticePage.tsx
web/src/i18n/                     en.json, vi.json
initial bundle                    166.6 kB gzipped of a 200 kB budget
```

**P6.1 is done.** The tokens it left you, and the one that is not obvious:

| Token | Use it for |
|---|---|
| `--color-primary` · `--color-primary-fg` · `--color-primary-hover` | the button **fill** and its own foreground. Fixed across both themes |
| `--color-primary-accent` | anything drawn **on a surface** — links, active nav, inline icons. Per-mode |
| `--color-surface` · `-muted` · `-card` · `--color-border` · `--color-text` · `-muted` | per-mode, via a `var(--x)` redirect |
| `--color-success` · `--color-warning` · `--color-danger` | fixed |

The two blues are not interchangeable and the split is the whole reason P6.1
needed a second token: `#2563eb` is 5.17:1 on the light surface but **3.90:1** on
the dark one — fine for a focus ring, a WCAG failure for a link. Using
`--color-primary` for text in dark mode reintroduces exactly the bug P6.2 exists
to remove.

**Tailwind v4 defines tokens in CSS, with `@theme`. It does not read `tailwind.config.js`.**
An agent working from Tailwind v3 memory will create that file, the build will ignore it in
silence, and the colours will look right by accident because `slate` and `blue` are
built-ins. If you find yourself writing `module.exports = { theme: ...`, stop.

---

## P6.1 — Design tokens `S`

| | |
|---|---|
| **Depends on** | — |
| **Context** | `web/src/index.css`, `web/AGENT.md`, ADR-0024 (mobile-first), the UI plan §3 |
| **Files** | `web/src/index.css` |
| **Do** | Add an `@theme` block defining the palette as **semantic** tokens, not raw colours. Primary blue `#2563EB`, success/progress green `#22C55E`, plus attention (amber), destructive (red) and a slate neutral ramp. Name them by role — `--color-primary`, `--color-primary-fg`, `--color-surface`, `--color-surface-muted`, `--color-border`, `--color-text`, `--color-text-muted`, `--color-success`, `--color-warning`, `--color-danger` — and define the dark values under the existing `.dark` variant. Keep `@custom-variant dark` exactly as it is. Add spacing, radius and shadow scales only if the defaults are actually insufficient; Tailwind's own scales are fine and adding a parallel one is pure cost. |
| **Acceptance** | Every token resolves in both themes. `body` uses tokens rather than `bg-white dark:bg-slate-950`. A test asserts the computed background differs between `<html>` and `<html class="dark">`. Bundle figure stated in the PR — CSS growth here should be under 1 kB gzipped. |
| **Trap** | Do not create `tailwind.config.js`. Do not switch the dark mechanism to `prefers-color-scheme` — the inline script in `index.html` sets the class before first paint precisely so an explicit user choice survives and nothing flashes. |

## P6.2 — Move the six existing primitives onto tokens, and fix light mode `M`

| | |
|---|---|
| **Depends on** | P6.1 |
| **Context** | `web/src/components/ui/*`, WCAG 2.1 SC 1.4.3 |
| **Files** | `web/src/components/ui/{button,input,label,checkbox,form,otp-input}.tsx`, `web/src/test/contrast.test.ts` (new) |
| **Do** | Replace every hardcoded `indigo-*`, `slate-*` and `rose-*` class with a semantic token. **Light mode is currently broken and this task fixes it, it does not preserve it.** `button.tsx` has `outline: "… text-slate-200"` and `ghost: "… text-slate-300"` with no `dark:` prefix, on a `bg-white` body: roughly 1.3:1, a shipped WCAG failure. `secondary` is a dark-theme button rendered on a light page. Every variant must be legible in both themes. |
| **Acceptance** | `grep -rE "indigo-\|slate-[0-9]" web/src/components/ui/` returns nothing. A test computes the contrast ratio of every button variant against its background in both themes and asserts ≥ 4.5:1 for text, ≥ 3:1 for borders and focus rings. **Phase 1's 39 E2E tests still pass** — they exercise every auth screen and are the regression net for this task. |
| **Tests** | `contrast.test.ts` — table-driven over `variant × theme`. Not a snapshot test; a snapshot would happily record the broken colour. |
| **Trap** | The focus ring is `focus-visible:ring-indigo-500` today. It becomes `--color-primary` — the ring may, because it only needs 3:1. **Text may not.** Link and ghost-button text takes `--color-primary-accent`; `--color-primary` as text is 3.90:1 on the dark surface, which is the failure this work package exists to remove. `contrast.test.ts` extends `tokens.test.ts`, it does not replace it. |

## P6.3 — The four primitives the shell and dashboard need `S`

| | |
|---|---|
| **Depends on** | P6.2 |
| **Context** | The review §B2 — four now, not twelve |
| **Files** | `web/src/components/ui/{card,badge,progress,skeleton}.tsx` |
| **Do** | `card` — surface, border, optional header/footer slots. `badge` — status pill with a variant per semantic colour, **and an icon or text label, never colour alone**. `progress` — a labelled bar with `role="progressbar"` and correct `aria-valuenow/min/max`. `skeleton` — a shimmer block that respects `prefers-reduced-motion`. |
| **Acceptance** | Each has a unit test. `progress` announces correctly to a screen reader (assert the ARIA attributes). `skeleton` renders static under `prefers-reduced-motion: reduce`. No new runtime dependency. |
| **Trap** | Do **not** build `dialog`, `tabs`, `tooltip`, `toast` or `empty-state` here. They are built by the first screen that needs one, in WP10, so their API is fitted to a real caller. If you reach for a headless-UI package, price it first — it lands in the eager shell bundle and there are 33.6 kB left. |

## P6.4 — AppShell: sidebar, header, bottom navigation `M`

| | |
|---|---|
| **Depends on** | P6.3 |
| **Context** | `web/src/components/layout/AppShell.tsx`, the UI plan §2 and §11, ADR-0024 |
| **Files** | `web/src/components/layout/{AppShell,Sidebar,Header,MobileNav}.tsx` |
| **Do** | Desktop ≥ 1024 px: a 260 px sidebar with Dashboard / Learn / Practice / Progress, then a divider, then profile and settings. Mobile < 640 px: a top header plus a bottom nav with Home / Learn / Practice / Progress. Tablet: collapsed sidebar or header nav. **Settings lives under the avatar, and Logout is not in the primary navigation** — both as the UI plan specifies. Leave the sidebar's streak/XP slot out entirely; that data is Phase 3 and an empty box is worse than no box. |
| **Acceptance** | Active route is indicated by more than colour (weight, an indicator bar, and `aria-current="page"`). Every nav target is ≥ 44 × 44 px — the `mobile-baseline` ESLint rules already enforce this and must stay green. No layout shift when the theme toggles. Bundle: this task is the largest eager addition in WP6; state the figure. |
| **Trap** | The shell is eager on every route. Anything imported here is downloaded by a learner who only ever visits `/login`. Import icons individually from `lucide-react`, never the barrel. |

## P6.5 — Register the routes, behind flags where the screen does not exist `S`

| | |
|---|---|
| **Depends on** | P6.4 |
| **Context** | `web/src/app/router.tsx`, the review §D3, `admin` feature flags (Phase 1 P4.2) |
| **Files** | `web/src/app/router.tsx`, `web/src/routes/` |
| **Do** | Register `/`, `/learn`, `/learn/lesson/$lessonId`, `/practice`, `/practice/vocabulary`, `/progress`, `/settings`, `/admin` — all lazy except the shell and the auth screens. The screens behind Learn, Practice and Progress land in WP10, weeks from now: until then either gate the nav item behind the existing feature-flag mechanism, or render a deliberate "coming in this release" state. Pick one and use it for all three. **Four nav items that 404 is worse than three that work.** Retire `HomePage.tsx` as the trace-proof harness — the ping button moves to a dev-only route or goes away; a learner should not see it. |
| **Acceptance** | Every registered route renders something intentional. `web/src/test/bootstrap.test.ts` still passes (it asserts no login-screen flash on the route sequence). Initial bundle **does not rise** — these are lazy routes. |
| **Trap** | `lazyRouteComponent` takes the named export as its second argument. Getting it wrong fails at navigation, not at build, so click every route once. |

## P6.6 — Feature scaffolding and the MSW harness `S`

| | |
|---|---|
| **Depends on** | P6.1 |
| **Context** | The review §B1 and §C, `web/src/features/admin/` as the shape to copy, `msw@^2` is already a devDependency |
| **Files** | `web/src/features/{lesson,learning,review,vocabulary}/{api,components,model,index.ts}`, `web/src/test/msw/` |
| **Do** | Create the four feature folders named after the **backend modules** that own the data (see the review §B1 for why not `features/dashboard`). Set up an MSW server for component tests whose handlers are typed against `components["schemas"][...]` from `web/src/types/api.ts`. Add a test that fails when a handler's payload does not satisfy its generated schema. |
| **Acceptance** | A deliberately wrong mock payload fails `pnpm test` at the type level. `pnpm gen:api` regenerates `src/types/api.ts` cleanly from the current spec. No production code imports anything under `src/test/`. |
| **Trap** | This is the task that decides whether WP10 can run in parallel with WP8 and WP9. If a mock type is hand-written here, the parallelism is fake and the mismatch surfaces during integration — which is exactly what happened in Phase 1 with `TrustedDeviceList`. Generated types or nothing. |

---

## Work-package gate

WP6 is done when:

- Light **and** dark are correct on every existing auth screen, proven by the contrast test
- Phase 1's 39 E2E tests pass unchanged
- The 320 px no-horizontal-scroll spec passes with the new shell
- The initial bundle figure is recorded in the last PR and is under 200 kB
- A mock payload that does not match the OpenAPI schema fails the test suite
