---
adr: 0006
title: "Manual constructor injection, no DI framework"
status: Accepted
date: 2026-08-06
tags: [backend]
---

# ADR-0006: Manual constructor injection, no DI framework

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | backend |

## Context

Thirty modules need wiring. Options range from a reflective container to compile-time code generation to writing it by hand.

## Decision

Wire everything by hand in `cmd/api/main.go` and `cmd/worker/main.go` using plain constructors. No wire, no fx, no dig, no service locator, no global state.

## Alternatives considered

### A. google/wire

| | |
|---|---|
| **Pros** | Compile-time, no reflection |
| **Cons** | Generated code is hard to read; a wiring error produces a confusing generator message; another tool in the build |
| **Why rejected** | It solves a problem — wiring tedium — that is not actually painful at 30 constructors, and it makes the dependency graph less legible. |

### B. uber/fx or dig

| | |
|---|---|
| **Pros** | Handles complex graphs and lifecycles |
| **Cons** | Runtime reflection; failures at startup rather than compile time; the graph is invisible in the source |
| **Why rejected** | The explicit graph in `main.go` is documentation; hiding it behind reflection removes the clearest artefact we have. |

## Consequences

### Positive

- The entire dependency graph is readable in one file, in order
- A missing dependency is a compile error
- No magic for a newcomer — human or agent — to learn
- Test wiring uses the same constructors with fakes

### Negative — accepted knowingly

- `main.go` grows to several hundred lines
- Adding a module means editing the composition root (which is arguably a feature: it makes the addition visible)

## Compliance

A global variable holding a dependency, or an `init()` performing wiring, fails review.

## Revisit when

If `main.go` exceeds roughly 500 lines and becomes genuinely hard to follow — the first remedy is per-tier wiring functions, not a framework.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
