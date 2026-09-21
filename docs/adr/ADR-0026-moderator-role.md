---
adr: 0026
title: "A third role: moderator"
status: Accepted
date: 2026-09-21
tags: [security]
---

# ADR-0026: A third role: moderator

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-21 |
| **Deciders** | Tech Lead |
| **Tags** | security |

## Context

Until Phase 3 the product had exactly two roles, `admin` and `user` (BR-RBAC-02), and a third
required an ADR. Work order 15 opened the marketplace: creators submit courses, and a human must
review and publish them. The permissions for that work — `content.review`, `content.publish`,
`moderation.read`, `moderation.act` — already existed and were granted only to `admin`, so the only
people who could review were people who could also move money, assign roles and read the audit log.

## Decision

Add a `moderator` role, seeded by `db/migrations/rbac/1700000748_add_moderator_role.sql`, granted
exactly those four permissions. Moderation routes are permission-gated, not `AdminOnly`, so a
moderator reaches the queue without holding `admin`.

A token still carries `admin` or `user` (`auth/domain.HighestRole`): a moderator's token says
`user`, and what they may do is read from the role tables on each `rbac.Require`, as ADR-0008 already
intends. The role is data, not a new code path.

## Alternatives considered

### A. Make reviewers admins

| | |
|---|---|
| **Pros** | No new role |
| **Cons** | Every reviewer can assign roles, run payouts and read the audit log |
| **Why rejected** | Review is the job most likely to be staffed widely; it is the one that must not carry admin. |

### B. Grant the review permissions to `user` behind a flag

| | |
|---|---|
| **Pros** | Still two roles |
| **Cons** | Grants write permissions to every learner, which BR-RBAC's "reading is a grant, writing is not" forbids |
| **Why rejected** | Breaks a rule in order to keep a count. |

## Consequences

### Positive

- Review can be staffed without handing out `admin`
- ADR-0008's promise holds: the role was a migration and a seed row, not code

### Negative — accepted knowingly

- BR-RBAC-02 now reads "exactly three"; a fourth role still needs an ADR
- The token's role claim no longer names every role an account holds; clients must use
  `/me/permissions`, not the claim, to decide what to show a moderator

## Revisit when

A role needs permissions scoped to particular resources (for example, moderators per topic). That is
ADR-0008's trigger too.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
