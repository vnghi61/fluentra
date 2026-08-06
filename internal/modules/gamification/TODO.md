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

# gamification — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] XP events with idempotency and daily caps
- [ ] Levels from cumulative XP
- [ ] Streaks with timezone-correct boundaries and freezes
- [ ] Badge catalogue and idempotent evaluators
- [ ] Daily goal tracking
- [ ] `streak_at_risk` event for reminders
- [ ] Gamification widgets in the web app

## Phase 4

- [ ] Quests
- [ ] Opt-in weekly leagues
- [ ] Leaderboard materialisation job
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Skill-matched leagues
- Personalised quests
- Team or friend challenges
- Seasonal events
<!-- END GENERATED: todo-future -->
