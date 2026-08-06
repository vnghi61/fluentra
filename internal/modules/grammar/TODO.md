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

# grammar — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3

- [ ] Taxonomy with CEFR levels and prerequisites
- [ ] Rules with examples and common errors
- [ ] Error tagging consuming writing and speaking events
- [ ] Weakness profile with decay
- [ ] Drill types and graders
- [ ] Grounded `grammar.explain` with a citation check
- [ ] Seed taxonomy covering A1–B2
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Corpus-derived common error lists
- Contrastive explanations for Vietnamese speakers specifically
- Automatic drill generation from a learner's own errors
<!-- END GENERATED: todo-future -->
