---
adr: 0007
title: "JWT access tokens with rotating refresh tokens"
status: Accepted
date: 2026-08-06
tags: [security]
---

# ADR-0007: JWT access tokens with rotating refresh tokens

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | security |

## Context

We need stateless request authentication that does not require a database round trip per request, combined with a revocation story that works when a token is stolen.

## Decision

Short-lived JWT access tokens (15 minutes) carried in the `Authorization` header and held only in browser memory. Long-lived opaque refresh tokens (30 days) stored hashed, single-use and rotating, grouped by a `family_id`, delivered in an `HttpOnly; Secure; SameSite=Lax` cookie scoped to `/api/v1/auth`. Reuse of a spent refresh token revokes the entire family and raises a security event.

## Alternatives considered

### A. Server-side sessions only

| | |
|---|---|
| **Pros** | Immediate revocation; simplest mental model |
| **Cons** | A datastore read on every request; session affinity considerations |
| **Why rejected** | Rejected mainly on the per-request read; the 15-minute window plus a denylist gives adequate revocation. |

### B. Long-lived JWTs without refresh

| | |
|---|---|
| **Pros** | Simplest client |
| **Cons** | No practical revocation; a stolen token is valid for its whole lifetime |
| **Why rejected** | Unacceptable for an application holding personal learning data. |

### C. Non-rotating refresh tokens

| | |
|---|---|
| **Pros** | Simpler |
| **Cons** | A stolen refresh token is usable indefinitely and undetectably |
| **Why rejected** | Rotation with reuse detection is what turns theft from silent into visible. |

## Consequences

### Positive

- No database read on the authentication path
- Theft of a refresh token is *detectable*, because the legitimate client will eventually replay a spent token
- Revocation window bounded at 15 minutes; explicit logout is immediate via the denylist
- Access token never touches `localStorage`, so XSS cannot exfiltrate it

### Negative — accepted knowingly

- More moving parts than plain sessions
- A revoked user can act for up to 15 minutes unless the denylist is consulted
- Refresh races need single-flight handling in the client
- Key rotation requires supporting two active signing keys

## Compliance

Integration tests assert family revocation on reuse, timing equalisation on login, and that access tokens are never persisted client-side.

## Revisit when

When the system is split into services — at that point `HS256` becomes `EdDSA` with a JWKS endpoint.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
