---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-01
---

# Phase 3 — session handoff

**Purpose.** What the next agent needs to start Phase 3 without re-deriving it, and what
production needs before any of it is visible.

**What this is not.** It does not repeat [phase-3-plan.md](phase-3-plan.md). The plan is the
instructions; this records *what is actually built*, *what is deployed*, and *what will cost
you an hour if you guess*.

---

## 1. Where the project is

`main` is green at `ea94faa` (PR #68). Three workflows ran and passed: `build`, `security`,
`ci-frontend`. `docs` and `ci-backend` did not run — their `paths` filters exclude a commit
that touches only `web/**`. That is correct behaviour, not a skipped gate.

The gap that matters is between **merged** and **deployed**, and between **backend done** and
**visible to a learner**.

| WP | Backend | Web | Reality |
|---|---|---|---|
| WP12 — seed beyond vocabulary | — | — | not started |
| WP13 — dictionary autocomplete | — | — | not started |
| WP14 — gamification | done | **missing** | see below |
| WP15 — `platform/ai` | — | — | not started, gates WP16-WP21 |
| WP16 — learner words | tables only | — | blocked on WP15 |
| WP17 — explanations | — | — | blocked on WP15 |
| WP18 — quota and queue | — | — | blocked on WP15 |
| WP19 — admin | — | — | blocked on WP16 and WP18 |
| WP20 / WP21 — four skills | — | — | blocked on WP15 |

### WP14 is half a feature

The backend is complete and wired. The module is mounted in
[`cmd/api/modules.go`](../../cmd/api/modules.go), four routes exist
(`/me/gamification`, `/me/streak`, `/me/streak/freeze`, `/leaderboard`), all four are in
`api/openapi/openapi.yaml`, and the generated TypeScript client already carries their types.

`web/src/features/` contains `account`, `admin`, `auth`, `learning`, `lesson`, `review`,
`vocabulary`. There is **no `gamification` directory**. The last unchecked line in
[the module TODO](../../internal/modules/gamification/TODO.md) is "Gamification widgets in
the web app".

That is the whole answer to "why does the dashboard have no XP bar and no leaderboard". It
was built and never surfaced.

---

## 2. Production is behind `main`

There is **no auto-migration on boot**. `cmd/api` does not call goose. Migrations are a
deliberate manual step.

### Four migrations are pending

| Version | Adds |
|---|---|
| `1700000260` | gamification tables — xp_events, streaks, badges, quests, leaderboards |
| `1700000270` | vocabulary upload tables |
| `1700000280` | `learn.courses.origin` |
| `1700000290` | `learn.review_cards.last_review_at` |

The last two are **read by code already on `main`**. The catalogue query filters
`AND origin = 'curriculum'`, and the FSRS scheduler reads `last_review_at`. Deploying the
code before the migration is a "column does not exist" outage.

```bash
DB_DSN="<the production DSN from Render>" go run ./cmd/migrate up
```

### Do not seed production

`cmd/seed` refuses by name:

```text
refusing to seed: APP_ENV is production
```

That guard is deliberate. Seeding writes demo accounts whose password is printed in a public
guide.

### But the badge catalogue has no way in

This is an open defect, found while verifying the above and **not yet fixed**.

The badge and quest catalogue exists only in
[`cmd/seed/gamification.go`](../../cmd/seed/gamification.go), which sits behind the
production guard. Its own comment states the case against that placement:

> It touches no learner state: XP, streaks and earned badges belong to learners and are never
> seeded.

It is reference data, not demo data. Production needs it. After migrating, `learn.badges` and
`learn.quests` will be **empty**, so WP14's web widgets would ship with no badge to award.

Needs a path that does not depend on `APP_ENV`: either the catalogue moves into a migration,
or a catalogue-only command is split out of the seeder.

### `S3_REGION` must be `auto`

Cloudflare R2 accepts seven values in a credential scope: `auto`, `wnam`, `enam`, `weur`,
`eeur`, `apac`, `oc`. `ap-southeast-1` is an AWS region name and R2 answers
`400 InvalidRegionName`.

`ValidateRegion` in `internal/platform/storage/region.go` now refuses to boot on a bad value,
in both `cmd/api` and `cmd/worker`. The failure it replaces was invisible: the region appears
only in the credential scope, nothing reads it until the browser spends the presigned URL,
and R2 checks signature and CORS *before* region — so it was the error that surfaced only
after everything else had been fixed.

The other buckets need no CORS policy. Only avatars is written from the browser. See
`deploy/r2/README.md`.

---

## 3. What to do next, in order

Ordered so that nothing waits on something that has not started.

### 1. WP14-web — gamification widgets

Not blocked by anything. Backend, spec and generated types are all in place, so this is
assembly, not design.

**Done means:** `web/src/features/gamification` exists; the dashboard shows an XP bar with
level, a streak indicator, and a leaderboard; the last box in the gamification `TODO.md` is
ticked; tests cover the empty state, because a new learner has no XP and no rank.

### 2. Badge catalogue into production

**Done means:** a fresh production database ends up with the ten badges and four quests
without `APP_ENV` being lied about, and re-running it is idempotent.

### 3. WP12 and WP13, in parallel

Seed expansion into gap-fill and exam-shaped items; dictionary autocomplete on the
add-a-word field. Neither needs AI, a provider, or money.

### 4. WP15 — `platform/ai`

The gate for WP16, WP17, WP18, WP20 and WP21, and the largest single item in the plan.
Start it alongside 1-3 rather than after them.

**Do not hand out WP16, WP17 or WP18 yet.** The plan marks them `Needs: WP15`. Started early,
they grow their own provider interface and get rewritten.

---

## 4. Traps found the hard way

### `.env` points at production

`.env` sets `DB_DSN` to the production Supabase pooler. Any `go run ./cmd/migrate` or
`./cmd/seed` invoked **without** an explicit `DB_DSN` targets production.

Precedence is defaults, then `.env`, then the YAML file, then environment variables — so an
exported `DB_DSN` does win. Verified rather than assumed, by pointing it at a closed port and
watching the connection error name that port.

### Migrations run as a different role

`cmd/migrate` assumes `fluentra_migrator` before applying anything; the application role stays
DDL-free. A privilege check run as `fluentra` therefore proves nothing about whether a
migration can run.

A local database created before that role split has tables owned by `fluentra`, and
migrations fail with `permission denied for table content_versions`. CI never sees it because
CI always starts empty. The fix is a grant to `fluentra_migrator` on the existing schemas.

### CI gates that `go build` and `go test` do not cover

`golangci-lint run`, the same with `--build-tags=integration`, `make gen-check`, `make arch`,
Spectral over `openapi.yaml`, markdownlint, docgen drift, `pnpm run typecheck` (which is
`tsc -b`, stricter than `--noEmit`), the bundle budget, and Playwright across five device
projects.

ESLint in `web/` is not optional either: `react-hooks/set-state-in-effect` rejects a
`setState` call in an effect body, and it was right to — the derivation it forced is better
than the state it refused.

### A green suite is not a correct one

`ci-frontend` failed on `main` against a tree **identical** to the branch PR that had passed
six minutes earlier. The trace showed the `startAttempt` POST beginning at `140220.854` and
taking 81 ms, and the Check Answer click running at `140264.7` — inside that window. The
answer was dropped by a bare `return`, so the screen after the click was identical to the
screen before it.

Four device projects won that race and one lost it. The lesson is that a race lost 81 ms at a
time passes on the machine that has the bug; hold the window open in a test instead of racing
it.

---

## 5. Reproducing the live rig

This is what caught three defects that the test suite did not. It is worth the ten minutes.

```bash
make dev-infra
DB_DSN="postgres://fluentra:fluentra@127.0.0.1:5432/fluentra?sslmode=disable" go run ./cmd/migrate up
DB_DSN="postgres://fluentra:fluentra@127.0.0.1:5432/fluentra?sslmode=disable" go run ./cmd/seed
go build -o ./bin/api ./cmd/api && go build -o ./bin/worker ./cmd/worker
```

Run the API on 8080 and the worker on 8081 with the environment block from
`.github/workflows/ci-frontend.yml`, then:

```bash
cd web && pnpm exec playwright test --workers=2
```

**Use `--workers=2`.** At the default three, a developer machine loses journeys to contention
and the failures look like regressions. The same suite that failed eight tests at three
workers passed twelve of twelve at two.

**Always check the worker actually booted.** It passes no `Guard`, so a panic there is silent
from the API's point of view — and a worker that will not start drains no outbox, sends no
OTP mail, and times out every E2E journey for a reason that looks nothing like the cause.

---

## 6. Decisions still open

**The XP rule contradicts itself.** The code awards a flat 10 XP per activity, idempotent on
activity id. `internal/modules/gamification/DECISIONS.md` records best-score-divided-by-ten,
granted as the increase. One of the two has to change, and it is a product decision rather
than a bug.

**OmniRoute.** Raised as a candidate for WP15 provider routing. Not evaluated. Note that it
has to run inside the existing Render service — a separate process is not free on that plan.
