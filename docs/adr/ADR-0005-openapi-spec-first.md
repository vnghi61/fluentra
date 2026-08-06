---
adr: 0005
title: "OpenAPI 3.1, spec-first"
status: Accepted
date: 2026-08-06
tags: [api]
---

# ADR-0005: OpenAPI 3.1, spec-first

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | api |

## Context

The HTTP contract is consumed by the Go server, the TypeScript client, the MSW mocks, the contract tests and the documentation. Any of these drifting from the others produces bugs that are only found in integration.

## Decision

`api/openapi/openapi.yaml` is the single source of truth. Server interfaces are generated with `oapi-codegen`, TypeScript types with `openapi-typescript`, and mocks from the spec's examples. Editing the spec is the first step of any API change (rule L10).

## Alternatives considered

### A. Code-first with huma

| | |
|---|---|
| **Pros** | One place to change; spec always matches code |
| **Cons** | The spec cannot be reviewed or agreed before implementation; frontend work must wait for backend code |
| **Why rejected** | We want the contract reviewable and parallelisable, and we want an AI agent's first edit to be the contract, not the handler. |

### B. Comment annotations (swaggo)

| | |
|---|---|
| **Pros** | Low ceremony |
| **Cons** | OpenAPI 3.0 only, drifts silently, no compile-time link between comment and code |
| **Why rejected** | Drift is the exact failure we are trying to eliminate. |

### C. No spec

| | |
|---|---|
| **Pros** | Nothing to maintain |
| **Cons** | Types hand-written twice; mocks hand-written a third time; all three diverge |
| **Why rejected** | Not viable with a separate frontend. |

## Consequences

### Positive

- Frontend and backend can proceed in parallel from an agreed contract
- Generated types make a breaking change a compile error on both sides
- MSW handlers generated from spec examples cannot drift from the API
- An AI agent cannot invent an endpoint — the generated interface will not compile

### Negative — accepted knowingly

- YAML editing is less pleasant than Go
- A codegen step CI must verify
- The spec file grows large and needs splitting into `components/`

## Compliance

CI runs `spectral` on the spec, regenerates code and fails on a diff, and runs contract tests asserting handlers match the spec.

## Revisit when

If spec maintenance visibly slows delivery without preventing proportionate bugs.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
