---
adr: 0012
title: "Prompts as versioned, evaluated artefacts"
status: Accepted
date: 2026-08-06
tags: [ai]
---

# ADR-0012: Prompts as versioned, evaluated artefacts

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | ai |

## Context

Prompts determine the quality of grading and feedback — the product's core value. A prompt change is as consequential as a code change and, unlike code, its effects are statistical rather than deterministic.

## Decision

Runtime prompts live in `docs/prompts/runtime/<task>/v<N>.md` with YAML front-matter and JSON input/output schemas. A published version is immutable; a change creates `vN+1`. Configuration pins the active version per environment. A version cannot become active until its eval suite meets its thresholds and is no worse than the current active version. Rollout is shadow → 10 % → 100 %.

## Alternatives considered

### A. Prompts as Go string constants

| | |
|---|---|
| **Pros** | Type-safe, co-located with the code |
| **Cons** | Not reviewable by non-engineers, not rollback-able without a deploy, not independently evaluable |
| **Why rejected** | It treats a statistical artefact as if it were deterministic code. |

### B. Prompts in a database, editable in an admin UI

| | |
|---|---|
| **Pros** | Change without a deploy |
| **Cons** | No review, no version control, no CI evaluation — the fastest available path to a silent quality regression |
| **Why rejected** | Speed of change is not the constraint; confidence in change is. |

### C. A third-party prompt management service

| | |
|---|---|
| **Pros** | Purpose-built tooling |
| **Cons** | Another vendor in a critical path; our prompts leave the repository |
| **Why rejected** | The repository already gives us versioning and review; we need the evaluation, which we build ourselves. |

## Consequences

### Positive

- A past grade can always be explained by the exact prompt that produced it
- Rollback is a configuration change
- Quality regressions are caught in CI rather than by learners
- Prompts are reviewable by the learning team, not only by engineers

### Negative — accepted knowingly

- Building and maintaining golden sets is real ongoing work
- Version proliferation over time
- Evaluation runs cost money and time

## Compliance

`ai-eval.yml` runs the suite on every change under `docs/prompts/runtime/` and blocks a merge on regression. Promotion to `active` requires a human approval.

## Revisit when

If eval maintenance cost exceeds the regressions it prevents — measured, not assumed.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
