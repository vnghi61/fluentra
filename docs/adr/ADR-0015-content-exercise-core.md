---
adr: 0015
title: "Shared content model and exercise engine for all skill modules"
status: Accepted
date: 2026-08-06
tags: [architecture]
---

# ADR-0015: Shared content model and exercise engine for all skill modules

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | architecture |

## Context

The original plan had six skill modules — vocabulary, grammar, reading, listening, speaking, writing — each owning its own items, attempts and progress. Inspection showed roughly 70 % of that would be identical.

## Decision

Extract two shared pieces. `content` owns items, immutable versions, media and taxonomy. `learning` owns the exercise engine: attempt lifecycle, idempotent submission, grader dispatch, scoring, progress rollup and event emission. Each skill module implements only `learning.ExerciseGrader` plus its genuinely skill-specific data.

## Alternatives considered

### A. Six independent skill modules

| | |
|---|---|
| **Pros** | Complete independence; no shared abstraction to get wrong |
| **Cons** | ~18 near-identical tables; six copies of the attempt lifecycle; six places to fix the same idempotency bug; mixed-skill lessons become very hard |
| **Why rejected** | The duplication is not incidental — it is the majority of the code, and copies diverge. |

### B. One monolithic learning module

| | |
|---|---|
| **Pros** | No cross-module calls at all |
| **Cons** | A single package containing every skill's logic; unclear ownership; a change to speaking risks reading |
| **Why rejected** | It trades one kind of duplication for a loss of boundaries. |

## Consequences

### Positive

- Estimated 40 % less backend code and 55 % fewer tables across the six skills
- Attempt lifecycle correctness is implemented and tested once
- Mixed-skill lessons are natural
- A seventh skill is a grader, not a module rebuild

### Negative — accepted knowingly

- The shared abstractions must be right; getting `ExerciseGrader` wrong would be expensive to change later
- A skill with a genuinely unusual shape may strain the model
- `learning` becomes a large and important module

## Compliance

A skill module that defines its own attempt table fails review. The grader registry is validated at startup.

## Revisit when

If a skill genuinely cannot express itself through `ExerciseGrader` — the response would be to widen the interface deliberately, not to fork the engine.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
