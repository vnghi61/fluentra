---
adr: 0014
title: "React + Vite + TanStack + shadcn/ui"
status: Accepted
date: 2026-08-06
tags: [frontend]
---

# ADR-0014: React + Vite + TanStack + shadcn/ui

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | frontend |

## Context

A single-page application behind a login, with rich interactive exercises, audio, a rich-text editor, and an admin back office. No SEO requirement.

## Decision

React 19 with TypeScript on Vite; TanStack Router and TanStack Query; Zustand for the small amount of true global state; shadcn/ui on Radix and Tailwind; React Hook Form with Zod; Vitest, Testing Library, MSW and Playwright.

## Alternatives considered

### A. Next.js

| | |
|---|---|
| **Pros** | SSR, routing, image optimisation, large ecosystem |
| **Cons** | SSR complexity we do not need behind a login; a Node server to operate alongside the Go API; framework opinions we would fight |
| **Why rejected** | We are building an application, not a content site. |

### B. A component library (MUI, Mantine, Ant)

| | |
|---|---|
| **Pros** | Comprehensive; fast initially |
| **Cons** | Theming fights; components are in `node_modules` where neither we nor an agent can read or adapt them; heavy bundles |
| **Why rejected** | shadcn/ui puts the components in our repository, which matters enormously for AI-assisted work — an agent can read and modify the button. |

### C. Svelte or SolidJS

| | |
|---|---|
| **Pros** | Smaller bundles, better raw performance |
| **Cons** | Smaller talent pool; less training data, so AI assistance is measurably weaker |
| **Why rejected** | Model familiarity is a real factor in a project designed around AI-assisted development. |

## Consequences

### Positive

- Fast development loop; no server-side rendering complexity
- End-to-end type safety from the OpenAPI spec
- Design-system components live in the repository and are editable
- TanStack Query removes an entire category of client-state bugs
- Excellent AI assistance quality for React and TypeScript

### Negative — accepted knowingly

- No SEO (mitigated: marketing lives on a separate site)
- We maintain the design system components ourselves
- TanStack Router is newer and less widely known than React Router

## Compliance

`eslint-plugin-boundaries` enforces feature-slice isolation. Hand-written API types fail review. The bundle budget is enforced in CI.

## Revisit when

If a public, SEO-relevant surface becomes part of this application rather than the marketing site.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
