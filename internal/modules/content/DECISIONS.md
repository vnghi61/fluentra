---
module: content
tier: learning
group: modules
status: PLANNED
phase: 2
owner: "@learning-team"
schema: content
tables: [content_items, content_versions, media_assets, taxonomies, content_tags, content_reviews]
depends_on: [storage, search, audit, ai, media]
depended_on_by: [lesson, learning, vocabulary, grammar, reading, listening, speaking, writing, questionbank]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# content — Decisions

Module-local decisions. Anything that affects other modules, adds a dependency, or changes a
contract belongs in a repository-level ADR instead — see [`/DECISIONS.md`](../../../DECISIONS.md).

## Decisions taken

<!-- BEGIN GENERATED: decisions -->
| Question | Decision | Rationale |
|---|---|---|
| Shared content model or per-skill models? | Shared | Six near-identical models would diverge within a year and make mixed-skill lessons impossible. The genuinely different part — the grader — stays in the skill module (ADR-0015) |
| Mutable content or versions? | Immutable versions | A learner's attempt must be interpretable against the material they actually saw; editing published material in place would silently rewrite history |
| Who assigns the CEFR level? | A human, with AI assistance | Levelling is a pedagogical judgement with real consequences for learner experience; the model proposes, the author disposes |
| Who may read published content, and who may change it? | Any signed-in learner reads; only an administrator writes | Published material is the first thing a learner reads that is not their own data, so `self` cannot express it and it needs a named permission — `content.read.published`, held by the `user` role. Leaving the reads anonymous would hand a whole course to anyone with the URL and remove the surface Phase 4 attaches entitlements to. Create, edit, review, publish and archive stay with `admin` (migration 1700000180) |
<!-- END GENERATED: decisions -->

## Related repository ADRs

<!-- BEGIN GENERATED: decisions-adr -->
- [ADR-0015](../../../docs/adr/ADR-0015-content-exercise-core.md) — Shared content + exercise engine
<!-- END GENERATED: decisions-adr -->

## Open questions

<!-- BEGIN GENERATED: decisions-open -->
_None._
<!-- END GENERATED: decisions-open -->
