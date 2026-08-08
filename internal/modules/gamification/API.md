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

# gamification — API Reference

> The **contract** is [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml), tag `gamification`.
> This file is a human-readable summary. If the two disagree, the spec is right and this file is a bug —
> CI's `api-drift` check will fail on the discrepancy.

Conventions: [`/API_GUIDELINE.md`](../../../API_GUIDELINE.md).
Error format: RFC 9457 Problem Details — [`/ERROR_HANDLING.md`](../../../ERROR_HANDLING.md).

## Endpoint summary

<!-- BEGIN GENERATED: api-summary -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/me/gamification` | `self` | XP, level, streak, badges, active quests |
| `GET` | `/api/v1/me/streak` | `self` | Streak with the freeze state and the day boundary |
| `POST` | `/api/v1/me/streak/freeze` | `self` | Use a freeze |
| `GET` | `/api/v1/leaderboard` | `self` | Current league standings |
| `PUT` | `/api/v1/me/daily-goal` | `self` | Set the daily XP goal |
<!-- END GENERATED: api-summary -->

## Endpoint detail

<!-- BEGIN GENERATED: api-detail -->
### `GET /api/v1/me/gamification`

XP, level, streak, badges, active quests

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `GET /api/v1/me/streak`

Streak with the freeze state and the day boundary

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

### `POST /api/v1/me/streak/freeze`

Use a freeze

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `NO_FREEZES_AVAILABLE` |

### `GET /api/v1/leaderboard`

Current league standings

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | `LEADERBOARD_NOT_OPTED_IN` |

### `PUT /api/v1/me/daily-goal`

Set the daily XP goal

| | |
|---|---|
| Permission | `self` |
| Success | 200 |
| Errors | standard set |

<!-- END GENERATED: api-detail -->

## Error codes

<!-- BEGIN GENERATED: api-errors -->
| Code | Status | Meaning |
|---|---|---|
| `NO_FREEZES_AVAILABLE` | 409 | No streak freeze remaining |
| `LEADERBOARD_NOT_OPTED_IN` | 403 | Learner has not opted in |
| `DAILY_XP_CAP_REACHED` | 200 | Not an error — the award is capped and reported as such |
<!-- END GENERATED: api-errors -->

## Rate limits

<!-- BEGIN GENERATED: api-rate -->
Standard limits apply — see [/API_GUIDELINE.md](../../../API_GUIDELINE.md) §11.
<!-- END GENERATED: api-rate -->
