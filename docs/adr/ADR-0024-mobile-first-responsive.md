---
adr: 0024
title: "Mobile-first responsive UI as a baseline requirement"
status: Accepted
date: 2026-08-06
tags: [frontend]
---

# ADR-0024: Mobile-first responsive UI as a baseline requirement

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | frontend |

## Context

Language learning is a habit built in short sessions, and those sessions happen on a phone — on a commute, in a queue, before bed. The original frontend specification treated responsiveness as an implicit quality attribute, which in practice means it is verified late, by hand, and inconsistently. Meanwhile several core interactions — recording speech, drilling review cards, typing an essay — have genuinely different ergonomics on a touch device.

## Decision

Adopt mobile-first as the development default rather than an adaptation: every component is built at the 375 px viewport first and enhanced upward. Enforce a concrete baseline — minimum 44×44 CSS px touch targets, 16 px minimum input font size, safe-area insets honoured, no hover-only interaction, virtual-keyboard-aware layout via `visualViewport`, bottom navigation on small viewports and a sidebar above `md`. Playwright runs every E2E journey across four device projects (iPhone-class, Android-class, tablet, desktop), and the performance budget is stated for a mid-tier Android device on 4G, not for a developer laptop.

## Alternatives considered

### A. Desktop-first, adapted down

| | |
|---|---|
| **Pros** | Matches how the team develops day to day |
| **Cons** | Mobile becomes a series of overrides and exceptions; touch ergonomics are discovered late; the layouts that break are the ones most learners see |
| **Why rejected** | It optimises for the environment the developers use rather than the one the learners use. |

### B. A separate mobile web application

| | |
|---|---|
| **Pros** | Each surface fully optimised |
| **Cons** | Two codebases, two release cycles, two sets of bugs, for one product |
| **Why rejected** | Unjustifiable at our team size, and the divergence starts immediately. |

### C. Build a native mobile app now

| | |
|---|---|
| **Pros** | Best possible mobile experience; push notifications; offline |
| **Cons** | A third codebase, app store review cycles, and a second API consumer before the web product has validated retention |
| **Why rejected** | Explicitly a year-one non-goal. A responsive web app is how we learn whether the product retains at all. |

### D. Responsive by convention, verified manually

| | |
|---|---|
| **Pros** | No tooling cost |
| **Cons** | Unenforced conventions decay; regressions are found by users |
| **Why rejected** | The same reasoning as every other rule in this repository — if CI does not check it, it is a suggestion. |

## Consequences

### Positive

- The primary usage context is the default development context, so mobile breakage is caught immediately
- Touch-target and font-size rules eliminate the two most common mobile usability defects before review
- Device-matrix E2E turns responsiveness from an opinion into a pass/fail signal
- A realistic performance budget prevents shipping something that only feels fast on a developer machine
- The same discipline prepares the UI for a future native shell without committing to one

### Negative — accepted knowingly

- Every component costs slightly more to build, because two layouts are considered from the start
- The E2E matrix multiplies runtime — mitigated by sharding and by running the full matrix nightly rather than on every push
- Some desktop-oriented admin screens gain little from the mobile constraint and still pay its cost

## Compliance

`eslint` rules and design-system primitives enforce touch-target and font-size minimums. Playwright device projects run in CI. The bundle and Web Vitals budgets are checked per pull request. A component merged without a mobile story fails review.

## Revisit when

If a native mobile application is built, at which point the web app's responsibility for small viewports may narrow — but not before retention on web is demonstrated.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
