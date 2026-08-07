---
adr: 0021
title: "Email OTP challenges instead of verification links"
status: Accepted
date: 2026-08-06
tags: [security]
---

# ADR-0021: Email OTP challenges instead of verification links

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-08-06 |
| **Deciders** | Principal Architect, Tech Lead |
| **Tags** | security |

## Context

Registration must prove control of an email address. The conventional mechanism is a signed link. In our market the majority of learners open email on a phone, where a link launches a second browser context with no session — the original tab is abandoned, the registration is not completed, and the learner is left in an unverified state they do not understand. The same primitive is also needed for password reset and, later, for step-up verification on an untrusted device.

## Decision

Build one generic challenge subsystem: `auth_challenges` rows carrying a `purpose` enum (`verify_email`, `login_otp`, `password_reset`, `link_oauth`), a 6-digit code stored as `HMAC-SHA256(code, server_key)`, a 10-minute TTL, a hard cap of 5 verification attempts, and single-use consumption. The client receives a `challenge_id`; the code goes to the email channel only. Resend has a 60-second cooldown and a cap of 3 issuances per subject per hour.

## Alternatives considered

### A. Signed verification links

| | |
|---|---|
| **Pros** | One click; no typing; higher entropy in the token |
| **Cons** | Breaks the mobile flow by switching browser context; link scanners in corporate mail systems consume single-use tokens before the human clicks; harder to explain when it fails |
| **Why rejected** | The context switch is not a minor annoyance — it is the dominant registration drop-off in mobile-first products. Link prefetching by mail scanners silently burning tokens is a second, harder-to-debug failure. |

### B. Magic-link login (no password at all)

| | |
|---|---|
| **Pros** | No password to forget or leak |
| **Cons** | Every sign-in depends on email delivery latency; account recovery becomes email recovery; still has the context-switch problem |
| **Why rejected** | It makes email an availability dependency on every single sign-in, not just on registration. |

### C. A separate table per purpose

| | |
|---|---|
| **Pros** | Simpler individual schemas |
| **Cons** | Rate limiting, hashing, attempt capping, expiry and burning duplicated three times |
| **Why rejected** | Three copies means three chances to get constant-time comparison or attempt capping wrong, and the bug would be silent in two of them. |

### D. SMS OTP

| | |
|---|---|
| **Pros** | Familiar; independent of email |
| **Cons** | Per-message cost; SIM-swap risk; carrier delivery variance; requires collecting phone numbers we otherwise do not need |
| **Why rejected** | More personal data, more cost, and a weaker channel than email for our threat model. |

## Consequences

### Positive

- The learner never leaves the tab they started in — the flow completes on one screen, on any device
- One implementation serves verification, password reset and future step-up, so the security-critical parts are written and tested once
- Mail scanners cannot consume a code by prefetching a URL
- A wrong code gives immediate, understandable feedback with a remaining-attempts count
- The code alone is useless without the `challenge_id`, which narrows the attack surface considerably

### Negative — accepted knowingly

- 10^6 of entropy is far less than a signed token — the attempt cap and rate limiters are load-bearing, not decorative
- Two API round trips instead of one link click
- The learner must switch to their mail app and back, and type six digits
- A distributed guessing campaign across many challenges needs a global limiter to catch; the per-challenge cap alone is insufficient

## Compliance

Integration tests assert: single use, expiry, exactly 5 attempts then burn, constant-time comparison, cross-challenge code rejection, resend cooldown, and that the code appears in no response body, log record or span attribute. The global per-IP issuance limiter has its own test.

## Revisit when

If measured registration completion does not improve over the link-based baseline, or if email delivery latency makes the 10-minute window impractical for a significant share of learners.

---

*Index: [/DECISIONS.md](../../DECISIONS.md) · Template: [/docs/templates/adr.md](../templates/adr.md)*
