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

# ai — AGENT.md

> AI entry point for this module. Read [`/AGENT.md`](../../../AGENT.md) and
> [`/MODULE_INDEX.md`](../../../MODULE_INDEX.md) first if you have not.
> **Everything you need for this module is below. Do not scan other modules.**

| | |
|---|---|
| Tier | `platform` |
| Path | `internal/platform/ai` |
| Schema | `ai` |
| Delivery phase | 3 |
| Status | **PLANNED** |
| Owner | @ai-team |

---

## 1. Overview

<!-- BEGIN GENERATED: overview -->
The single door through which every LLM call passes. Business code asks for an **AI task** by name; this module decides which provider and model serve it, renders the versioned prompt, enforces quota and budget, caches, retries, falls back, validates the structured output, and records cost and usage.
<!-- END GENERATED: overview -->

**Context.** No other module may import a provider SDK (rule: `AGENT.md` §10). That rule is what makes the model choice a configuration decision instead of a code change, and what makes cost, quality and safety controllable in one place.

## 2. Responsibilities

<!-- BEGIN GENERATED: responsibilities -->
**This module owns:**

- Provider registry and adapters (anthropic, openai, gemini, openrouter, local, mock)
- Task routing: task → model tier → concrete model, from configuration
- Prompt registry: versioned templates with typed input and output schemas
- Resilience: timeout, retry with backoff, circuit breaker, provider fallback chain
- Caching: exact-hash and semantic (pgvector) with per-task policy
- Quota (per user) and budget (global daily) enforcement
- PII redaction before send and untrusted-content wrapping
- Structured output validation, one repair attempt, and sanity bounds
- Streaming (SSE) with partial persistence
- Usage, cost, latency and quality metrics; the `ai_requests` / `ai_usage` audit trail
- The evaluation harness that gates prompt changes

**This module does NOT own:**

- Deciding what a good essay score is — the rubric belongs to `writing`
- Storing a graded submission — that is the calling module's data
- Speech recognition or synthesis — that is `platform/media`
- Choosing when to call an LLM — see `/AI_GUIDE.md` §B2 before you do
<!-- END GENERATED: responsibilities -->

## 3. Entry points

<!-- BEGIN GENERATED: entrypoints -->
| File | Read it when |
|---|---|
| `internal/platform/ai/module.go` | You need to see what this module depends on and what it exposes |
| `internal/platform/ai/contract/` | You are calling this module from another module |
| `internal/platform/ai/service/` | You are changing behaviour |
| `db/migrations/ai/` | You need the real schema |
<!-- END GENERATED: entrypoints -->

## 4. Public API (contract)

Other modules may import **only** `internal/platform/ai/contract`.

<!-- BEGIN GENERATED: contract -->
| Kind | Name | Purpose |
|---|---|---|
| interface | `ai.Client` | `Run(ctx, TaskRequest) (TaskResult, error)` and `Stream(ctx, TaskRequest) (iter, error)` — the only surface business code touches |
| struct | `ai.TaskRequest` | `{Task, Input (typed), UserID, Locale, IdempotencyKey}` — never a model name, never a raw prompt |
| struct | `ai.TaskResult` | `{Output (validated JSON), Provider, Model, PromptVersion, Tokens, CostUSD, CacheHit}` |
| interface | `ai.Provider` | The Strategy interface each adapter implements — internal, not for business code |

### Events

| Event | Direction | Payload summary |
|---|---|---|
| `ai.budget_threshold_reached` | publishes | `{scope, percent, date}` |
| `ai.provider_degraded` | publishes | `{provider, reason}` |
<!-- END GENERATED: contract -->

## 5. Database schema

<!-- BEGIN GENERATED: schema -->
All tables live in the `ai` schema and are owned exclusively by this module (rule DB1).
Migrations: `db/migrations/ai/` · Queries: `db/queries/ai/`

| Table | Purpose | Key columns / notes |
|---|---|---|
| `ai.ai_requests` | One row per LLM call | Partitioned monthly. `task`, `prompt_version`, `provider`, `model`, `status`, `attempt`, `fallback_from`, `cache_hit`, `latency_ms`, `trace_id`. Request/response bodies redacted and retained 30 days. |
| `ai.ai_usage` | Cost attribution | `user_id`, `task`, `tokens_in`, `tokens_out`, `cost_usd`, `occurred_on` — the source for per-user unit economics |
| `ai.prompt_versions` | Registry of deployed prompt versions | `task`, `version`, `status` (draft/shadow/active/deprecated), `checksum`, `activated_at` |
| `ai.ai_cache_entries` | Response cache index | `input_hash` UNIQUE, `task`, `prompt_version`, `embedding vector(1536)` for semantic lookup, `expires_at` |
| `ai.ai_budgets` | Daily spend accounting | `scope` (global/user), `date`, `spent_usd`, `cap_usd` |

<!-- END GENERATED: schema -->

## 6. HTTP endpoints

Full definitions are in [`api/openapi/openapi.yaml`](../../../api/openapi/openapi.yaml)
(tag: `ai`). See also [`API.md`](API.md).

<!-- BEGIN GENERATED: endpoints -->
| Method | Path | Permission | Purpose |
|---|---|---|---|
| `GET` | `/api/v1/admin/ai/usage` | `ai.read` | Spend and volume by task, provider and day |
| `GET` | `/api/v1/admin/ai/requests/{id}` | `ai.read` | Inspect one call for debugging |
| `GET` | `/api/v1/admin/ai/prompts` | `ai.read` | Deployed prompt versions and their status |
| `POST` | `/api/v1/admin/ai/prompts/{task}/activate` | `ai.manage` | Promote a prompt version |
| `GET` | `/api/v1/admin/ai/budget` | `ai.read` | Budget consumption today |
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
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: folders -->

## 8. Related modules

<!-- BEGIN GENERATED: related -->
| Module | Direction | Why |
|---|---|---|
| [`cache`](../../platform/cache/AGENT.md) | → depends on | Response cache, quota counters, provider circuit state |
| [`telemetry`](../../platform/telemetry/AGENT.md) | → depends on | Spans and metrics on every call |
| [`job`](../../platform/job/AGENT.md) | → depends on | Long-running calls execute inside jobs; evals run on a schedule |
| [`writing`](../../modules/writing/AGENT.md) | ← used by | consumes this module's contract |
| [`speaking`](../../modules/speaking/AGENT.md) | ← used by | consumes this module's contract |
| [`grammar`](../../modules/grammar/AGENT.md) | ← used by | consumes this module's contract |
| [`questionbank`](../../modules/questionbank/AGENT.md) | ← used by | consumes this module's contract |
| [`content`](../../modules/content/AGENT.md) | ← used by | consumes this module's contract |
| [`reading`](../../modules/reading/AGENT.md) | ← used by | consumes this module's contract |
| [`media`](../../platform/media/AGENT.md) | ← used by | consumes this module's contract |
| [`learning`](../../modules/learning/AGENT.md) | ← used by | consumes this module's contract |
<!-- END GENERATED: related -->

**Boundary reminder:** you may call these through their `contract` package only.
Reaching into `service/`, `repository/`, `domain/` or their tables violates rules L1/L2
and fails `go-arch-lint` in CI.

## 9. Business rules

<!-- BEGIN GENERATED: rules -->
1. **BR-AI-01** — Business code names a **task**, never a model or a provider. A task not present in the routing config is a startup error, not a runtime surprise.
2. **BR-AI-02** — Prompts are loaded from `docs/prompts/runtime/<task>/v<N>.md`; a published version is immutable, and the active version is pinned by configuration.
3. **BR-AI-03** — Learner-supplied text is inserted only inside the untrusted-content wrapper; it is never concatenated into the instruction block.
4. **BR-AI-04** — Output must validate against the task's JSON schema. One repair attempt is allowed; a second failure fails the call with `AI_OUTPUT_INVALID`.
5. **BR-AI-05** — Numeric outputs are clamped to the rubric's range and cross-checked for internal consistency before being returned.
6. **BR-AI-06** — Every call checks the per-user quota and the global daily budget first; exceeding either fails fast without contacting a provider.
7. **BR-AI-07** — Retries apply only to 429, 5xx and timeouts. A refusal, a schema failure, or a content-policy rejection is not retried against the same provider.
8. **BR-AI-08** — When a provider's circuit is open, the next provider in the task's fallback chain is used and the substitution is recorded.
9. **BR-AI-09** — No call blocks an HTTP request for more than 2 seconds; anything longer runs in a job and streams or notifies.
10. **BR-AI-10** — Every call writes `ai_requests` and `ai_usage`, including cache hits and failures — otherwise cost attribution has holes.
11. **BR-AI-11** — In tests, the `mock` provider is used. Unit and integration tests never reach a network.
12. **BR-AI-12** — A prompt version cannot be activated unless its eval suite meets its thresholds and is no worse than the currently active version.
<!-- END GENERATED: rules -->

## 10. Common tasks

<!-- BEGIN GENERATED: tasks -->
### Add a provider

1. Create `provider/<name>/` implementing `ai.Provider`. The vendor SDK may be imported **only** inside this directory.
2. Map vendor errors onto the module's retryable/non-retryable classification.
3. Add config keys and register the provider in the registry.
4. Add `mock` fixtures so tests covering this provider stay offline.
5. Add it to the fallback chains that should use it, in config.
6. Run the eval suites against it and record the results in `docs/ai/provider-comparison.md`.

### Add an AI task

1. Follow the checklist in `/AI_GUIDE.md` §B4 — it is the authoritative list.
2. Define the task in the routing config with tier, timeout, cache policy and budget.
3. Write `docs/prompts/runtime/<task>/v1.md` plus input and output schemas.
4. Build the golden set (≥ 30 examples) and `thresholds.yaml`, including red-team cases.
5. Add `mock` fixtures.
6. Call it from the owning module by task name only.

### Change the model for a task

1. Edit the routing config — **do not touch Go code**.
2. Run the eval suite; compare cost and quality deltas.
3. Shadow, then 10 %, then 100 %.
4. Record the change and its measured effect in `docs/ai/cost-model.md`.
<!-- END GENERATED: tasks -->

## 11. Known limitations

<!-- BEGIN GENERATED: limitations -->
- Semantic caching depends on `pgvector` and an embedding call, which itself costs money — it is enabled only for tasks where the hit rate justifies it.
- The repair attempt is a single retry with the validation error appended; there is no multi-turn negotiation.
- Streaming partials are persisted per chunk, which is chatty for long outputs.
- Cost figures are computed from a published price table in config and can drift from actual invoices; monthly reconciliation is manual.
<!-- END GENERATED: limitations -->

## 12. Coding conventions (module-specific)

Global rules: [`/CODING_STANDARD.md`](../../../CODING_STANDARD.md). Deviations and additions
for this module:

<!-- BEGIN GENERATED: conventions -->
- Vendor SDK types never leave `provider/<name>/`.
- No prompt string literal anywhere in Go code.
- Every provider adapter has a `mock` fixture set covering success, refusal, timeout, rate limit and malformed output.
<!-- END GENERATED: conventions -->

### Cache strategy

| Key | TTL | Invalidated by |
|---|---|---|
| `fluentra:{env}:ai:resp:{task}:{prompt_version}:{input_hash}:v1` | per task (see /PROMPT_LIBRARY.md §2.5) | Prompt version bump |
| `fluentra:{env}:ai:quota:{user_id}:{date}:v1` | until midnight in the user's timezone | Natural expiry |
| `fluentra:{env}:ai:budget:{date}:v1` | 48 h | Natural expiry |
| `fluentra:{env}:ai:breaker:{provider}:v1` | 30 s | Half-open probe |

### Error codes owned by this module

| Code | Status | Meaning |
|---|---|---|
| `AI_QUOTA_EXCEEDED` | 429 | Per-user daily limit reached |
| `AI_BUDGET_EXCEEDED` | 503 | Global daily spend cap reached; non-critical tasks are shed first |
| `AI_UNAVAILABLE` | 503 | Every provider in the chain failed |
| `AI_TIMEOUT` | 504 | Provider exceeded the task timeout |
| `AI_OUTPUT_INVALID` | 500 | Output failed schema validation after the repair attempt |
| `AI_CONTENT_FLAGGED` | 422 | Moderation blocked the input |
| `AI_OPTED_OUT` | 403 | The user disabled AI processing |
| `EVAL_THRESHOLD_NOT_MET` | 409 | Prompt promotion blocked by its eval suite |

### Security considerations

- Provider API keys live in configuration only and never appear in a span, a log, or an error.
- PII redaction runs before send; the redaction map is retained only long enough to restore names in the response.
- Only providers contractually barred from training on API data receive learner content; `openrouter` is restricted to non-personal input.
- The untrusted-content wrapper plus a hardened system preamble is the primary prompt-injection defence; the eval suite includes a red-team subset that must score 1.0.
- Score sanity bounds are the second line of defence: an out-of-range or internally inconsistent score fails the job rather than reaching the learner.

## 13. Testing

See [`TESTING.md`](TESTING.md) for the full plan.

<!-- BEGIN GENERATED: testing -->
Coverage target: **85% service, 90% domain**

```bash
go test ./internal/platform/ai/...                    # unit
go test -tags=integration ./internal/platform/ai/...  # integration (testcontainers)
```

**Focus areas**

- Fallback chain: killing provider 1 must produce a correct result from provider 2 with the substitution recorded
- Quota and budget enforcement, including the boundary at exactly the cap
- Schema validation and the single repair attempt
- Score clamping and consistency checks reject an out-of-range score
- Prompt injection: red-team fixtures must not change the output score
- Cache correctness: a different input must never return a cached response
- Cost accounting matches the token counts reported by the provider
- Streaming: a disconnect mid-stream does not lose already-generated content
<!-- END GENERATED: testing -->

## 14. Do NOT

<!-- BEGIN GENERATED: donot -->
- Do not import a provider SDK outside `provider/<name>/`.
- Do not name a model in business code — name a task.
- Do not inline a prompt string.
- Do not call an LLM synchronously in an HTTP handler for anything expected to take more than 2 seconds.
- Do not trust model output — validate the schema, clamp the numbers, check the consistency.
- Do not concatenate learner text into the instruction block.
- Do not skip the eval suite because "it's a small prompt change". That is exactly when regressions slip through.
- Do not use an LLM where an algorithm exists — read `/AI_GUIDE.md` §B2 first.
<!-- END GENERATED: donot -->

---

*Generated by `tools/docgen` from `tools/docgen/data/`. Hand-written text outside the
GENERATED markers is preserved. Update the manifest, then run `make docs`.*
