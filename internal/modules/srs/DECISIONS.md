---
module: srs
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: learn
tables: [review_cards, review_logs, srs_params, review_daily_stats]
depends_on: [cache, job, content]
depended_on_by: [learning, vocabulary, grammar, gamification, notification, analytics]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# srs — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| FSRS or SM-2? | FSRS | Explicit stability and difficulty modelling, published parameters, and materially fewer reviews for the same retention. SM-2 is simpler but leaves measurable learning value on the table (ADR-0016) |
| Four grades or six? | Four | The algorithm is defined for four; more options increase learner hesitation without improving scheduling accuracy |
| Cards reference versions or items? | Versions | A card must test what the learner actually learned; if the content is revised, a new card is correct |
| How does the dashboard learn the due count? | learning injects srs.QueueReader | Keeps dashboard assembly consolidated inside learning while honoring architecture boundaries (P9.1 Decision 7.1) |
| Who writes review cards, and in which transaction? | Post-commit via contract call with outbox backstop | Avoids cross-module transaction coupling; activity.completed in outbox serves as replayable backstop (P9.1 Decision 7.2) |
| Does answering a review immediately invalidate the dashboard cache? | No, accept 2-minute staleness for alpha | Dashboard has 2-minute TTL; cross-module invalidation deferred to Phase 3 (P9.1 Decision 7.3) |
| One ReviewItem struct or two? | One struct owned by learning/contract | Used by all skill graders implementing ExerciseGrader without requiring c_srs dependencies across 6 skill modules (P9.1 Decision 7.4 / Trap 1) |
| Is GET /reviews/forecast implemented in Phase 2? | Implemented | P9.1 Decision 7.5 deferred it, and the deferral produced a handler answering 200 with an empty day list — a specced endpoint telling every learner they have nothing due. The grouped query is smaller than the workaround, buckets by the learner local day like the due queue, and no screen depends on it either way |
| How does another module stop scheduling content a learner already knows? | srs.CardWriter.SetCardsSuspended, through the contract | learn.review_cards belongs to srs. vocabulary marking a word known has to reach the due queue, and the alternative — a second suspension flag in skill.user_word_state that the due query would have to join — is the table duplication ADR-0015 exists to prevent |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0016](../../../docs/adr/ADR-0016-srs-fsrs.md) — FSRS instead of SM-2
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
