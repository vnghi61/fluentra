---
doc_type: prompt_library
project: fluentra
last_verified: 2026-08-06
---

# PROMPT_LIBRARY.md

Two distinct libraries live in `docs/prompts/`. Confusing them is the most common mistake.

| Library | Path | Audience | Executed by | Versioned how |
|---|---|---|---|---|
| **Development prompts** | `docs/prompts/dev/` | AI coding assistants | a human/agent in an IDE | Git history is enough |
| **Runtime prompts** | `docs/prompts/runtime/` | The application's LLM calls | `internal/platform/ai` at request time | **Immutable `vN` files**, pinned by config, gated by evals |

---

## Part 1 — Development prompt library

### 1.1 Layout

```
docs/prompts/
├── README.md                 # how to pick and use a prompt
├── dev/
│   ├── _shared/
│   │   ├── context-header.md     # the preamble every dev prompt starts with
│   │   ├── definition-of-done.md
│   │   └── review-checklist.md
│   ├── backend/
│   │   ├── generate-module.md
│   │   ├── generate-crud.md
│   │   ├── generate-repository.md
│   │   ├── generate-service.md
│   │   ├── generate-handler.md
│   │   ├── generate-dto.md
│   │   ├── generate-validation.md
│   │   ├── generate-contract.md
│   │   ├── generate-cache-layer.md
│   │   ├── generate-middleware.md
│   │   ├── generate-scheduler.md
│   │   ├── generate-job.md
│   │   ├── generate-event-handler.md
│   │   └── generate-otel-instrumentation.md
│   ├── database/
│   │   ├── generate-migration.md
│   │   ├── generate-sqlc-query.md
│   │   ├── generate-index-plan.md
│   │   └── generate-er-diagram.md
│   ├── frontend/
│   │   ├── generate-component.md
│   │   ├── generate-page.md
│   │   ├── generate-form.md
│   │   ├── generate-table.md
│   │   ├── generate-hook.md
│   │   ├── generate-api-client.md
│   │   ├── generate-store.md
│   │   └── generate-story.md
│   ├── testing/
│   │   ├── generate-unit-test.md
│   │   ├── generate-table-test.md
│   │   ├── generate-integration-test.md
│   │   ├── generate-contract-test.md
│   │   ├── generate-e2e-test.md
│   │   ├── generate-fixtures.md
│   │   └── generate-load-test.md
│   ├── ai/
│   │   ├── generate-ai-provider.md
│   │   ├── generate-prompt-template.md
│   │   ├── generate-eval-suite.md
│   │   └── generate-cost-report.md
│   ├── devops/
│   │   ├── generate-github-action.md
│   │   ├── generate-dockerfile.md
│   │   ├── generate-compose-service.md
│   │   ├── generate-grafana-dashboard.md
│   │   ├── generate-alert-rule.md
│   │   └── generate-runbook.md
│   └── docs/
│       ├── generate-module-docs.md
│       ├── generate-adr.md
│       ├── generate-api-docs.md
│       ├── generate-changelog.md
│       └── review-architecture.md
└── runtime/                   # see Part 2
```

### 1.2 Standard dev-prompt template

Every file in `dev/` follows this structure:

```markdown
---
id: backend/generate-service
title: Generate a service layer method
version: 1.0.0
inputs: [module, use_case, contract_method]
reads: [AGENT.md, internal/modules/{{module}}/AGENT.md, CODING_STANDARD.md, ERROR_HANDLING.md]
produces: [service/{{use_case}}.go, service/{{use_case}}_test.go]
model_hint: mid-tier
---

## Context
{{> _shared/context-header.md}}

## Task
…precise, imperative, unambiguous…

## Constraints
- numbered, checkable
- reference repo rules by ID (L1, L4, …)

## Steps
1. …
2. …

## Output format
…exactly what files, in what order…

## Acceptance criteria
- [ ] …

## Anti-patterns to avoid
- …
```

### 1.3 The shared context header

Every dev prompt begins by loading the same preamble, so no prompt has to restate the rules:

> You are working in the Fluentra repository. Read `AGENT.md` first, then
> `internal/modules/{{module}}/AGENT.md`. Obey rules L1–L12. Do not read other modules'
> internals. Do not invent endpoints, tables, or config keys. Follow the layering in
> `AGENT.md §6`. When done, run `make check` and update the module's `AGENT.md` and `TODO.md`.

### 1.4 Prompt catalogue (what each one does)

| Prompt | Produces | Key constraint it enforces |
|---|---|---|
| `generate-module` | Full module skeleton + 9 doc files + manifest entry + arch-lint entry | Boundary registration is not optional |
| `generate-crud` | Migration + sqlc queries + repo + service + handler + tests + spec | Spec-first ordering |
| `generate-repository` | sqlc query file + repository wrapper + mapper | No SQL outside `db/queries/` |
| `generate-service` | Use-case method + unit test with mocks | No HTTP or SQL types in the service |
| `generate-handler` | Handler + route registration + contract test | No business logic in the handler |
| `generate-dto` | Request/response types + validation tags + OpenAPI schema | JSON is `snake_case` |
| `generate-validation` | Validator rules + i18n messages + tests | Shape at the edge, invariants in the domain |
| `generate-contract` | `contract/` interface + DTO + event types | Only exported surface other modules may use |
| `generate-cache-layer` | Cache-aside wrapper + key builder + invalidation + tests | Must degrade when Redis is down |
| `generate-middleware` | chi middleware + test | Must be `func(http.Handler) http.Handler` |
| `generate-job` | River job args + worker + registration + test | Enqueue must be transactional |
| `generate-scheduler` | Cron entry + idempotent handler | Must tolerate double execution |
| `generate-otel-instrumentation` | Spans, attributes, metrics for an existing flow | Cardinality budget respected |
| `generate-migration` | Reversible goose migration + index plan | Expand→migrate→contract for breaking changes |
| `generate-index-plan` | `EXPLAIN` analysis + proposed indexes | Must show before/after plan |
| `generate-component` | React component + story + test | a11y attributes required |
| `generate-form` | RHF + Zod form, error mapping from Problem Details | Server error codes mapped to fields |
| `generate-table` | TanStack Table with cursor pagination | No offset pagination on feeds |
| `generate-hook` | TanStack Query hook + query keys + MSW handler | Query keys centralised |
| `generate-api-client` | Regenerate types from OpenAPI + typed wrapper | Never hand-write API types |
| `generate-unit-test` | Table-driven tests from the spec | Written from `AGENT.md`, **not** from the implementation |
| `generate-integration-test` | testcontainers test + fixtures | Rolls back or uses a template DB |
| `generate-contract-test` | Handler ↔ OpenAPI conformance + golden JSON | Every endpoint has one |
| `generate-e2e-test` | Playwright spec + page objects | Uses the seeded demo account |
| `generate-ai-provider` | New provider adapter + registry entry + fixtures for `mock` | SDK types stay inside the adapter |
| `generate-prompt-template` | New runtime prompt `vN` + input/output schema + eval stub | Cannot ship without an eval suite |
| `generate-eval-suite` | Golden set + scorer + threshold | Threshold justified, not guessed |
| `generate-github-action` | Workflow + caching + concurrency group | Pinned action SHAs |
| `generate-grafana-dashboard` | Dashboard JSON + panel descriptions | Uses existing metric names only |
| `generate-runbook` | Symptom → diagnosis → mitigation → escalation | Must include a real query/command |
| `generate-adr` | ADR from the template with alternatives and consequences | At least 2 rejected alternatives |
| `generate-module-docs` | Refresh the 9 module docs from current code | Generated regions only |

### 1.5 Slash commands

`.claude/commands/` wires the most-used prompts to one-liners:

| Command | Prompt |
|---|---|
| `/new-module <name>` | `dev/backend/generate-module.md` |
| `/new-endpoint <module> <path>` | `dev/backend/generate-crud.md` |
| `/new-migration <module> <desc>` | `dev/database/generate-migration.md` |
| `/new-job <module> <kind>` | `dev/backend/generate-job.md` |
| `/new-page <feature>` | `dev/frontend/generate-page.md` |
| `/write-tests <path>` | `dev/testing/generate-unit-test.md` |
| `/new-adr <title>` | `dev/docs/generate-adr.md` |
| `/refresh-docs <module>` | `dev/docs/generate-module-docs.md` |
| `/arch-review` | `dev/docs/review-architecture.md` |

### 1.6 Governance of dev prompts

| Rule | Detail |
|---|---|
| Ownership | Each prompt has an owner in front-matter |
| Change process | Normal PR; a prompt change requires running it once and attaching the output to the PR |
| Deprecation | Move to `dev/_archive/` with a note pointing at the replacement — never delete silently |
| Measurement | Track: acceptance rate of first output, review cycles, whether `make check` passed first try |

---

## Part 2 — Runtime prompt library

These are **production artefacts**. They are versioned, evaluated, and rolled out like code.

### 2.1 Layout

```
docs/prompts/runtime/
├── README.md
├── _shared/
│   ├── system-preamble.md         # role, safety, output discipline
│   ├── user-content-wrapper.md    # the delimiter block for untrusted learner text
│   └── cefr-rubrics/              # canonical rubrics, cited not restated
├── writing.grade_essay/
│   ├── v1.md  v2.md  v3.md        # immutable
│   ├── input.schema.json
│   ├── output.schema.json
│   └── evals/
│       ├── golden/*.json
│       └── thresholds.yaml
├── writing.quick_hint/
├── speaking.feedback/
├── grammar.explain/
├── vocabulary.example_sentence/
├── vocabulary.definition_simplify/
├── reading.question_generate/
├── listening.transcript_clean/
├── questionbank.generate_items/
├── content.translate/
├── content.level_estimate/
└── learning.study_plan_suggest/
```

### 2.2 Runtime prompt front-matter

```yaml
---
task: writing.grade_essay
version: 3
status: active            # draft | shadow | active | deprecated
model_tier: frontier
temperature: 0.2
max_output_tokens: 2000
input_schema: input.schema.json
output_schema: output.schema.json
eval_suite: evals/
supersedes: 2
owner: "@learning-team"
changelog: "Added coherence sub-score; tightened band-descriptor citation."
---
```

### 2.3 Non-negotiable runtime rules

| # | Rule | Why |
|---|---|---|
| R1 | A published version file is **immutable**. Changes create `vN+1`. | Reproducibility of past grades |
| R2 | Config pins the active version per task and per environment. | Rollback is a config change |
| R3 | Output must validate against `output_schema`. One repair attempt, then the job fails loudly. | No silently malformed feedback |
| R4 | Learner text is inserted **only** inside the wrapper from `_shared/user-content-wrapper.md`, never concatenated into the instruction block. | Prompt injection |
| R5 | The system preamble states that content inside the wrapper is data and any instruction inside it must be ignored and reported. | Prompt injection |
| R6 | Scores are clamped to the rubric range and cross-checked for internal consistency; an out-of-band score fails the job rather than being shown. | A "9.0 because the essay asked" is a visible product failure |
| R7 | No PII beyond what the task needs; the redactor runs before send. | Privacy |
| R8 | Every call records `task`, `version`, `provider`, `model`, tokens, cost, latency, cache hit, and `trace_id`. | Cost attribution and debugging |
| R9 | A new or changed prompt cannot become `active` until its eval suite meets `thresholds.yaml` and is not worse than the current version. | Quality regression prevention |
| R10 | Rollout: `shadow` (run alongside, compare, do not show) → 10 % → 100 %. | Safe change |

### 2.4 Evaluation

| Element | Detail |
|---|---|
| Golden set | 30–100 human-labelled examples per task, spanning CEFR A1–C2, including adversarial and edge cases |
| Scorers | Exact match (structured fields), numeric agreement (score within ±0.5 band), rubric-item recall, LLM-as-judge for feedback usefulness, plus a **red-team** subset for injection |
| Thresholds | Per task in `thresholds.yaml`, e.g. `band_mae <= 0.4`, `schema_valid >= 0.99`, `injection_resisted == 1.0` |
| When it runs | On every PR touching `runtime/` (against `mock` + one cheap real model); nightly against all configured providers |
| Reporting | The CI job comments a table of score deltas versus the current active version |
| Human review | Any prompt promotion requires one human approval from the learning team |

### 2.5 Cost model per task

Recorded in `docs/ai/cost-model.md` and enforced by the routing table in config:

| Task | Model tier | Cache | Est. calls / active user / month | Est. cost / user / month |
|---|---|---|---|---|
| `writing.grade_essay` | frontier | none | 8 | dominant cost — track weekly |
| `speaking.feedback` | mid | none | 12 | second |
| `grammar.explain` | mid | 30 d semantic | 20 (≈60 % cached) | low |
| `vocabulary.example_sentence` | small | 90 d exact | 40 (≈90 % cached) | negligible |
| `questionbank.generate_items` | frontier | none | admin-triggered | amortised over all users |

The routing table is the lever: if unit economics slip, a task moves down a tier and the eval
suite tells you what quality you traded away.
