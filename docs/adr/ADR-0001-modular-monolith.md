---
adr: 0001
title: "Modular monolith over microservices"
status: Accepted
date: 2026-08-06
tags: [architecture]
---

# ADR-0001: Modular monolith over microservices

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | architecture |

## Context

We are a team of 2–6 engineers building an English learning platform with an unstable domain, a single deployment target, and no organisational pressure to split ownership. The system must nevertheless be modifiable in bounded pieces, understandable by AI coding assistants without loading the whole repository, and capable of scaling out later.

## Decision

Build a single Go application (plus a worker binary) internally partitioned into modules with enforced boundaries. Modules communicate only through explicit `contract` packages and an event bus. Boundaries are enforced by `go-arch-lint` in CI, not by convention.

## Alternatives considered

### A. Microservices from day one

| | |
|---|---|
| **Pros** | Independent deploy and scale; hard boundaries; team autonomy |
| **Cons** | Distributed transactions, network failure modes, 10× operational surface, slower feature delivery, painful local development |
| **Why rejected** | We have none of the problems microservices solve and all of the costs. At 2–6 engineers the coordination overhead alone would dominate. |

### B. Unstructured monolith

| | |
|---|---|
| **Pros** | Fastest to start; no boundary ceremony |
| **Cons** | Boundaries erode; every change risks everything; AI agents cannot scope their reading; extraction later is a rewrite |
| **Why rejected** | The erosion is not hypothetical — it is the default outcome, and it is what makes later change expensive. |

### C. Serverless functions

| | |
|---|---|
| **Pros** | Scale to zero; no server management |
| **Cons** | Cold starts on a latency-sensitive path; awkward with long-running AI grading and ffmpeg; vendor lock-in |
| **Why rejected** | Our workload has long-running CPU-heavy jobs that fit poorly, and the cost model is worse at steady traffic. |

## Consequences

### Positive

- One repository, one deploy, one debugger
- In-process calls: no network failure modes between modules
- Boundaries are compiler- and CI-enforced, so they hold under deadline pressure
- AI agents read one module's `AGENT.md` (~4k tokens) instead of scanning the repo (~200k)
- Refactoring across a boundary is a normal refactor, not a cross-team negotiation

### Negative — accepted knowingly

- One deployment unit: a change anywhere redeploys everything
- Scaling is all-or-nothing until a module is extracted
- A memory leak or panic in one module affects the whole process
- Boundary discipline requires active enforcement — without the linter it would decay

## Compliance

`.go-arch-lint.yml` declares the allowed dependency graph; CI fails on a violation. `MODULE_INDEX.md` §3 and the linter config must agree, checked by the `dep-drift` job.

## Revisit when

When any trigger in `ARCHITECTURE.md` §20.1 fires: a module needs independent scaling, a different runtime, an isolation boundary for compliance, or more than three teams contend on the same deploy.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
