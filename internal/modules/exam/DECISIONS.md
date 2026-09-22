---
module: exam
tier: learning
group: modules
status: DONE
phase: 3
owner: "@learning-team"
schema: assess
tables: [exams, exam_sections, exam_attempts, score_reports, integrity_events, exam_versions, exam_parts, blueprints, mock_tests]
depends_on: [questionbank, job, ai, writing, speaking, learning, lesson, listening]
depended_on_by: [learning, analytics, admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# exam — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Client or server timing? | Server, always | Client timing is trivially manipulable, and an exam whose timing can be manipulated has no value as practice |
| Auto-invalidate on integrity signals? | No — record and show | False positives (a notification, a second monitor) would penalise honest learners; the signal is more useful as self-awareness feedback than as enforcement |
| Store or recompute score reports? | Store | A learner's past band must not change because we revised a conversion table |
| How are exam timeouts enforced? | 3-tier expiry: River job at deadline, 1-minute sweep cron, and lazy check on read | Guarantees attempt finalisation and score reporting even if learner abandons sitting or background workers experience delays |
| How does practice mode differ from exam mode? | Custom duration 10–180 minutes or unlimited (a generous backstop still applies), free section navigation, and re-recordable speaking | Enables targeted practice without breaking realistic exam constraints |
| How is the exam pool isolated from the practice pool? | Distinct pool course pool-exam and strict refusal guards in preview and lessons | Prevents exam questions from leaking through regular study or practice routes |
| When can a listening item be drawn for an exam sitting? | Only when TTS audio has been rendered and object key exists in storage | A learner cannot be tested on listening audio that does not exist |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
