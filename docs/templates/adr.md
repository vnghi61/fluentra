---
adr: NNNN
title: "<Short imperative title>"
status: Proposed
date: YYYY-MM-DD
tags: [architecture|backend|frontend|data|api|security|ai|observability|testing]
---

# ADR-NNNN: <Title>

| | |
|---|---|
| **Status** | Proposed / Accepted / Rejected / Deprecated / Superseded by ADR-XXXX |
| **Date** | YYYY-MM-DD |
| **Deciders** | <names> |
| **Tags** | <tags> |

## Context

What forces are at play? What problem are we solving? What constraints apply — team size,
timeline, existing decisions, external requirements?

Write this so that someone joining in two years understands the situation *without* already
knowing the answer. Do not argue for the decision here; describe the world it was made in.

## Decision

What we will do, in the active voice, in as few sentences as it takes to be unambiguous.

State the mechanism, not just the intent: "use X, enforced by Y" rather than "prefer X".

## Alternatives considered

> At least two. An ADR with no rejected alternatives is a description, not a decision.

### A. <Alternative>

| | |
|---|---|
| **Pros** | … |
| **Cons** | … |
| **Why rejected** | The specific reason it loses *for us*, given the context above. Not a generic criticism. |

### B. <Alternative>

| | |
|---|---|
| **Pros** | … |
| **Cons** | … |
| **Why rejected** | … |

## Consequences

### Positive

- …

### Negative — accepted knowingly

- …

> Be honest here. An ADR that lists only benefits is marketing. The negatives are what makes
> this document useful later, when someone hits one of them and needs to know it was expected.

### What this makes harder later

- …

## Compliance

How will we know this decision is actually being followed? Name the mechanism:

- a lint rule, a CI job, a test, a review checklist item
- if there is no mechanism, say so — an unenforced decision will decay

## Revisit when

The concrete, observable condition under which this should be reconsidered. Not "if it becomes a
problem" — something you could put on a dashboard or check in a retro.

---

*Index: [/DECISIONS.md](../../DECISIONS.md)*
