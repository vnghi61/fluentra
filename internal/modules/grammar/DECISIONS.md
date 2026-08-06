---
module: grammar
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [grammar_points, grammar_rules, grammar_exercises, error_tags, user_grammar_state]
depends_on: [content, srs, ai, learning]
depended_on_by: [writing, speaking, learning, exam]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# grammar — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Free-form AI explanation or grounded citation? | Grounded | An ungrounded model will invent a confident, plausible, wrong rule — and a learner cannot tell the difference. Citation makes the explanation checkable and the failure mode visible |
| Tag errors on first occurrence or after teaching? | Tag always, schedule only after teaching | The weakness profile needs all the data; spaced repetition of something never taught is just confusing |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
