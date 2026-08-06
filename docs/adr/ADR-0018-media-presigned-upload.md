---
adr: 0018
title: "Presigned direct-to-storage uploads"
status: Accepted
date: 2026-08-06
tags: [architecture]
---

# ADR-0018: Presigned direct-to-storage uploads

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | architecture |

## Context

Learners upload voice recordings and avatars. Recordings can be several megabytes and arrive in bursts during practice sessions.

## Decision

The API never handles file bytes. It issues a presigned PUT URL with a pinned content type, maximum size and five-minute expiry; the browser uploads directly to MinIO; the API then verifies the object before accepting the reference.

## Alternatives considered

### A. Proxy uploads through the API

| | |
|---|---|
| **Pros** | Full control; simpler client |
| **Cons** | API memory and bandwidth scale with file size and concurrency; a burst of uploads degrades every other request |
| **Why rejected** | It couples the scalability of the whole API to the largest thing a user can send. |

### B. Base64 in a JSON body

| | |
|---|---|
| **Pros** | One request |
| **Cons** | 33 % size inflation; large request bodies; no resumability |
| **Why rejected** | Strictly worse in every dimension. |

## Consequences

### Positive

- API memory stays flat regardless of upload volume
- Uploads scale with the object store, not the application
- A CDN can serve downloads directly
- Resumable and multipart uploads become possible without API changes

### Negative — accepted knowingly

- More client-side steps (intent, upload, confirm)
- Post-upload verification is mandatory — the client cannot be trusted about what it uploaded
- Presigned URLs are sensitive to clock skew

## Compliance

Streaming file bytes through a handler fails review. Every upload path has a test asserting post-upload verification rejects a mismatched object.

## Revisit when

Only if we move to a storage backend without presigning support, which would itself be a questionable choice.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
