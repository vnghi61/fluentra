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

# ai — TODO

Ordered backlog. Every item states what "done" means. Keep this current — it is how the next
agent knows what is already handled and what is deliberately deferred.

<!-- BEGIN GENERATED: todo -->
## Phase 3 — foundation

- [ ] Provider interface, registry and the `mock` provider
- [ ] Anthropic and OpenAI adapters
- [ ] Routing config with tiers, timeouts and budgets
- [ ] Prompt registry loading versioned templates with schemas
- [ ] Resilience chain: timeout, retry, breaker, fallback
- [ ] Quota and budget enforcement
- [ ] `ai_requests` / `ai_usage` recording and the cost dashboard
- [ ] Output schema validation, repair, clamping
- [ ] Untrusted-content wrapper and PII redaction
- [ ] Eval harness and the CI gate

## Phase 3 — extensions

- [ ] Gemini and OpenRouter adapters
- [ ] Exact-hash response cache
- [ ] SSE streaming with partial persistence
- [ ] Semantic cache with pgvector
<!-- END GENERATED: todo -->

## Deferred (deliberately not doing yet)

<!-- BEGIN GENERATED: todo-deferred -->
_Nothing deferred._
<!-- END GENERATED: todo-deferred -->

## Future improvements

<!-- BEGIN GENERATED: todo-future -->
- Automatic model selection from measured quality-per-dollar
- Per-learner model preference for feedback style
- Batch API usage for non-urgent tasks
- Self-hosted small model for high-volume tasks
<!-- END GENERATED: todo-future -->
