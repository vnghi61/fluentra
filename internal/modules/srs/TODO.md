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

# srs — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 2

- [ ] FSRS implementation in `domain/` with property-based tests
- [ ] Cards, logs and daily stats tables with partitioning
- [ ] Session building with limits and interleaving
- [ ] Answer endpoint with rescheduling and event emission
- [ ] Due-count caching with correct invalidation
- [ ] Suspend, bury and reset
- [ ] Review player in the web app

## Phase 3

- [ ] Forecast and workload projection
- [ ] Heat map
- [ ] `review.due_soon` reminder job

## Phase 5

- [ ] Per-learner parameter optimisation from review logs
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Per-learner FSRS optimisation
- Load balancing to smooth daily workload
- Cross-skill interference modelling
- Offline review queue
<!-- END GENERATED: todo-future -->
