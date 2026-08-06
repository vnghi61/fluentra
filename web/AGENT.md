---
doc_type: agent_entrypoint
scope: frontend
last_verified: 2026-08-06
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

## 7. Accessibility (non-negotiable)

WCAG 2.2 AA. Every exercise is completable by keyboard alone. All audio has a transcript
available after the activity's reveal point. Focus is managed on route change and on modal
open/close. `vitest-axe` runs on every page-level component. Colour is never the only signal —
correct/incorrect states carry an icon and text.

## 8. Performance budget (enforced in CI)

| Metric | Budget |
|---|---|
| Initial JS | < 200 KB gzip |
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
