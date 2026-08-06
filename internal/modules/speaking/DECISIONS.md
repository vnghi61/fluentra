---
module: speaking
tier: learning
group: modules
status: PLANNED
phase: 3
owner: "@learning-team"
schema: skill
tables: [speaking_tasks, speaking_attempts, pronunciation_scores, speaking_feedback]
depends_on: [media, ai, storage, job, content, learning]
depended_on_by: [learning, analytics, gamification]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# speaking — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Buy or build pronunciation assessment? | Buy, behind an interface | Phoneme-level scoring is a research problem; buying lets us ship and measure, and the interface keeps a self-hosted GOP option open (plan review Q2) |
| Keep recordings indefinitely? | 90 days, learner-deletable, pinnable | Storage cost and privacy exposure both grow with retention, and the scores — which are what progress is built on — do not need the audio |
| Send audio to the LLM? | No — transcript and scores only | Cheaper, faster, and it keeps voice data out of a second vendor's hands |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
