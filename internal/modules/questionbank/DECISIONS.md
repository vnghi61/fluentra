---
module: questionbank
tier: learning
group: modules
status: PLANNED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_options, question_sets, question_set_items, question_stats]
depends_on: [content, ai, audit, search]
depended_on_by: [exam, reading, listening, grammar, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# questionbank — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Classical statistics or IRT? | Classical for now | IRT needs far more data per item and a calibration process we cannot staff yet; the interface leaves room to upgrade |
| Allow AI-generated items to auto-publish? | Only for generated practice pool | By the owner's decision on 2026-09-11, generated practice pool items auto-publish with 6-stage verification (including blind solve model agreement) and the learner report queue as compensating controls; exam and authored curriculum items still require human review |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
