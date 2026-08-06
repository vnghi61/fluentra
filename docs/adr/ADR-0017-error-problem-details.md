---
adr: 0017
title: "RFC 9457 Problem Details for all API errors"
status: Accepted
date: 2026-08-06
tags: [api]
---

# ADR-0017: RFC 9457 Problem Details for all API errors

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | api |

## Context

Errors cross four boundaries: domain to service, service to transport, API to client, client to user. Without one shape, each boundary invents its own.

## Decision

One `shared/apperr.Error` type with a kind, a stable machine `code`, a safe message, optional field errors and metadata, and non-exposed cause and internal detail. Rendered as `application/problem+json` per RFC 9457. Clients branch on `code`, never on message text.

## Alternatives considered

### A. A custom error envelope

| | |
|---|---|
| **Pros** | Exactly our shape |
| **Cons** | No tooling understands it; every client integration re-learns it |
| **Why rejected** | A standard costs nothing here and buys interoperability. |

### B. Plain HTTP status codes with a text body

| | |
|---|---|
| **Pros** | Simplest |
| **Cons** | A client cannot distinguish two different 409s; localisation is impossible |
| **Why rejected** | Insufficient for a UI that must respond differently to different conflicts. |

### C. GraphQL-style errors in a 200 response

| | |
|---|---|
| **Pros** | Uniform transport |
| **Cons** | Breaks HTTP caching, monitoring and every intermediary's error handling |
| **Why rejected** | Fights the protocol. |

## Consequences

### Positive

- Clients branch on stable codes, so messages can change and be localised freely
- Internal details cannot leak — the type separates exposed from internal fields
- One rendering path means one place to get security right
- Field-level validation errors map directly onto form fields

### Negative — accepted knowingly

- Every error needs a code, which is a small ongoing discipline
- Codes are public API and cannot be changed once released
- The catalogue must be maintained in `ERROR_HANDLING.md`

## Compliance

Returning an unclassified error to HTTP produces a generic 500 and an error log — visible in monitoring. Contract tests assert the error shape for each documented failure.

## Revisit when

Unlikely. This is a standard doing exactly what standards are for.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
