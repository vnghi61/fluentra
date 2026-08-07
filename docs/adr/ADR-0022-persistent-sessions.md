---
adr: 0022
title: "Persistent sign-in: sliding idle window with an absolute cap"
status: Accepted
date: 2026-08-06
tags: [security]
---

# ADR-0022: Persistent sign-in: sliding idle window with an absolute cap

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | security |

## Context

Learners study in short daily sessions. Being asked to sign in again is friction at exactly the moment habit formation matters most, and it is the single most common complaint about consumer learning apps. The product requirement is 'sign in once and stay signed in'. The security requirement is that every credential has a bounded lifetime and a working revocation path. These are usually presented as opposites.

## Decision

Refresh tokens become **sliding**: each rotation issues a replacement with a fresh idle window rather than inheriting the original expiry. Each session additionally carries an **absolute** expiry that activity does not extend. Idle windows are 30 days by default and 90 days on a device the learner explicitly chose to trust; the absolute cap is 180 days. Administrator accounts receive neither extension — 12-hour idle, 7-day absolute. Trusted devices are listed in the account and revocable from anywhere; a password change, reset or suspension revokes all of them.

## Alternatives considered

### A. A long-lived or non-expiring 'remember me' token

| | |
|---|---|
| **Pros** | Trivially satisfies the requirement |
| **Cons** | An immortal credential; a stolen cookie is valid forever; no natural point at which possession is re-proven |
| **Why rejected** | It cannot be reasoned about. 'When does this stop being dangerous?' has no answer, and that is disqualifying for a credential. |

### B. Fixed 30-day expiry, no sliding

| | |
|---|---|
| **Pros** | Simplest to reason about |
| **Cons** | A learner active every single day is still signed out on day 31, for no security benefit — their possession was re-proven daily |
| **Why rejected** | It imposes the cost of expiry without the corresponding benefit; expiry should mean 'we have not seen you in a while', not 'time passed'. |

### C. Sliding window with no absolute cap

| | |
|---|---|
| **Pros** | The learner truly never signs in again |
| **Cons** | A stolen token that is used regularly renews itself indefinitely — the theft becomes permanent and invisible |
| **Why rejected** | The absolute cap is precisely what bounds a successful theft. Without it, sliding is the immortal-token option wearing a different name. |

## Consequences

### Positive

- An actively studying learner never sees a login screen again, which is the actual product requirement
- Every credential still has two independent bounds: inactivity and absolute age
- A stolen refresh cookie expires at the absolute cap even if the attacker keeps it alive
- Trusted devices are visible and individually revocable, so the learner can act on a lost phone
- Reuse detection from ADR-0007 continues to apply unchanged, so theft remains *detectable* as well as bounded
- Admin accounts are explicitly excluded, so the highest-privilege sessions stay short

### Negative — accepted knowingly

- A longer idle window is a longer exposure window for a stolen cookie
- More state per session: idle window, absolute expiry, device linkage
- A learner who clears browser storage looks like a new device and must sign in again
- The interaction between sliding rotation and the absolute cap needs explicit tests, or one of the two bounds silently stops applying

## Compliance

Integration tests assert that rotation moves the idle window forward but never past the absolute expiry; that the absolute expiry forces re-authentication even under continuous activity; that admin sessions do not receive the extended window; and that password change, reset and suspension revoke every trusted device.

## Revisit when

If session-theft incidents occur, or if a compliance requirement imposes a shorter maximum. The three numbers (30/90/180 days) are configuration and can be tightened without a code change.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
