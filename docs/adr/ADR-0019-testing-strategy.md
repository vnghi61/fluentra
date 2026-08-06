---
adr: 0019
title: "Testcontainers over mocked infrastructure"
status: Accepted
date: 2026-08-06
tags: [testing]
---

# ADR-0019: Testcontainers over mocked infrastructure

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | testing |

## Context

Repository code is mostly SQL. The bugs that reach production in data-access code are SQL bugs — wrong join, missing filter, wrong index assumption — none of which a mocked database can detect.

## Decision

Unit-test domain and service layers with mocked *ports* (repository and contract interfaces). Integration-test repositories against real PostgreSQL, Redis and MinIO started by `testcontainers-go`, with a container per package and a template-database clone per test.

## Alternatives considered

### A. Mock the database

| | |
|---|---|
| **Pros** | Fast; no Docker |
| **Cons** | Proves the mock works, not the SQL; every real SQL bug survives |
| **Why rejected** | It tests the wrong thing precisely where the risk is. |

### B. A shared CI database

| | |
|---|---|
| **Pros** | Fast startup |
| **Cons** | Test pollution; ordering dependencies; cannot run in parallel; different behaviour locally and in CI |
| **Why rejected** | Flakiness and environment divergence. |

### C. SQLite in tests

| | |
|---|---|
| **Pros** | Fast, embedded |
| **Cons** | Different SQL dialect, no `jsonb`, no partial indexes, no partitioning — we would test a database we do not run |
| **Why rejected** | The differences are exactly where our queries live. |

## Consequences

### Positive

- SQL correctness is actually verified
- Identical behaviour locally and in CI
- Redis degradation and MinIO presigning are exercised for real
- Migrations are tested on every integration run

### Negative — accepted knowingly

- Slower than mocks (minutes, not seconds) — hence the build tag and a separate `make test-int`
- Docker required for the full suite
- Container startup adds CI time, mitigated by per-package reuse

## Compliance

Integration tests carry `//go:build integration`. A repository with no integration test fails review.

## Revisit when

If integration suite runtime exceeds roughly ten minutes and becomes a delivery drag — the remedy is better parallelism and template databases, not mocks.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
