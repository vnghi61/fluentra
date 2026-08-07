---
adr: 0023
title: "Google OAuth and the account-linking policy"
status: Accepted
date: 2026-08-06
tags: [security]
---

# ADR-0023: Google OAuth and the account-linking policy

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | security |

## Context

Social sign-in materially reduces registration friction, and Google covers the overwhelming majority of our target market. The difficulty is not the protocol; it is deciding what happens when a Google identity presents an email address that already exists locally. Getting that decision wrong is a well-known account-takeover path that appears in production systems regularly.

## Decision

Implement Google sign-in using the authorization code flow with PKCE, a server-side single-use `state`, and a `nonce` verified in the returned ID token, which is validated against Google's cached JWKS for signature, `iss`, `aud` and `exp`. Linking policy: link automatically **only** when Google asserts `email_verified: true` **and** the address matches an already-verified local account. If the matching local account is unverified, refuse with `OAUTH_ACCOUNT_CONFLICT` and require an OTP challenge on that address first. If no local account exists, create one already verified. Unlinking is refused when it would leave the account with no sign-in method.

## Alternatives considered

### A. Auto-link on any email match

| | |
|---|---|
| **Pros** | Smoothest possible experience |
| **Cons** | An attacker registers victim@example.com locally without verifying it, waits, and later signs in through Google to claim the account — or the reverse, depending on which side is created first |
| **Why rejected** | This is the takeover path. The convenience is real but it is bought with the account's integrity. |

### B. Never link — separate accounts per provider

| | |
|---|---|
| **Pros** | No linking logic at all; no takeover path |
| **Cons** | A learner who registered with a password and later taps 'Continue with Google' silently gets a second, empty account and loses their progress |
| **Why rejected** | It converts a security problem into a support problem, and the support problem is the one learners actually experience. |

### C. Implicit flow, or skipping PKCE as a confidential client

| | |
|---|---|
| **Pros** | Slightly less code |
| **Cons** | Implicit is deprecated; without PKCE an intercepted authorization code on a mobile browser is exchangeable |
| **Why rejected** | PKCE costs almost nothing to add and closes a real interception path on exactly the devices most of our learners use. |

### D. Include Apple sign-in now

| | |
|---|---|
| **Pros** | Covers iOS learners; required by the App Store when other social logins exist |
| **Cons** | Additional integration with different token semantics and a hide-my-email relay address, for no web benefit today |
| **Why rejected** | Deferred to whenever an iOS app is actually built, at which point it becomes a requirement rather than a nice-to-have. |

## Consequences

### Positive

- One tap to register on mobile, where typing a password is the highest-friction step
- Email verification is inherited from Google, so those accounts skip the OTP step entirely
- The takeover path is closed explicitly, and the refusal is a documented, testable branch rather than an accident
- State, nonce and PKCE together make CSRF and code interception concrete, tested defences rather than assumptions
- Every failure branch raises a security event, so an attempted takeover is visible in the security dashboard

### Negative — accepted knowingly

- A dependency on Google's availability for those learners' sign-in
- The `OAUTH_ACCOUNT_CONFLICT` branch requires clear UI copy or it reads as a bug to the learner
- More surface to test: five distinct linking outcomes, each with its own correct behaviour
- Learners who lose access to their Google account need a recovery path that does not go through Google

## Compliance

Integration tests cover all five linking outcomes plus forged, reused and expired `state`, and ID tokens failing each of signature, `iss`, `aud`, `exp` and `nonce`. A test asserts that no partial account is created on any failure branch.

## Revisit when

When an iOS application ships — Apple sign-in becomes an App Store requirement at that point, and the linking policy must be extended to its relay addresses.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
