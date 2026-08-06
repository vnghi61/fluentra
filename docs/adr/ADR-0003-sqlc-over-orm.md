---
adr: 0003
title: "sqlc + pgx instead of an ORM"
status: Accepted
date: 2026-08-06
tags: [data]
---

# ADR-0003: sqlc + pgx instead of an ORM

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | data |

## Context

We need type-safe database access that is fast, transparent about the queries it runs, and — importantly for this project — reliably writable by AI coding assistants.

## Decision

Write SQL in `db/queries/<module>/*.sql`, generate typed Go with `sqlc`, and execute through `pgx` v5.

## Alternatives considered

### A. GORM

| | |
|---|---|
| **Pros** | Fast for simple CRUD; large community |
| **Cons** | Hidden queries, N+1 by default, struct tags encode behaviour, performance is hard to reason about, migrations drift from the model |
| **Why rejected** | Opacity is the problem: you cannot review a query you cannot see, and the failure mode is a production performance cliff. |

### B. ent

| | |
|---|---|
| **Pros** | Excellent type safety; graph traversal API |
| **Cons** | Heavy code generation, its own schema language, steep learning curve, awkward for hand-tuned SQL |
| **Why rejected** | Another DSL to learn and, for an AI agent, another language to hallucinate in. |

### C. sqlx

| | |
|---|---|
| **Pros** | Thin, close to SQL |
| **Cons** | Type safety only at runtime; a renamed column fails in production, not at compile time |
| **Why rejected** | sqlc gives the same transparency with compile-time checking, for the cost of a codegen step. |

## Consequences

### Positive

- A wrong column, type or arity is a compile error
- `EXPLAIN` works on exactly the SQL that runs
- Full access to Postgres features: CTEs, window functions, `jsonb`, partial indexes
- AI agents write SQL far more reliably than ORM DSL — this is the highest-leverage choice in the stack for AI-assisted work
- Reviewers can read the query in the diff

### Negative — accepted knowingly

- More boilerplate for trivial CRUD
- A codegen step in the build, which CI must verify is current
- Dynamic filtering needs explicit query variants rather than a builder

## Compliance

String-concatenated SQL fails `golangci-lint`. CI runs `make gen` and fails if the generated code differs.

## Revisit when

If a use case genuinely requires runtime-composed queries across many optional filters — at which point a narrowly scoped query builder for that one case, not an ORM everywhere.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
