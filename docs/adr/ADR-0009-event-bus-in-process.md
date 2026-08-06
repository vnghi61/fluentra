---
adr: 0009
title: "In-process event bus with a transactional outbox"
status: Accepted
date: 2026-08-06
tags: [architecture]
---

# ADR-0009: In-process event bus with a transactional outbox

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | architecture |

## Context

Modules must react to each other's events without calling each other synchronously, and an event must never be lost or emitted for a transaction that rolled back.

## Decision

An in-process publish/subscribe bus, fed by a transactional outbox. Publishers write the event row inside the business transaction; a publisher loop polls unpublished rows with `FOR UPDATE SKIP LOCKED` and dispatches after commit. Consumers are idempotent because delivery is at-least-once.

## Alternatives considered

### A. Direct synchronous calls between modules

| | |
|---|---|
| **Pros** | Simple; immediately consistent |
| **Cons** | Tight coupling; a slow or failing consumer breaks the publisher; fan-out becomes a distributed transaction problem |
| **Why rejected** | Notification failing must not fail essay grading. |

### B. A message broker (NATS/Kafka) now

| | |
|---|---|
| **Pros** | Real durability and decoupling; ready for extraction |
| **Cons** | New infrastructure; still needs an outbox to be transactional; operational burden today for a benefit at extraction time |
| **Why rejected** | The outbox is the part that makes it correct, and we can have that without the broker. |

### C. Publishing after commit, without an outbox

| | |
|---|---|
| **Pros** | No extra table |
| **Cons** | A crash between commit and publish loses the event silently |
| **Why rejected** | Silent loss is the worst failure mode available. |

## Consequences

### Positive

- No event is emitted for rolled-back data, and no committed data lacks its event
- No new infrastructure
- The bus interface is broker-shaped, so swapping in NATS later is an adapter change
- Consumers are already idempotent, which is a prerequisite for any broker

### Negative — accepted knowingly

- Polling introduces a small publish latency (sub-second in practice)
- In-process delivery means a slow consumer occupies a worker
- Ordering is per-aggregate, not global
- Outbox rows add write volume to the primary database

## Compliance

Publishing outside a transaction fails review. Every consumer has a test asserting idempotency under duplicate delivery.

## Revisit when

When the first module is extracted, or when outbox lag becomes a real constraint.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
