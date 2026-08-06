---
adr: 0002
title: "chi + stdlib net/http for the HTTP layer"
status: Accepted
date: 2026-08-06
tags: [backend]
---

# ADR-0002: chi + stdlib net/http for the HTTP layer

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | backend |

## Context

We need routing, middleware, and route groups that map onto module boundaries, with instrumentation that works out of the box and no framework types leaking into handler signatures.

## Decision

Use `go-chi/chi` v5 on top of the standard library's `net/http`. Handlers are `http.HandlerFunc`; each module exposes a `chi.Router` that `cmd/api` mounts under a prefix.

## Alternatives considered

### A. Gin or Echo

| | |
|---|---|
| **Pros** | Batteries included: binding, validation, rendering |
| **Cons** | Custom `Context` type in every handler signature; framework lock-in; their binding hides validation we want explicit |
| **Why rejected** | The framework `Context` propagates into every layer boundary we touch and makes handlers non-portable. |

### B. Fiber

| | |
|---|---|
| **Pros** | Very fast benchmarks |
| **Cons** | Built on fasthttp, not `net/http`-compatible; loses otelhttp, httptest and the wider middleware ecosystem |
| **Why rejected** | Incompatibility with the standard interface costs us more than the benchmark gains, which are irrelevant at our latency budget. |

### C. Bare `net/http.ServeMux` (Go 1.22+)

| | |
|---|---|
| **Pros** | Zero dependencies; pattern matching is now decent |
| **Cons** | No route groups, no middleware chaining, no `Mount` |
| **Why rejected** | We would end up writing chi, worse. |

## Consequences

### Positive

- Every `net/http` tool works unmodified, including `otelhttp` and `httptest`
- `Mount` maps cleanly onto module boundaries
- Route patterns give low-cardinality metric labels for free
- ~1k lines of dependency, easily audited

### Negative — accepted knowingly

- We supply binding, validation and rendering ourselves
- Slightly more code per handler than a batteries-included framework

## Compliance

A handler whose signature is not `func(http.ResponseWriter, *http.Request)` fails review.

## Revisit when

If the boilerplate of manual binding becomes a measurable drag — at which point `huma` on top of chi is the likely answer, not a different router.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
