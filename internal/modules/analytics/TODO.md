---
module: analytics
tier: commerce
group: modules
status: PLANNED
phase: 4
owner: "@data-team"
schema: analytics
tables: [analytics_events, daily_rollups, cohorts, funnel_steps, learner_outcomes]
depends_on: [job, cache]
depended_on_by: [admin, notification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# analytics — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 4

- [ ] Event ingestion with idempotency and partitioning
- [ ] Nightly rollup job with dirty-day recomputation
- [ ] Funnel and retention computation
- [ ] Learning outcome metrics
- [ ] AI cost per active learner
- [ ] Admin KPI dashboards
- [ ] Learner insights endpoint
- [ ] Weekly progress email payload
- [ ] Metric definitions document
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Real-time streaming metrics
- A dedicated analytical store
- Experiment framework with proper statistics
- Predictive churn scoring
<!-- END GENERATED: todo-future -->
