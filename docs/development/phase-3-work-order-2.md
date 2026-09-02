---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-02
---

# Phase 3 — work order 2

**Purpose.** What to build after `34ee4c9`. It replaces §2 of
[phase-3-next-steps.md](phase-3-next-steps.md); §3 (open decisions) and §4 (traps) there are
still current and still worth reading.

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification and has not changed.

---

## 1. Where this leaves off

| WP | State |
|---|---|
| WP12 — seed beyond vocabulary | not started |
| WP13 — dictionary autocomplete | done |
| WP14 — gamification | backend done, web done |
| WP15 — `platform/ai` | connected and recording; two gaps below |
| WP16 — learner words | tables only, unblocked |
| WP17 — explanations | unblocked |
| WP18 — quota, warnings, queue | `ai.ai_budgets` migrated, no code |
| WP19 — admin | needs WP16 and WP18 |
| WP20 / WP21 — four skills | needs WP18 |

WP15 works end to end. `ai.New` returns a `Router`; `cmd/worker` passes a `Pool` and gets
persistent caching and usage recording from it. Verified against a real database, not read:
one call leaves a cache row, a second client built the way a restarted worker builds one is
served from that row, and `ai_requests` carries one `success` and one `cached`.

---

## 2. The work, in order

### 1. Finish WP15's two loose ends

Small, and both get worse the longer the table is in use.

**Expired cache rows are never deleted.** `ai_cache_entries` has an index on `expires_at` and
nothing that reads it. The repository already runs four retention jobs —
`prune_published_outbox_events`, `audit.enforce_retention`, `learning.refresh_retention`,
`user.export_retention_cleanup` — so there is a shape to copy rather than invent.

**A cache hit is recorded with `Provider: "cache"`.** The `status` column already says
`cached`, so the provider field is both redundant and misleading: it invents a provider that
does not exist and puts it in the same column WP18 is about to group by. Record the provider
that produced the cached answer, or leave it empty; either reads better than a fifth
"provider" appearing in the cost report.

**Done means:** expired rows are removed on a schedule, and grouping `ai_usage` by provider
returns only real providers.

### 2. WP18 — quota, warnings, the queue

The next real feature, and the reason step 1 comes first: budgets are enforced against
`ai_usage`, so the rows have to be trustworthy before anything reads them.

`ai.ai_budgets` is migrated and unused. It carries `task`, `daily_request_limit`,
`daily_token_limit`, `is_active` — per task, not per provider, which is worth noticing before
writing the enforcement: the plan's warning model is about aggregate quota across free
providers, and this table does not model providers at all. Either extend it or decide
deliberately that the budget is per task and providers are the router's business.

**Done means:** an exhausted budget queues the request rather than failing it; the learner can
still review queued words as flashcards, which §1 of the plan promised them; the learner is
told only when *every* provider is exhausted, not on the first refusal; and the admin sees it
before the learner does.

**Watch the failure direction.** A quota check that errors must not silently allow — that
spends money — and must not silently deny either, which strands the learner. Decide which way
it fails, write it in `DECISIONS.md`, and test that path specifically.

### 3. WP12 — seed beyond vocabulary

Gap-fill and exam-shaped items. No AI, no provider, no money, no dependency on anything above.
Take it in parallel if two things can run at once.

### 4. WP16, then WP17

Learner-contributed words, then answer explanations. Both are unblocked now, but WP16's whole
cost argument — a word already in the dictionary makes no AI call — is only demonstrable once
step 2 can show the spend. Do them after WP18 unless something forces the order.

### 5. WP19, then WP20 / WP21

Admin, then the four skills.

---

## 3. How the last two rounds were reviewed

Not as bureaucracy — these are the two checks that actually caught things.

**Ask the database, not the code.** `DBCache` and `DBUsageRecorder` write raw SQL, and no test
touched a real schema until `internal/platform/ai/integration_test.go`. A misspelled column
there compiles, passes every unit test, and fails at runtime on a path whose errors are
deliberately swallowed — so the symptom is not an error but a cache that quietly never fills
and a bill that grows. Any new raw SQL needs a test that runs it.

**A silent failure is worse than a loud one.** Three separate paths this round threw their
errors away. Not returning the error was right; discarding it was not. When a failure must not
propagate, log it — the fault that reports nothing is the one nobody fixes.

---

## 4. Still on the human, not on gravity

- **The XP rule contradicts itself.** Code awards a flat 10 XP per activity;
  `internal/modules/gamification/DECISIONS.md` records best-score-divided-by-ten. The dashboard
  now shows XP, so the two disagree in public.
- **Whether the leaderboard shows faces.** `LeaderboardEntry` says *"No other identifier is
  exposed"* — a deliberate privacy position. `GET /api/v1/storage/avatars/{assetId}` could
  serve one. Adding `avatar_url` is a privacy decision.
- **Production.** Migrations are a manual step and several are pending; `.env` points `DB_DSN`
  at the production Supabase pooler, so any migrate or seed run without an explicit `DB_DSN`
  targets production.
