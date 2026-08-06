---
module: media
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@platform-team"
schema: content
tables: [media_derivatives, transcripts, tts_cache]
depends_on: [storage, job, ai, telemetry]
depended_on_by: [speaking, listening, content, vocabulary, user]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# media — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| ffmpeg in the worker or a separate service? | In the worker for now | One fewer moving part; the boundary is already an interface, so extraction is a transport change when CPU contention justifies it — see ARCHITECTURE §20.2 where media is the first extraction candidate |
| Which pronunciation scoring approach? | Provider API first, GOP self-hosted as a fallback option | Building phoneme-level scoring in-house is a research project; buying it lets us ship and measure. Open question Q2 in the plan review |
| Keep raw uploads? | Delete after derivatives are verified | Storage cost and privacy; the normalised copy is what any re-scoring would use anyway |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
_None specific to this module._
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
