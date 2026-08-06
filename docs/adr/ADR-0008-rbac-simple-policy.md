---
adr: 0008
title: "Table-driven permissions, no Casbin or OPA"
status: Accepted
date: 2026-08-06
tags: [security]
---

# ADR-0008: Table-driven permissions, no Casbin or OPA

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | security |

## Context

The product has exactly two roles and roughly forty named permissions. Authorization must be deny-by-default and enforced at the service layer as well as the route.

## Decision

Roles, permissions and their mapping live in four database tables. A `rbac.Require(ctx, permission)` guard is called in service methods. Ownership filtering happens in each module's queries. No policy engine, no policy language.

## Alternatives considered

### A. Casbin

| | |
|---|---|
| **Pros** | Mature; supports RBAC, ABAC, resource scoping |
| **Cons** | A policy DSL to learn and debug; model file becomes a second source of truth; overkill for two roles |
| **Why rejected** | The complexity is not repaid until we need resource-scoped or hierarchical policies. |

### B. OPA / Rego

| | |
|---|---|
| **Pros** | Extremely expressive; policy as code |
| **Cons** | A separate language and evaluation model; latency or an embedded engine; a large conceptual jump for the team |
| **Why rejected** | Same reasoning, more so. |

### C. Hard-coded role checks

| | |
|---|---|
| **Pros** | Trivial |
| **Cons** | `if role == "admin"` scattered everywhere; adding a role means finding every check |
| **Why rejected** | Named permissions cost almost nothing now and make a future role a data change. |

## Consequences

### Positive

- Readable and debuggable with a SQL query
- Named permissions mean a third role would be data, not code
- No new language for humans or agents
- Fast, cached, no external dependency

### Negative — accepted knowingly

- No resource-scoped permissions ("can edit *these* lessons")
- No time-bounded grants
- We own the evaluation logic, including its tests

## Compliance

CI checks that every non-public OpenAPI operation declares `x-permission` and that the handler enforces it. A permission written as a string literal at a call site fails review.

## Revisit when

If resource-scoped permissions become a requirement — content ownership per author is the likely trigger. That is when Casbin earns its keep.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
