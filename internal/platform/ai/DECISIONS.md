---
module: ai
tier: platform
group: platform
status: PLANNED
phase: 3
owner: "@ai-team"
schema: ai
tables: [ai_requests, ai_usage, prompt_versions, ai_cache_entries, ai_budgets]
depends_on: [cache, telemetry, job]
depended_on_by: [writing, speaking, grammar, questionbank, content, reading, media, learning]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# ai — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Task-based or model-based API? | Task-based | Callers should not know or care which model runs; it makes model changes a config decision, keeps routing and cost policy in one place, and makes the eval story coherent |
| Where do prompts live? | Versioned Markdown in `docs/prompts/runtime/` | Reviewable by non-engineers, diffable, evaluable in CI, rollback-able by config — none of which is true of a Go string constant |
| Fail open or closed on budget exhaustion? | Shed non-critical tasks, keep grading | A learner who paid for grading should still get it; example-sentence generation can wait |
| Semantic cache everywhere? | Only where measured hit rate justifies the embedding cost | An embedding call to save a small-model call can be a net loss |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0011](../../../docs/adr/ADR-0011-ai-provider-abstraction.md) — Task-based AI provider abstraction
- [ADR-0012](../../../docs/adr/ADR-0012-prompt-versioning.md) — Prompts as versioned, evaluated artefacts
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
- Do we self-host a small model for high-volume cheap tasks once volume justifies it?
- Should the semantic cache be per-user or global for privacy-sensitive tasks?
<!-- END GENERATED: decisions-open -->
