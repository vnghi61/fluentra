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

# questionbank — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `learning` |
| Path | `internal/modules/questionbank` |
| Schema | `assess` |
| Delivery phase | 4 |
| Status | **PLANNED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
The reusable item bank: authoring, typing, tagging, difficulty statistics, review workflow, and AI-assisted generation. One item, many uses — in a lesson, in a drill, in an exam.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Question items across all supported types
- Options, correct answers and per-option feedback
- Question sets: reusable ordered groups
- Tagging by skill, level, topic and exam relevance
- Difficulty and discrimination statistics from real attempts
- Authoring and review workflow
- AI-assisted item generation for admin review
- Item exposure control so the same items are not overused

**This module does NOT own:**

- Delivering an exam — that is `exam`
- Grading a learner's whole attempt — that is the exercise engine
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/modules/questionbank/module.go` | You need to see what this module depends on and what it exposes |
| `internal/modules/questionbank/contract/` | You are calling this module from another module |
| `internal/modules/questionbank/service/` | You are changing behaviour |
| `db/migrations/questionbank/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/modules/questionbank/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `questionbank.Reader` | `GetSet`, `SampleItems(criteria)` — used by `exam` and by skill modules |
| struct | `questionbank.Item` | `{ID, Type, Stem, Options, CorrectAnswer, Explanation}` — correct answers stripped for learner-facing calls |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `questionbank.item_published` | publishes | `{question_id, skill, level}` |
| `activity.completed` | consumes | Accumulate attempt statistics for difficulty estimation |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `assess` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/questionbank/` · Queries: `db/queries/questionbank/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `assess.questions` | One item | Content-versioned. `type`, `stem`, `explanation`, `cefr_level`, `skill`, `status` |
| `assess.question_options` | Choices and answers | `question_id`, `position`, `text`, `is_correct`, `feedback` |
| `assess.question_sets` | Reusable group | `name`, `purpose`, `shuffle_policy` |
| `assess.question_set_items` | Membership | `set_id`, `question_id`, `position` |
| `assess.question_stats` | Empirical difficulty | `question_id`, `attempts`, `p_value`, `discrimination`, `avg_time_ms`, `last_computed_at` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `questionbank`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/questions` | `questionbank.read` | Search and filter items |
| `POST` | `/api/v1/admin/questions` | `questionbank.create` | Create an item |
| `POST` | `/api/v1/admin/questions/{id}/review` | `questionbank.review` | Approve or reject |
| `POST` | `/api/v1/admin/questions/generate` | `questionbank.create` | AI-generate draft items for review |
| `GET` | `/api/v1/admin/questions/{id}/stats` | `questionbank.read` | Empirical difficulty and discrimination |
<!-- END GENERATED: endpoints -->

## 7. Folder map

<!-- BEGIN GENERATED: folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`content`](../../modules/content/AGENT.md) | → depends on | see its contract |
| [`ai`](../../platform/ai/AGENT.md) | → depends on | see its contract |
| [`audit`](../../modules/audit/AGENT.md) | → depends on | see its contract |
| [`search`](../../platform/search/AGENT.md) | → depends on | see its contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
| [`reading`](../../modules/reading/AGENT.md) | ← used by | consumes this module's contract |
| [`listening`](../../modules/listening/AGENT.md) | ← used by | consumes this module's contract |
| [`grammar`](../../modules/grammar/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-QUESTIONBANK-01** — The correct answer is **never** included in a learner-facing payload. The DTO used by learner endpoints does not contain the field at all — it cannot be leaked by an oversight.
2. **BR-QUESTIONBANK-02** — AI-generated items always enter as drafts. They are never published without human review.
3. **BR-QUESTIONBANK-03** — An author cannot approve their own item.
4. **BR-QUESTIONBANK-04** — Difficulty is empirical (p-value from real attempts), with the authored estimate used only until enough attempts exist.
5. **BR-QUESTIONBANK-05** — An item with fewer than 30 attempts has provisional statistics, clearly marked.
6. **BR-QUESTIONBANK-06** — An item with discrimination below a threshold is flagged for review — it is not distinguishing strong from weak learners.
7. **BR-QUESTIONBANK-07** — Exposure control caps how often an item can appear for the same learner within a window.
8. **BR-QUESTIONBANK-08** — Editing a published item creates a new version; statistics do not carry over, because it is effectively a different item.
9. **BR-QUESTIONBANK-09** — Every item states which skill and CEFR level it targets — an untagged item cannot be sampled.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a question type

1. Define the type, its option schema and its answer schema.
2. Implement grading in the consuming skill module, not here.
3. Add the authoring UI and the learner-facing renderer.
4. Confirm the learner DTO for the new type excludes the answer.
5. Add generation support to the prompt if AI authoring should cover it.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Difficulty is classical test theory (p-value, discrimination), not item response theory — adequate for sampling, not for adaptive certification.
- AI generation quality varies by type; it is strongest for vocabulary and reading MCQs and weakest for nuanced grammar items.
- There is no automatic detection of near-duplicate items beyond stem similarity.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
_No deviations from the global standard._
<!-- END GENERATED: conventions -->

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `SELF_APPROVAL_FORBIDDEN` | 403 | Author reviewing their own item |
| `INSUFFICIENT_ITEMS` | 409 | Not enough approved items matching the sampling criteria |
| `ITEM_IN_USE` | 409 | Cannot archive an item used by a published exam |

### Security considerations

- Correct answers are excluded at the DTO level, not filtered at the handler — a leak must be structurally impossible, not merely unlikely.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **80% service, 90% domain**

```bash
go test ./internal/modules/questionbank/...                    # unit
go test -tags=integration ./internal/modules/questionbank/...  # integration (testcontainers)
```

**Focus areas**

- Learner DTO cannot contain a correct answer, asserted structurally
- Self-approval refused
- Statistics computed correctly and marked provisional below the attempt threshold
- Sampling respects level, skill, exposure and approval status
- Generated items land as drafts and are deduped
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not include the correct answer in a learner-facing response.
- Do not publish an AI-generated item without review.
- Do not carry statistics across a content edit.
- Do not sample from unapproved items.
<!-- END GENERATED: donot -->

---

_Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`._
