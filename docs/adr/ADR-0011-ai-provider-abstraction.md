---
adr: 0011
title: "Task-based AI provider abstraction"
status: Accepted
date: 2026-08-06
tags: [ai]
---

# ADR-0011: Task-based AI provider abstraction

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | ai |

## Context

LLM providers change pricing, deprecate models and suffer outages. Business code must not encode any of that, and cost and quality must be governable in one place.

## Decision

Business code calls `ai.Client.Run(ctx, TaskRequest{Task: "writing.grade_essay", ...})`. The platform module resolves the task to a model tier and a concrete model from configuration, renders the pinned prompt version, enforces quota and budget, caches, retries, falls back across providers, validates the output schema and records usage. Provider SDKs may be imported only inside `provider/<name>/`.

## Alternatives considered

### A. Call provider SDKs directly

| | |
|---|---|
| **Pros** | Least indirection |
| **Cons** | Vendor lock-in in every feature; no central cost control, caching, fallback or usage tracking; swapping a model is a code change everywhere |
| **Why rejected** | It puts the most volatile external dependency in the most places. |

### B. A thin `Complete(prompt)` wrapper

| | |
|---|---|
| **Pros** | Simple |
| **Cons** | Callers still choose the model and own the prompt; no routing policy, no per-task budgets or cache policies |
| **Why rejected** | It abstracts the wrong thing — the transport rather than the decision. |

### C. LangChain-style framework

| | |
|---|---|
| **Pros** | Many building blocks |
| **Cons** | Large surface, opinionated abstractions, Go support is weaker than Python |
| **Why rejected** | We need six well-understood tasks, not a general orchestration framework. |

## Consequences

### Positive

- Changing a model is a configuration change plus an eval run
- Cost, quota, caching, fallback and usage tracking exist in exactly one place
- Tests use a `mock` provider — deterministic, offline, free
- A provider outage degrades rather than fails

### Negative — accepted knowingly

- Indirection between the caller and the model
- The routing configuration becomes an important artefact in its own right
- Provider-specific capabilities must be surfaced deliberately or they are unavailable

## Compliance

An import of a provider SDK outside `internal/platform/ai/provider/<name>/` fails `go-arch-lint`. A prompt string literal in Go fails review.

## Revisit when

If a provider-specific capability becomes essential and cannot be expressed through the task abstraction.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
