---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-02
---

# Phase 3 — what to build next

**Purpose.** The work order after `feat/gamification-web-and-ai-platform`. It supersedes the
"What to do next" section of [HANDOFF-PHASE3.md](HANDOFF-PHASE3.md), which was written before
WP13 and WP14-web existed.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification and has not changed.
This says what is now true, what is next, and what will waste your time if you guess.

---

## 1. What is actually built

Branch `feat/gamification-web-and-ai-platform`, commit `67d30cc`. Every gate green: `go test
./...`, golangci-lint, `make arch`, `make gen-check`, Spectral, docgen, markdownlint, `tsc -b`,
ESLint, 256 web tests, bundle 193.6 kB of 200 kB.

| WP | State |
|---|---|
| WP12 — seed beyond vocabulary | not started |
| WP13 — dictionary autocomplete | done |
| WP14 — gamification | backend done, web done |
| WP15 — `platform/ai` | built but **not connected** — see below |
| WP16 — learner words | tables only, blocked on WP15 |
| WP17 — explanations | blocked on WP15 |
| WP18 — quota, warnings, queue | `ai.ai_budgets` exists, no code |
| WP19 — admin | blocked on WP16 and WP18 |
| WP20 / WP21 — four skills | blocked on WP15 |

### WP15 is wired to nothing

`Router`, `ProviderRegistry`, `MemoryCache` and `DBUsageRecorder` all exist and all compile.
Nothing calls them. `cmd/worker` still builds its AI client the old way:

```go
aiClient, err := ai.New(ai.Config{ Provider: cfg.AI.Provider, ... })
```

So today: no request is routed, no fallback ever happens, `ai.ai_requests` and `ai.ai_usage`
stay empty, and `ai.ai_cache_entries` has no writer at all. The only cache is
`MemoryCache`, which lives in the process — and on a free Render dyno the process sleeps.
Every wake starts with an empty cache, which is exactly the cost the plan's cache-first rule
exists to avoid.

This is the next task and it is the cheapest large win available.

---

## 2. The work, in order

### 1. Connect WP15 to its one caller

`cmd/worker`'s upload verification is the only AI caller that exists. Route it through
`Router`, record every attempt, and back the cache with the table that is already there.

**Done means:** one verification produces a row in `ai.ai_requests` and updates
`ai.ai_usage`; the same verification run twice makes one provider call; and a worker restart
does not lose the cache, because it is read from `ai.ai_cache_entries` rather than a map.

Keep the existing failure behaviour. The comment at that call site is load-bearing: a client
it cannot build is logged and dropped rather than fatal, because verification degrades to the
dictionary alone and the dictionary still answers the question that matters most.

### 2. WP12 — seed beyond vocabulary

Gap-fill and exam-shaped items. No AI, no provider, no money, no dependency on anything above.
Take this in parallel if two things can run at once.

### 3. WP18 — quota, warnings, the queue

`ai.ai_budgets` is already migrated. With step 1 done there is real usage data to enforce
against, which is the right order: a budget with nothing recording spend cannot be tested.

**Done means:** an exhausted budget queues the request rather than failing it, the learner is
told only when every provider is exhausted, and the admin sees it before the learner does.

### 4. WP16 and WP17

Learner-contributed words, then answer explanations. Both need step 1 and step 3 first —
WP16's whole cost argument is that a word already in the dictionary makes no AI call, and that
argument cannot be verified until requests are recorded.

### 5. WP19, then WP20 / WP21

Admin, then the four skills.

---

## 3. Two decisions that are not code

**The XP rule contradicts itself.** The code awards a flat 10 XP per activity, idempotent on
activity id. `internal/modules/gamification/DECISIONS.md` records best-score-divided-by-ten,
granted as the increase. The dashboard now displays XP, so the two disagree in public. One has
to change and it is a product call, not a bug.

**Whether the leaderboard shows faces.** `LeaderboardEntry` exposes `rank`, `user_id`,
`display_name`, `xp`, `is_self`, and its schema says *"No other identifier is exposed"* — a
deliberate privacy position. `GET /api/v1/storage/avatars/{assetId}` is readable by any
signed-in learner and could serve a leaderboard directly. Adding `avatar_url` to the entry is
a privacy decision, so it needs a person, not a patch.

---

## 4. Traps, all found the hard way

### The TODO checkboxes are stale and generated

Every box in `internal/modules/gamification/TODO.md` is unticked, including "XP events with
idempotency and daily caps", which has been built and shipped. Repo-wide only eight boxes are
ticked. They are generated from `tools/docgen/data/*.json`, so hand-editing `TODO.md` is undone
by the next `make docs`.

**Do not read an unticked box as unbuilt work.** Read the code. When you finish something,
tick it in the docgen data file and regenerate.

### Adding an OpenAPI path is two steps

`go build` and `go test` both stay green while `make gen-check` fails, because the generated
server and client are only refreshed by `make gen-api`. This was missed once already this
week: a route was added to `openapi.yaml`, everything compiled, and CI caught the stale
generated surface.

### Migration numbers are taken up to 1700000400

`1700000300` (gamification catalogue), `1700000310` (ai tables) and `1700000400` (avatar
assets) are used. Take `1700000410` and upward. Goose applies in lexical order across all
module directories, flattened by `//go:embed all:*`, so a new directory needs no wiring but a
duplicate number breaks the run.

### The badge catalogue exists twice, on purpose

A migration cannot call Go and production needs SQL, so the catalogue is in both
`db/migrations/gamification/1700000300_seed_gamification_catalogues.sql` and
`cmd/seed/gamification.go`. `cmd/seed/gamification_migration_test.go` holds them to each other.
If you add a badge, add it to both — the test will tell you if you forget, and it fails on the
case that matters: a criteria kind the evaluator does not understand is a badge nobody can ever
earn, and it looks perfectly healthy in the database.

### A frontend path that no route serves

This repository's recurring failure. `GET /api/v1/storage/avatars/{id}` was advertised by the
API for months with nothing mounted there, and the test that should have caught it asserted
`strings.HasPrefix` on the URL rather than fetching it. When you add a client call, check the
path against **both** `openapi.yaml` and the chi router, and write the test that asks for the
response rather than the shape of the string.

### Running migrations locally on a reused cluster

`cmd/migrate` assumes `fluentra_migrator` before applying anything. That role is cluster-wide,
so on a *brand-new* database in a cluster that already has it, the bootstrap migration runs as
the non-superuser role and cannot create extensions or grant roles. Apply to an
already-migrated database instead — which is also what production does.

### Playwright on a developer machine

Use `--workers=2`. At the default three, journeys fail on contention and the failures read like
regressions: the same suite that failed eight tests at three workers passed twelve of twelve at
two.

---

## 5. Production is still behind

Unchanged from [HANDOFF-PHASE3.md](HANDOFF-PHASE3.md) §2 and still not done:

- `S3_REGION` must be `auto`. The boot guard now refuses to start on `ap-southeast-1`, so the
  API will not come up until this is changed on Render.
- Migrations are a manual step; there is no auto-migration on boot. Five are now pending, and
  three of them are read by code already merged.
- `.env` points `DB_DSN` at the production Supabase pooler, so any migrate or seed command run
  without an explicit `DB_DSN` targets production.
