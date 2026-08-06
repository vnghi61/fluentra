---
adr: 0004
title: "One PostgreSQL schema per module"
status: Accepted
date: 2026-08-06
tags: [data]
---

# ADR-0004: One PostgreSQL schema per module

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | data |

## Context

Modules must own their data so that boundaries mean something, but running a database per module in v1 would multiply operational cost for no benefit at our scale.

## Decision

One PostgreSQL instance; one schema per module tier (`core`, `learn`, `skill`, `assess`, `content`, `comm`, `billing`, `ai`, `ops`, `audit`, `analytics`). A module reads and writes only its own tables. Cross-schema foreign keys are forbidden, with one documented exception: `→ core.users(id)`.

## Alternatives considered

### A. One shared schema

| | |
|---|---|
| **Pros** | Simplest; joins anywhere |
| **Cons** | No ownership signal; any module can couple to any table; extraction later is archaeology |
| **Why rejected** | It makes the boundary unenforceable in exactly the place it matters most. |

### B. A database per module

| | |
|---|---|
| **Pros** | Hard isolation; extraction is trivial |
| **Cons** | N connection pools, N backup jobs, N migration pipelines, no cross-module transactions even where they would be legitimate |
| **Why rejected** | Operational cost now for a benefit we do not need until extraction, which the schema boundary already prepares for. |

## Consequences

### Positive

- Ownership is visible in every query
- Extraction means moving a schema, not untangling tables
- Per-schema permissions are possible
- One instance to operate, back up and monitor

### Negative — accepted knowingly

- Discipline is required — Postgres will happily join across schemas
- The `core.users` exception is a real coupling that must be handled at extraction time

## Compliance

Migrations live in `db/migrations/<module>/`; a migration touching another module's schema fails review. Cross-schema joins are caught in review and by a query linter.

## Revisit when

At extraction: the extracted module's schema becomes its own database, and the `core.users` foreign key becomes a local `user_id` maintained by events.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
