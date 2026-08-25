---
module: srs
tier: learning
group: modules
status: ACTIVE
phase: 2
owner: "@learning-team"
schema: learn
tables: [review_cards, review_logs, srs_params, review_daily_stats]
depends_on: [cache, job, content, user]
depended_on_by: [learning, vocabulary, grammar, gamification, notification, analytics]
spec_version: 1.0.0
last_verified: 2026-08-25
---

# srs — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/srs` |
| Schema | `learn` |
| Delivery phase | 2 |
| Status | **ACTIVE** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Spaced repetition. Owns review cards, the FSRS scheduler, review logs, due queues and retention forecasting. Any material worth remembering long-term enters this module and comes back at the right time.
<!-- END GENERATED: overview -->

**Context.** FSRS (Free Spaced Repetition Scheduler) replaces SM-2: it models memory stability and difficulty explicitly and typically reaches the same retention with 20–30 % fewer reviews (ADR-0016). The algorithm is documented in `docs/knowledge/fsrs.md` — read it before touching the scheduler.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Review cards: one per (learner, learnable item)
- FSRS scheduling: stability, difficulty, retrievability, next due date
- Review logs: every grade, for scheduler improvement and analytics
- Due queue construction with daily limits and interleaving
- Suspend, bury and reset operations
- Retention forecasting and workload projection
- Per-learner parameter optimisation (later phase)

**This module does NOT own:**

- Deciding what is worth remembering — graders return `ReviewItems`
- Rendering a review — the skill module supplies the renderer
- XP and streaks — that is `gamification`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/srs/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/srs/contract/` | You are calling this module from another module |
| `internal/modules/srs/service/` | You are changing behaviour |
| `db/migrations/srs/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/srs/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `srs.CardWriter` | `UpsertCards(ctx, userID, items)` after grading, and `SetCardsSuspended(ctx, userID, contentVersionIDs, suspended)` when a skill module learns the content is already known |
| interface | `srs.QueueReader` | `DueCount`, `DueCards` — used by the dashboard and by `gamification` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `review.card_answered` | publishes | `{user_id, card_id, grade, interval_days}` |
| `review.session_completed` | publishes | `{user_id, reviewed, correct, minutes}` |
| `review.due_soon` | publishes | `{user_id, due_count}` — emitted by a scheduled job for reminders |
| `content.archived` | consumes | Suspend cards whose content is gone |
| `user.deleted` | consumes | Delete cards and logs |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `learn` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/srs/` · Queries: `db/queries/srs/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `learn.review_cards` | Scheduling state per learner per item | `user_id`, `content_version_id`, `skill`, `stability`, `difficulty`, `due_at`, `reps`, `lapses`, `state`, `suspended_at`. Unique on (user_id, content_version_id) |
| `learn.review_logs` | Every review answered | Partitioned monthly. `card_id`, `grade`, `elapsed_ms`, `stability_before/after`, `scheduled_days`, `reviewed_at` |
| `learn.srs_params` | FSRS weights | Global defaults; per-user overrides once enough review history exists |
| `learn.review_daily_stats` | Per-day counters | Drives limits, forecasting and the heat map |

**Indexes of note**

- `idx_review_cards_user_due` — partial `WHERE suspended_at IS NULL`; **the hottest query in the product**
- `idx_review_logs_card_time` — history and scheduler analysis
<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `srs`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/reviews/session` | `self` | Build a review session from due cards |
| `GET` | `/api/v1/reviews/due-count` | `self` | Badge count |
| `POST` | `/api/v1/reviews/{card_id}/answer` | `self` | Record a grade and reschedule |
| `POST` | `/api/v1/reviews/session/complete` | `self` | Close the session |
| `POST` | `/api/v1/reviews/{card_id}/suspend` | `self` | Stop scheduling this card |
| `POST` | `/api/v1/reviews/{card_id}/reset` | `self` | Treat as new again |
| `GET` | `/api/v1/reviews/forecast` | `self` | Projected workload for the next 30 days |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`cache`](../../platform/cache/AGENT.md) | → depends on | Due-count caching |
| [`job`](../../platform/job/AGENT.md) | → depends on | Daily partition rotation and stats aggregation |
| [`content`](../../modules/content/AGENT.md) | → depends on | Word sense and lesson content versions |
| [`user`](../../modules/user/AGENT.md) | → depends on | Learner timezone lookup for local midnight due queue boundaries |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
| [`vocabulary`](../../modules/vocabulary/AGENT.md) | ← used by | consumes this module's contract |
| [`grammar`](../../modules/grammar/AGENT.md) | ← used by | consumes this module's contract |
| [`gamification`](../../modules/gamification/AGENT.md) | ← used by | consumes this module's contract |
| [`notification`](../../modules/notification/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-SRS-01** — Scheduling is FSRS, implemented in `domain/` as pure functions with no I/O — it must be testable and provable without a database.
2. **BR-SRS-02** — Grades are exactly four: `again`, `hard`, `good`, `easy`. Do not add a fifth without reading the algorithm's assumptions.
3. **BR-SRS-03** — A card's next due date depends only on its state, the grade, the elapsed time and the parameters — never on the time of day or the session.
4. **BR-SRS-04** — Reviews answered before they are due update stability but do not shorten the schedule punitively; early review is allowed and modelled.
5. **BR-SRS-05** — Daily new-card and review limits are per learner and enforced when building the session, not by refusing an answer mid-session.
6. **BR-SRS-06** — A session interleaves skills and avoids presenting two cards from the same word family consecutively.
7. **BR-SRS-07** — A lapse (grading `again` on a mature card) reduces stability according to the algorithm — it does not reset to zero.
8. **BR-SRS-08** — Suspended cards never appear in a queue and are excluded from forecasts.
9. **BR-SRS-09** — Every answer writes a review log; the log is the raw material for future parameter optimisation and must never be pruned before the retention window.
10. **BR-SRS-10** — The scheduler is versioned: a parameter change records the version on each log so past scheduling remains explainable.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Tune scheduler parameters

1. Read `docs/knowledge/fsrs.md` first — the parameters are not independent knobs.
2. Change the values in `srs_params`, never in code.
3. Bump the scheduler version so past logs remain interpretable.
4. Simulate against historical review logs before shipping; the simulation harness lives in `test/fixtures/srs/`.
5. Roll out behind a feature flag and compare retention over at least four weeks — SRS effects are slow and noisy.

### Make new material reviewable

1. Have the skill's grader return `ReviewItems` from `Grade`.
2. Confirm the content version is stable and published — cards reference versions.
3. Add the renderer for that card type in the review player.
4. Test the full path: attempt → card created → due tomorrow → answered → rescheduled.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Parameters are global in Phase 2; per-learner optimisation needs several hundred reviews per learner and arrives later.
- There is no cross-device offline review queue.
- Interleaving is heuristic rather than optimised.
- The forecast assumes the learner reviews everything on time, which is optimistic.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
- All scheduling logic lives in `domain/fsrs.go` as pure functions. If a scheduling decision needs a database read, the design is wrong.
- Property-based tests (`rapid`) assert the algorithm's invariants: intervals are monotonic in grade, stability never goes negative, a lapse never increases the interval.
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:srs:due_count:{user_id}:v1` | 60 s | Any answer |
| `fluentra:{env}:srs:forecast:{user_id}:v1` | 1 h | Nightly rebuild job |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `REVIEW_CARD_SUSPENDED` | 409 | Card is suspended |
| `REVIEW_NOT_DUE` | 409 | Answered a card outside the session, before it was due |
| `DAILY_LIMIT_REACHED` | 409 | New-card limit reached for today |

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **95% domain (the scheduler is pure and must be provably correct), 85% service**

```bash
go test ./internal/modules/srs/...                    # unit
go test -tags=integration ./internal/modules/srs/...  # integration (testcontainers)
```

**Focus areas**

- FSRS invariants under property-based testing
- Interval monotonicity across grades
- Lapse reduces but does not zero stability
- Early review does not punish the learner
- Daily limits applied at session build time
- Due-count cache invalidation on every answer
- Timezone correctness for 'today' across DST
- Concurrent answers to the same card produce one log
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not put I/O in the scheduling functions.
- Do not add a fifth grade.
- Do not reset stability to zero on a lapse.
- Do not change parameters without simulating against historical logs.
- Do not let cards reference mutable content items.
- Do not prune review logs before the retention window — they are the training data for future tuning.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
