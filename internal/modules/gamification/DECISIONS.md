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
| How much XP does a graded item award? | Its best score ever, divided by ten, granted as the increase over what it has already granted | 80/100 grants 8. Retaking to 100/100 grants 2, not 10. A third attempt grants nothing. XP therefore measures what a learner can do rather than how many times they pressed retry, and cannot be farmed by repetition. The award is never negative: a worse retake takes nothing back |
| Where does the XP high-water mark live? | In this module, not in learn.progress | UpsertProgress sets score = EXCLUDED.score, so learn.progress holds the most recent score and not the best one — a bad retake lowers it. XP must never fall, so it cannot be derived from that column and needs its own per-item record of what has already been granted |
| What earns XP? | Lessons completed, word senses reaching known, and tests passed | The three things a learner does that represent knowledge rather than activity. Tests have nothing to attach to yet — the exam module is Phase 4 and its tables do not exist — so that source is declared and left unwired rather than faked |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
