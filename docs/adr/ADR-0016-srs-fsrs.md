---
adr: 0016
title: "FSRS instead of SM-2 for spaced repetition"
status: Accepted
date: 2026-08-06
tags: [learning]
---

# ADR-0016: FSRS instead of SM-2 for spaced repetition

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | learning |

## Context

Vocabulary and grammar retention is the core learning mechanism. The scheduling algorithm determines how much of a learner's time produces durable memory.

## Decision

Implement FSRS (Free Spaced Repetition Scheduler) with four grades, in pure functions in `srs/domain/`, with globally tuned parameters initially and per-learner optimisation later.

## Alternatives considered

### A. SM-2 (the Anki classic)

| | |
|---|---|
| **Pros** | Simple; well documented; easy to implement |
| **Cons** | Ease-factor model does not represent memory well; over-reviews easy material and under-reviews hard material |
| **Why rejected** | Published comparisons consistently show FSRS reaching equivalent retention with materially fewer reviews. That difference is learner time. |

### B. A fixed Leitner box schedule

| | |
|---|---|
| **Pros** | Trivial to implement and explain |
| **Cons** | Ignores individual item difficulty and learner history entirely |
| **Why rejected** | Adequate for a hobby app, not for a product whose value proposition is efficient learning. |

### C. Build our own model

| | |
|---|---|
| **Pros** | Tailored to our data |
| **Cons** | A research project requiring data we do not yet have |
| **Why rejected** | FSRS is published, validated and free; our review logs will let us tune it later. |

## Consequences

### Positive

- Explicit stability and difficulty modelling per item and learner
- Roughly 20–30 % fewer reviews for equivalent retention
- Published parameters give a good starting point without our own data
- Pure functions make it provably correct with property-based tests

### Negative — accepted knowingly

- More complex than SM-2; the parameters are not independent knobs
- Per-learner optimisation needs several hundred reviews per learner
- Team must understand the model before changing it — hence `docs/knowledge/fsrs.md`

## Compliance

Scheduling logic performing I/O fails review. Property-based tests assert the algorithm's invariants. Parameter changes require simulation against historical logs.

## Revisit when

If a materially better published algorithm appears, or if our own review data supports a custom model.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
