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

# ai

The single door through which every LLM call passes. Business code asks for an **AI task** by name; this module decides which provider and model serve it, renders the versioned prompt, enforces quota and budget, caches, retries, falls back, validates the structured output, and records cost and usage.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
AI grading is the product's differentiator and its largest variable cost. Both facts demand that model routing, spend, and output quality be governed centrally and measurably rather than scattered through feature code.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
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
<!-- END GENERATED: readme-resp -->

## Where things are

<!-- BEGIN GENERATED: readme-folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: readme-folders -->

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**PLANNED** — planned for delivery phase 3. See [/ROADMAP.md](../../../ROADMAP.md).
