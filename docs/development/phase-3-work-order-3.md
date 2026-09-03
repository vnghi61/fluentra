---
doc_type: handoff
phase: 3
status: in_progress
last_verified: 2026-09-02
---

# Phase 3 — work order 3

**Purpose.** The work after `7dc249b`, with four decisions now taken by the product owner.
It replaces §2 of [phase-3-work-order-2.md](phase-3-work-order-2.md).

**Read first.** [phase-3-plan.md](phase-3-plan.md) is the specification.
[phase-3-next-steps.md](phase-3-next-steps.md) §4 holds the traps — the generated-code gates,
the migration numbering, the stale TODO checkboxes — and they are all still current.

---

## 1. Decisions taken

| Question | Decision |
|---|---|
| What comes next | WP18 — quota, warnings and the queue, before WP16/WP17 |
| The XP rule | **Change the code** to match `DECISIONS.md`, not the other way round |
| Avatars on the leaderboard | **Yes** — add `avatar_url` to `LeaderboardEntry` |
| How quota is modelled | **Per provider, aggregated** — not per task alone |

The reasoning behind the first one is worth carrying: WP16 lets a learner add any word, and
every word the dictionary does not already have is an AI call. Shipping that before a ceiling
exists means the first enthusiastic user exhausts a free provider's quota and the feature
fails midway through, for everyone.

---

## 2. The work, in order

### 1. WP15's two leftovers — small, and they block correct quota grouping

**Expired cache rows are never deleted.** `ai.ai_cache_entries` has an index on `expires_at`
and nothing reads it. Four retention jobs already exist to copy the shape from:
`prune_published_outbox_events`, `audit.enforce_retention`, `learning.refresh_retention`,
`user.export_retention_cleanup`.

**A cache hit is recorded with `Provider: "cache"`.** The `status` column already says
`cached`, so the provider field invents a provider that does not exist — and puts it in the
exact column step 2 groups by. Record the provider that produced the cached answer, or leave
it empty.

**Done means:** expired rows are removed on a schedule, and `SELECT ... GROUP BY provider FROM
ai.ai_usage` returns only real providers.

### 2. WP18 — quota, warnings and the queue

The main piece of work.

**The table needs a provider dimension.** `ai.ai_budgets` today is:

```
task, daily_request_limit, daily_token_limit, is_active
```

keyed uniquely by `task` and modelling no providers at all. The decision is aggregate quota
across free providers, so a budget has to know which provider it belongs to before "everything
is exhausted" can mean anything. Take migration number `1700000410` or above — `1700000300`,
`1700000310` and `1700000400` are used.

**What "exhausted" means is the whole feature.** Warn the learner only when *every* provider is
out, not on the first refusal — that was explicit in the original request. Until then a
refusal by one provider is the router's business and the learner should never see it.

**The queue must leave the learner something to do.** §1 of the plan promised that a learner
whose word is queued can still review it as an ordinary flashcard. A queue that only says
"come back later" does not satisfy this.

**Decide which way a broken quota check fails, and write it down.** A check that errors must
not silently allow — that spends money nobody authorised — and must not silently deny either,
which strands a learner for a reason they cannot see. Pick one, record it in
`internal/modules/gamification/DECISIONS.md`'s sibling for the ai platform, and write the test
for that path specifically.

**Done means:** an exhausted budget queues rather than fails; queued words are reviewable as
flashcards; the learner is told only when every provider is exhausted; the admin sees it
first; and `ai_usage` grouped by provider is what the enforcement reads.

### 3. Make the XP code match the recorded rule

`DECISIONS.md` already specifies this exactly, and the code does something else.

The recorded rule, for **a graded item**:

> Its best score ever, divided by ten, granted as the increase over what it has already
> granted. 80/100 grants 8. Retaking to 100/100 grants 2, not 10. A third attempt grants
> nothing. The award is never negative: a worse retake takes nothing back.

What the code does: `domain.BaseAward` returns a flat 10 for `SourceActivity`, and
`learn.xp_events` carries `UNIQUE (user_id, source, source_id)` with an `ON CONFLICT DO
NOTHING` insert — so a retake awards nothing at all, whatever the score.

Three things follow, and the third is the one that bites:

- **Only `SourceActivity` is a graded item.** Lesson completion (40), review session (25) and
  upload verified (5) have no score and keep their flat awards. A change that sweeps all of
  `BaseAward` into a score formula breaks three sources that were never in question.
- **The high-water mark needs its own storage**, and `DECISIONS.md` says why: `UpsertProgress`
  sets `score = EXCLUDED.score`, so `learn.progress` holds the *most recent* score, not the
  best one. A bad retake would lower it, and XP must never fall.
- **The idempotency constraint is currently doing the anti-farm work**, and the new rule takes
  that job over. Granting an increase means more than one event per `(user, source,
  source_id)`, which that unique key forbids. Changing it needs care: it is what stops a
  redelivered event awarding twice, so whatever replaces it has to keep that property.

**Done means:** 80/100 grants 8; retaking to 100 grants 2; a third attempt grants 0; a worse
retake grants 0 and takes nothing back; a redelivered event still awards nothing twice; and
the three unscored sources are untouched.

### 4. `avatar_url` on the leaderboard

Smaller than it looks. `gamification/service` already reads `usercontract.Reader` to resolve
display names, and `usercontract.Summary` already carries `AvatarURL *string`. The field is
being fetched and dropped.

- Add `avatar_url` to `LeaderboardEntry` in `api/openapi/components/gamification.yaml`, and
  carry it through the DTO.
- **Regenerate both sides.** `make gen-api` *and* `make gen-web`, then `make gen-check` *and*
  `make gen-check-web`, and commit the result. This exact step cost two CI rounds last week.
- Render it at `?size=sm` — 64 px. A forty-row board at `lg` is forty 256 px images through
  the API on a free dyno.

**This reverses a recorded decision, so record the reversal.** `DECISIONS.md` currently says
of a league board: *"Display name is the only name a learner chooses and the only one already
meant to be seen; email and profile are never on a board."* An avatar is profile. Update that
row to say what is now shown and why, rather than leaving the document contradicting the code
— which is precisely the state the XP rule is in today, and the reason item 3 exists.

### 5. WP16, then WP17

Learner-contributed words, then answer explanations. Both are unblocked, and both are what the
project is actually for. They come after the ceiling exists.

---

## 3. Still on the human

- **Production migrations.** A manual step, several pending. `.env` points `DB_DSN` at the
  production Supabase pooler, so any migrate or seed run without an explicit `DB_DSN` targets
  production.
- **The badge catalogue** now reaches production through
  `1700000300_seed_gamification_catalogues.sql`, and `cmd/seed/gamification_migration_test.go`
  keeps the two copies honest.

---

## 4. How the last three rounds were reviewed

Two checks caught everything that was found:

**Ask the database, not the code.** Raw SQL that no test executes is untested however green
the suite looks. `DBCache` and `DBUsageRecorder` had thorough unit tests against fakes and not
one line run against a real schema; a misspelled column there compiles, passes, and fails at
runtime on a path whose errors are deliberately swallowed — so the symptom is not an error but
a cache that quietly never fills.

**A silent failure is worse than a loud one.** Three separate paths threw their errors away
last round. Not propagating the error was right; discarding it was not. When a failure must
not reach the caller, log it. The fault that reports nothing is the one nobody fixes — and on
this project it is usually the one that spends money.
