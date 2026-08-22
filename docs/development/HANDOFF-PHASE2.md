---
doc_type: handoff
phase: 2
status: in_progress
last_verified: 2026-08-21
---

# Phase 2 — session handoff

**Purpose.** Everything a new conversation needs to pick up Phase 2 without re-deriving it.
Read this, then [phase-2-plan.md](phase-2-plan.md) §2, then the work-package file for
whatever task is next.

**What this is not.** It does not repeat the plan. The plan and its six work-package files
are the instructions; this records *what happened*, *what was decided*, and *what will cost
you an hour if you guess*.

---

## 1. Where the project is

Phase 1 shipped: `v0.1.0`, auth · user · rbac · audit · admin, 39 E2E journeys green, five CI
workflows green. Phase 2 is the learning core — `content`, `lesson`, `learning`, `srs`,
`vocabulary`, plus the learner web app.

**5 of 30 Phase 2 tasks are done.**

| WP | Progress | Next |
|---|---|---|
| WP6 — design system & shell | P6.1 ✓ P6.2 ✓ **P6.3 in progress** | P6.4 |
| WP7 — content + lesson | P7.1 ✓ P7.2 ✓ | **P7.3** |
| WP8 — learning engine | — | blocked on P7.3 |
| WP9 — srs + vocabulary | — | blocked on P8.3 |
| WP10 — learner web | — | blocked on P6.5 + the contract tasks |
| WP11 — seed, E2E, ship | — | **P11.1 unblocks when P7.3 lands** |

Merged: PR #48 (P7.1), #49 (P6.1), #50 (P6.2), #51 (P7.2).
In flight: branch `feat/web-ui-primitives` (P6.3) — uncommitted working tree at the time of
writing.

Initial bundle: **167.2 kB gzipped** of a 200 kB budget.

---

## 2. How the work is run

Three agents, one task at a time:

```
Gravity   implements one task            → one branch, one PR
DeepSeek  reviews the diff               → reports findings, does not rewrite
Claude    reviews again, fixes, commits  → verifies against the real stack
```

Two tracks run in parallel — frontend (WP6 → WP10) and backend (WP7 → WP8 → WP9).
**Never hand an agent two tasks at once.** WP8 defines an interface WP9 implements and
Phase 3 implements five more times; an agent that sees two tasks designs for the one caller
in front of it, which is the exact failure ADR-0015 exists to prevent.

The prompt templates are in [phase-2/README.md](phase-2/README.md). Two lines have been
added to them by experience and must stay:

- backend prompts say **"and `make docs-check`"** — see §4.1
- backend prompts state **the next migration number** — see §4.2

---

## 3. Decisions taken in this conversation

These are not derivable from the code. They were decisions, and they are recorded in the
repository as well as here.

### 3.1 The UI/UX plan was re-phased, not adopted as written

A separate "Fluentra UI/UX Final Implementation Plan" was reviewed in full
([phase-2-ui-plan-review.md](phase-2-ui-plan-review.md)). Verdict: **7 of its 10 phases
belong in Phase 2, 2 belong in Phase 3, 1 is not a phase.**

| Moved | Where | Why |
|---|---|---|
| Speaking AI | **Phase 3** | needs `platform/ai`, `platform/media`, `speaking` — all Phase 3, all empty |
| XP, streaks, badges, achievements | **Phase 3** | `gamification` is a Phase 3 module and owns those tables |
| "Responsive + A11y + QA" phase | **dissolved** | became per-task Definition of Done; a terminal QA phase means nine screens built wrong then retrofitted |

Consequences that bind later tasks:

- **The dashboard is three cards**, not six: Continue Learning, Reviews Due, Skill Progress.
  `learning/AGENT.md` documents `GET /me/dashboard` as returning a `streak` — that field is
  Phase 3 and must not ship. Do **not** add a `streaks` table to `learn` to fill the gap.
- **The learning path is a unit list**, not a vertical node map. Deferred deliberately;
  revisit after the 20-learner alpha, with evidence.
- **Progress screen** carries study time, words mastered, per-skill mastery. No XP, no
  streak, no achievements tab, no chart library.

### 3.2 Contract-first, and mocks are generated

The plan's original wording ("typed mock adapter") was changed. A hand-written TypeScript
mock type is an invented DTO, and Phase 1 shipped two schema mismatches into the E2E suite
exactly that way (`TrustedDeviceList` is `{devices: []}`, not `{items: []}`).

```
api/openapi/openapi.yaml   →  pnpm gen:api  →  web/src/types/api.ts
                                                     ↓
                            MSW handlers typed as components["schemas"][...]
```

Each backend work package therefore opens with a **contract-only task** (P7.1, P8.1, P9.1)
that lands the OpenAPI paths with no implementation. That is what lets WP10 run beside WP8
and WP9 instead of behind them.

### 3.3 A signed-in learner reads published content; only admin writes

**The user's decision.** Implemented in migration `1700000180`:

- `user` role holds `content.read.published` — and nothing else
- `content.create`, `.edit`, `.review`, `.publish` are admin-only
- the five learner read endpoints require authentication (they were briefly specified as
  `security: []`, i.e. anonymous, which would have published the entire course to anyone
  with the URL and removed the surface Phase 4 attaches entitlements to)

This is the **first named permission the learner role has ever held**. 1700000020 said it
holds none and a test asserted it. That test is now
`TestAdminHoldsEverythingAndLearnerReadsOnly` and it checks **both directions** against a
declared one-item list, so a learner gaining a write permission cannot pass quietly —
it has to edit that list in a diff someone reads.
`TestLearnerCanReadPublishedContentButNotChangeIt` drives the guard itself.

### 3.4 Archiving is an item-level action

Settled by P7.3, recorded in `content/DECISIONS.md` (version stays readable by direct ID, hidden from discovery only; see P7.3 trap and integration test step 12).

`trg_content_versions_immutable` refuses **every** update to a published version — a status
change included — so `content_versions.status` can never become `archived`. Archiving sets
`content_items.status`.

**The consequence is the part that bites.** A published version stays `published` for ever,
so a learner query filtered only on `content_versions.status = 'published'` **returns
archived material**. Every learner-facing read joins `content_items` and filters there too;
`idx_content_items_status_kind` exists to serve exactly that. P7.5's trap says so.

### 3.5 The two blues, and now two borders

`--color-primary` (#2563eb) is the button **fill** and pairs with `--color-primary-fg`.
`--color-primary-accent` is anything drawn **on a surface** — links, active nav, inline
icons — and is per-mode (#2563eb light / #60a5fa dark).

They cannot be one token: #2563eb reads 5.17:1 on the light surface and **3.90:1** on the
dark one — enough for a focus ring, a WCAG failure for a link. Lightening it breaks the
button instead (white on a lighter blue drops to 2.5:1).

P6.2 applied the same shape to danger (`--color-danger` for text, `--color-danger-fill` for
the button) and raised `--color-border` to 4.76:1 so it clears WCAG 1.4.11's 3:1 for
controls. P6.3 is adding `--color-border-subtle` for decoration — a card edge is not a
control and 4.76:1 makes every card look outlined.

`contrast.test.ts` enforces all of this, including a component-level check that the rendered
class list actually references the verified token.

---

## 4. Traps found the hard way

Each of these cost real time in this conversation. They are the reason the prompt templates
say what they say.

### 4.1 `make check` does not run `docs-check`

`make check` = fmt, vet, lint, arch, test. The documentation drift gate is only in
`make ci-backend`. An agent can honestly report "make check green" while CI is red.

**Backend prompts must ask for `make docs-check` explicitly.**

### 4.2 Migrations are one global sequence, and goose refuses out-of-order

Not per-module despite the per-module directories. Highest applied is `1700000190`;
**the next is `1700000200`.**

P7.1 numbered its migration `1700000022`, below the database version, so it would never have
applied anywhere:

```
detected 1 missing (out-of-order) migration lower than database version (1700000170)
```

Only visible when you actually run `make migrate-up`.

### 4.3 `TODO.md` and `DECISIONS.md` are generated

Both live inside `<!-- BEGIN GENERATED: … -->` regions produced from
`tools/docgen/data/*.json`. Hand-editing them fails `make docs-check` and the next
`make docs` erases the edit.

The generator has **no notion of a done item** — every Phase 1 module still shows all its
work unticked. The convention the repo settled on is a hand-written `## Progress` section
**below** the generated block; see `audit/TODO.md`, `auth/TODO.md`, and now
`content/TODO.md`.

To record a decision, edit `tools/docgen/data/learning.json` (or `core.json`, …) and run
`make docs`. Keep the file's compact one-line-per-object formatting — reserialising it with
`json.dumps(indent=2)` rewrites 2,600 lines.

### 4.4 DB4 — one cross-schema foreign key, and nothing checks it

`DATABASE_GUIDELINE.md` DB4 / ADR-0004: the **only** permitted cross-schema foreign key is
`→ core.users(id)`. Phase 2 creates `content`, `learn` and `skill` side by side, so
`learn.*` pointing at `content.content_versions` is the shape to watch for.

`go-arch-lint` reads imports, **not migrations**. This is a review condition and nothing
more. A review round in this conversation introduced exactly this violation into a plan
document; it is now in the review checklist.

### 4.5 Tailwind v4 has no `tailwind.config.js`

Tokens are declared in CSS with `@theme`. A v3-style config file is ignored **silently**, and
the colours look right by accident because `slate` and `blue` are built-ins.

Dark mode is a class on `<html>`, set by an inline script in `index.html` before first paint.
Do not switch it to `prefers-color-scheme` — that cannot honour an explicit user choice.

### 4.6 The E2E suite needs one worker

`fullyParallel: true` locally, but Mailpit's inbox is shared across workers, so parallel runs
produce failures that look like regressions and are not. CI sets `workers: 1`.

**Reproduce CI with `pnpm exec playwright test --workers=1`.** Verified: 39/39 in 3.7 min.

---

## 5. This machine

| Thing | State |
|---|---|
| `.env` `DB_DSN` | points at **Supabase**, not the local container. `make migrate-up` writes there. |
| Local Postgres | `postgres://fluentra:fluentra@127.0.0.1:5432/fluentra` — what integration tests take as `TEST_DATABASE_URL` |
| `make dev` | container build is very slow here (>12 min, never finished). Use `make dev-infra` plus host-run `go run ./cmd/api` |
| Host-run API for E2E | needs the dev overlay's limits or every journey 429s: `OTP_ISSUE_PER_IP_PER_HOUR=500 RATE_LIMIT_AUTH_PER_MIN=500 RATE_LIMIT_ANON_PER_MIN=2000`, plus `SMTP_HOST=127.0.0.1 SMTP_PORT=1025` with blank credentials |
| `make check` | fails at `mailer` and `storage` with `ThreadSanitizer failed to allocate … (error code: 87)`. A Windows `-race` limitation, not a code failure — both pass under plain `go test` |
| Port 6379 | another project's `redis` container takes it. Stop it, never delete it |
| `pnpm` | **run installs from PowerShell, not Git Bash.** Git Bash creates POSIX symlinks (`/c/Users/…`) that Node on Windows cannot resolve |
| `pnpm store` | the `eslint-plugin-boundaries` entry is corrupt — its `index.js` is missing, so `pnpm lint` cannot run locally. CI installs fresh and is unaffected. Fix with `pnpm store prune` and reinstall |
| Prettier | something reformats unrelated `web/src` files during test runs. Whitespace only, but keep it out of feature commits |

---

## 6. Standing preferences

Stated by the user across this and the previous session:

- **Test directly on this machine.** It is a personal machine; run the real stack rather than
  simulating one or standing up a new service.
- **If a port is occupied, stop the container — never delete it.**
- Documentation stays in **English**, matching the rest of the repository. Conversation is in
  Vietnamese.
- Plans are split into small files, one per work package, so they can be handed to Gravity
  one at a time and reviewed by DeepSeek.
- Commit when asked. Push when asked. Not before.

---

## 7. Immediate next actions

1. **Finish P6.3.** Branch `feat/web-ui-primitives` has an uncommitted working tree: four
   new primitives (`card`, `badge`, `progress`, `skeleton`), `primitives.test.tsx`, a new
   `--color-border-subtle` token, and unrelated prettier churn across `features/admin` and
   `features/account` that should stay out of the commit. `web/package.json` and
   `pnpm-lock.yaml` are also modified — check whether a dependency was added, and price it
   against the 33 kB of remaining bundle budget.
2. **P7.3** — `feat/content-module`. The largest task in WP7. Both of its traps matter: the
   half of the archive question P7.2 left open, and `media_refs` carrying no foreign key so
   publish is the only thing that can check it.
3. **P11.1 — content authoring — unblocks the moment P7.3 lands.** One course, eight lessons,
   200 word senses at A2–B1. `ROADMAP.md` calls content production "**the real bottleneck**;
   staff it early", and it is the item most likely to become the critical path to the alpha.
   It appears last in the work-package list and must not be started last.

---

## 8. Map of the plan documents

| File | What it is |
|---|---|
| [phase-2-plan.md](phase-2-plan.md) | master: work packages, dependency graph, Definition of Done, verification gates |
| [phase-2-ui-plan-review.md](phase-2-ui-plan-review.md) | the UI/UX plan review — scope, architecture, API readiness, UX, risks, edge cases |
| [phase-2/README.md](phase-2/README.md) | the handoff loop and the prompt templates |
| [phase-2/REVIEW-CHECKLIST.md](phase-2/REVIEW-CHECKLIST.md) | what the reviewing agent checks |
| [phase-2/WP6-design-system.md](phase-2/WP6-design-system.md) … [WP11](phase-2/WP11-seed-e2e-ship.md) | six work packages, one file each |
| [phase-1-plan.md](phase-1-plan.md) | the previous phase, delivered |
| [/ROADMAP.md](../../ROADMAP.md) | phases 0–5 |
