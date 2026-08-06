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

# analytics — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Same database or a separate analytical store? | Same, partitioned, for now | A second datastore adds a synchronisation problem and operational burden that our volume does not justify; the rollup design means moving later is a change of destination, not of pipeline |
| Ingest content? | Never | Essay and transcript text would turn an analytics store into a second copy of the most sensitive data we hold, for no analytical benefit that aggregate metrics do not already provide |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
