---
adr: 0020
title: "AGENT.md per module as the unit of AI context"
status: Accepted
date: 2026-08-06
tags: [ai]
---

# ADR-0020: AGENT.md per module as the unit of AI context

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | ai |

## Context

This project is built with AI coding assistants. Their most common failure modes are reading too much context, reading the wrong context, and inventing things that do not exist. All three are addressable by how the repository is organised.

## Decision

Every module carries an `AGENT.md` with fourteen fixed sections in a fixed order, plus machine-readable YAML front-matter. A root `AGENT.md` is the single entry point; `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` and the Copilot instructions are thin pointers to it. Drift is caught by CI.

## Alternatives considered

### A. One large root instruction file

| | |
|---|---|
| **Pros** | Single place to maintain |
| **Cons** | Either too long to load usefully or too shallow to answer module-specific questions |
| **Why rejected** | Context budget is the constraint; a 40k-token file defeats the purpose. |

### B. Rely on code comments and good naming

| | |
|---|---|
| **Pros** | No extra artefacts |
| **Cons** | Cannot express business rules, ownership, boundaries or 'do not do this'; an agent still has to read everything to find out |
| **Why rejected** | Comments describe code; agents need to know what *not* to read. |

### C. A vector-indexed documentation search

| | |
|---|---|
| **Pros** | Scales to any size |
| **Cons** | Retrieval is probabilistic; the agent cannot know what it missed; another system to run |
| **Why rejected** | Deterministic, named files beat probabilistic retrieval when the file set is small enough to enumerate — and ours is. |

## Consequences

### Positive

- An agent reads roughly 10k tokens instead of 150k+ to start work correctly
- Fixed section order makes the documents skimmable and machine-checkable
- The §14 'Do NOT' section prevents the specific mistakes agents actually make
- Front-matter lets CI verify documentation against reality
- Human onboarding benefits identically

### Negative — accepted knowingly

- Thirty documents to keep current — mitigated by generation and drift checks
- Discipline required: an out-of-date `AGENT.md` is worse than none
- Some duplication between `AGENT.md` and code

## Compliance

`docs.yml` validates front-matter, required sections, table drift against migrations, dependency drift against `.go-arch-lint.yml`, and flags `last_verified` older than 90 days.

## Revisit when

Measured quarterly: files read before the first edit, rework rate on AI-authored PRs, median tokens per completed task. If the numbers do not improve, the format is wrong and should change.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
