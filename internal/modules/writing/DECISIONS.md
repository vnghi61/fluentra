---
module: writing
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [writing_tasks, writing_drafts, writing_submissions, writing_feedback, writing_revisions]
depends_on: [ai, job, content, learning, notification]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# writing — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Synchronous or asynchronous grading? | Asynchronous with streaming | 10–30 seconds is far outside an acceptable request budget, and a provider outage must not surface as a failed submission |
| Charge quota on submit or on success? | On success | A learner should never lose a credit to our infrastructure failure |
| Allow editing a submission? | No — resubmit instead | Band progression only means something if each submission is a fixed artefact |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
