---
module: gamification
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: learn
tables: [xp_events, streaks, badges, badges_earned, quests, user_quests, leaderboard_snapshots]
depends_on: [learning, srs, cache, job, notification]
depended_on_by: [notification, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# gamification — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/gamification` |
| Schema | `learn` |
| Delivery phase | 3 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
Motivation mechanics: XP, levels, streaks, badges, quests and leaderboards. It reacts to learning events and never drives them.
<!-- END GENERATED: overview -->

**Context.** Gamification is downstream by design. It subscribes to events from `learning`, `srs`, `writing` and `speaking`, and owns no learning logic — so a change to how motivation works can never break how learning works.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- XP awards per learning action, with anti-farming rules
- Levels derived from cumulative XP
- Streaks with freezes and timezone-correct day boundaries
- Badges and their unlock conditions
- Quests: time-boxed multi-step goals
- Leaderboards (opt-in, league-based)
- Daily goal tracking

**This module does NOT own:**

- Deciding what counts as learning — the source modules do
- Sending reminders — that is `notification`
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/gamification/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/gamification/contract/` | You are calling this module from another module |
| `internal/modules/gamification/service/` | You are changing behaviour |
| `db/migrations/gamification/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/gamification/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `gamification.Reader` | `SummaryOf(ctx, userID)` — used by the dashboard and `notification` |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `gamification.xp_awarded` | publishes | `{user_id, amount, source}` |
| `gamification.level_up` | publishes | `{user_id, level}` |
| `gamification.badge_earned` | publishes | `{user_id, badge_code}` |
| `gamification.streak_at_risk` | publishes | `{user_id, hours_remaining}` — drives the reminder |
| `gamification.streak_broken` | publishes | `{user_id, previous_length}` |
| `activity.completed` | consumes | Award XP |
| `lesson.completed` | consumes | Award XP and progress quests |
| `review.session_completed` | consumes | Award XP and extend the streak |
| `writing.graded` | consumes | Award XP |
| `speaking.scored` | consumes | Award XP |
| `exam.attempt_finished` | consumes | Award XP and badges |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `learn` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/gamification/` · Queries: `db/queries/gamification/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `learn.xp_events` | Every XP award | Partitioned monthly. `user_id`, `source`, `source_id`, `amount`, `multiplier`, `awarded_at`. Unique on (user_id, source, source_id) for idempotency |
| `learn.streaks` | Streak state | `user_id`, `current`, `longest`, `last_active_on`, `freezes_available`, `freeze_used_on` |
| `learn.badges` | Badge catalogue | `code`, `name`, `criteria` jsonb, `tier` |
| `learn.badges_earned` | Awards | Unique on (user_id, badge_id) |
| `learn.quests` | Time-boxed goals | `code`, `steps` jsonb, `window`, `reward` |
| `learn.user_quests` | Quest progress | `user_id`, `quest_id`, `progress` jsonb, `completed_at` |
| `learn.leaderboard_snapshots` | Weekly league standings | Materialised weekly; opt-in only |


<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `gamification`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/me/gamification` | `self` | XP, level, streak, badges, active quests |
| `GET` | `/api/v1/me/streak` | `self` | Streak with the freeze state and the day boundary |
| `POST` | `/api/v1/me/streak/freeze` | `self` | Use a freeze |
| `GET` | `/api/v1/leaderboard` | `self` | Current league standings |
| `PUT` | `/api/v1/me/daily-goal` | `self` | Set the daily XP goal |
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
| [`learning`](../../modules/learning/AGENT.md) | → depends on | see its contract |
| [`srs`](../../modules/srs/AGENT.md) | → depends on | see its contract |
| [`cache`](../../platform/cache/AGENT.md) | → depends on | see its contract |
| [`job`](../../platform/job/AGENT.md) | → depends on | see its contract |
| [`notification`](../../modules/notification/AGENT.md) | → depends on | see its contract |
| [`notification`](../../modules/notification/AGENT.md) | ← used by | consumes this module's contract |
| [`analytics`](../../modules/analytics/AGENT.md) | ← used by | consumes this module's contract |
| [`admin`](../../modules/admin/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-GAMIFICATION-01** — XP is awarded from events only, and is idempotent on (user, source, source_id) — a redelivered event must not double-award.
2. **BR-GAMIFICATION-02** — The streak day boundary is in the learner's own timezone, and a timezone change never retroactively breaks a streak.
3. **BR-GAMIFICATION-03** — A streak extends when the daily goal is met, not merely when the app is opened.
4. **BR-GAMIFICATION-04** — Freezes are limited and replenish slowly; a freeze is consumed automatically at the day boundary if the goal was missed and one is available.
5. **BR-GAMIFICATION-05** — Anti-farming: XP per source type is capped daily, and repeating the same activity yields diminishing returns.
6. **BR-GAMIFICATION-06** — Badges are idempotent — earning conditions are re-evaluated safely and a badge is awarded once.
7. **BR-GAMIFICATION-07** — Leaderboards are opt-in, display names only, and are league-based so a learner competes with peers rather than with the whole platform.
8. **BR-GAMIFICATION-08** — Gamification never blocks learning: running out of XP, freezes or league position must not prevent any learning action.
9. **BR-GAMIFICATION-09** — Streak reminders respect quiet hours and are sent by `notification`, not from here.
<!-- END GENERATED: rules -->


## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a badge

1. Add the badge with its criteria expression to the catalogue.
2. Implement the evaluator, which must be idempotent and cheap — it runs on many events.
3. Backfill for existing learners with a job if the badge is retroactive.
4. Add the artwork and the description copy.
5. Test that re-running the evaluator awards it exactly once.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Leaderboard leagues are static rather than skill-matched.
- Quests are hand-authored; there is no generation from a learner's profile.
- Anti-farming is rule-based and will not catch a determined script.
- Streak evaluation runs per timezone bucket, so a learner who changes timezone mid-day sees an approximation.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:gamification:summary:{user_id}:v1` | 2 min | Any XP award |
| `fluentra:{env}:gamification:leaderboard:{league}:{week}:v1` | 1 min | Periodic rebuild |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `NO_FREEZES_AVAILABLE` | 409 | No streak freeze remaining |
| `LEADERBOARD_NOT_OPTED_IN` | 403 | Learner has not opted in |
| `DAILY_XP_CAP_REACHED` | 200 | Not an error — the award is capped and reported as such |


## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/gamification/...                    # unit
go test -tags=integration ./internal/modules/gamification/...  # integration (testcontainers)
```

**Focus areas**

- XP idempotency under redelivered events
- Streak day boundaries across timezones and DST
- Timezone change does not retroactively break a streak
- Freeze consumption at the boundary
- Daily caps and diminishing returns
- Badge awarded exactly once under concurrent evaluation
- Leaderboard excludes learners who have not opted in
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not call gamification synchronously from a learning flow.
- Do not let a gamification failure block a learning action.
- Do not award XP without an idempotency key.
- Do not break a streak because a learner travelled.
- Do not put a learner on a leaderboard without opt-in.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
