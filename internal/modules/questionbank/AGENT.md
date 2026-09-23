---
module: questionbank
tier: learning
group: modules
status: IMPLEMENTED
phase: 4
owner: "@learning-team"
schema: assess
tables: [questions, question_stats]
depends_on: [content, lesson, learning, rbac]
depended_on_by: [exam]
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
| Status | **IMPLEMENTED** |
| Owner | @learning-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
The exam item bank: generated questions tagged to the spine and to an exam part, each a content version drawn as an activity, with provenance, a fingerprint and empirical statistics.
<!-- END GENERATED: overview -->

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Bank metadata for exam questions: kind, skill, CEFR level, exam part, questions per group, provenance
- Generating draft questions for a part and spine nodes through `learning.Generator`
- A normalised fingerprint per question, unique in the database
- Moving an approved question into the bank course (`pool-bank`) once its content is published
- Answering which published questions an exam can draw for a part
- Holding `question_stats` for empirical difficulty

**This module does NOT own:**

- The question's body, options and answer key — that is a `content` version, graded by the existing graders
- Reviewing a question — that is `content`'s review queue (`/admin/review-queue`)
- Composing or delivering a test — that is `exam`
- Question sets — a set is a mock test composition, which `exam` stores
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
| interface | `questionbank.Reader` | `ListQuestions` (permission-checked) and `DrawableForPart` (system read for `exam`: published questions with an activity, stable order) |
| interface | `questionbank.Author` | `GenerateQuestions`, `PublishQuestion` (refuses unreviewed content), `RetireQuestion` |
| struct | `questionbank.Question` | Bank metadata only; the body is the content version it points at |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `content.published` | consumes | Approving a bank question's content in the review queue appends it to the bank course and marks it published |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `assess` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/questionbank/` · Queries: `db/queries/questionbank/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `assess.questions` | One item | Content-versioned. `content_item_id`, `activity_id`, `exam_part_id`, `kind`, `skill`, `cefr_level`, `difficulty`, `question_count`, `fingerprint`, `provenance`, `status` |
| `assess.question_stats` | Empirical difficulty | `question_id`, `attempts`, `p_value`, `discrimination`, `avg_time_ms`, `last_computed_at` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `questionbank`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/questions` | `questionbank.read` | Filter by exam part, kind, CEFR, spine node, status. Reachable by moderators |
| `POST` | `/api/v1/admin/questions/generate` | `questionbank.create` | Generate draft questions for a part and nodes; they wait in the review queue |
| `GET` | `/api/v1/admin/questions/{id}/stats` | `questionbank.read` | Empirical difficulty and discrimination. Reachable by moderators |
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
| [`lesson`](../../modules/lesson/AGENT.md) | → depends on | see its contract |
| [`learning`](../../modules/learning/AGENT.md) | → depends on | see its contract |
| [`rbac`](../../modules/rbac/AGENT.md) | → depends on | see its contract |
| [`exam`](../../modules/exam/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-QUESTIONBANK-01** — A bank question's body is a content version and it is drawn as an activity. There is no second copy of its answer (no `question_options`).
2. **BR-QUESTIONBANK-02** — The fingerprint is unique; a duplicate is refused by the database.
3. **BR-QUESTIONBANK-03** — Every bank question carries provenance; an item without it is refused.
4. **BR-QUESTIONBANK-04** — A question enters the bank only after its content version is approved, by a person or by an independent verifier that confirmed it. `PublishQuestion` on unreviewed content fails with `QUESTION_NOT_REVIEWED`.
5. **BR-QUESTIONBANK-05** — Spine tags are `content.content_tags` on the question's content item, resolved through `content.TagIndex` — never joined from this module's SQL.
6. **BR-QUESTIONBANK-06** — The correct answer never reaches a learner: sittings are served through the existing redaction.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a question kind

1. Register the kind's grader and redaction rule in its skill module and in `content/contract/redact.go`.
2. Add a runner component on the web.
3. Teach `domain.FingerprintFromBody` the kind's question text and options.
4. Seed an `assess.exam_parts` row whose `kind` it is.
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
