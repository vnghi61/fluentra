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
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0016](../../../docs/adr/ADR-0016-srs-fsrs.md) — FSRS instead of SM-2
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
