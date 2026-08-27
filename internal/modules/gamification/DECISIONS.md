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

# gamification — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Event-driven or called directly? | Event-driven | It keeps motivation mechanics strictly downstream — a bug here can never break a learning flow, and learning modules need no knowledge of XP at all |
| Streak freezes? | Yes, limited and automatic | An all-or-nothing streak punishes the illness or travel that real learners have, and losing a long streak is a strong churn trigger. Forgiveness retains better than strictness |
| Global leaderboard? | No — opt-in leagues | A global board demotivates everyone outside the top few and creates a privacy exposure nobody asked for |
| Which day boundary does a streak use? | The learner's own timezone | core.users already stores a timezone and the seed sets Asia/Ho_Chi_Minh. Counting in UTC would end the day at 7am local, so a learner studying in the evening in Vietnam loses a streak they kept |
| What identity appears on a league board? | The display name, and only inside a league the learner opted into | Display name is the only name a learner chooses and the only one already meant to be seen; email and profile are never on a board. The opt-in boundary is what keeps this compatible with refusing a global leaderboard — a name is shown to people the learner joined, not to everyone |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
