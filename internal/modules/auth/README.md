---
module: auth
tier: core
group: modules
status: PLANNED
phase: 1
owner: "@backend-team"
schema: core
tables: [credentials, sessions, refresh_tokens, mfa_secrets, auth_challenges, trusted_devices, login_attempts, oauth_identities, oauth_states]
depends_on: [user, rbac, audit, mailer, cache]
depended_on_by: [admin]
spec_version: 1.0.0
last_verified: 2026-08-06
---

# auth

Owns everything about proving who a caller is: registration, email verification, login, multi-factor authentication, token issuance and rotation, session lifecycle, password reset, and OAuth sign-in. It answers "who is this?" and nothing else.

> **AI assistants: read [`AGENT.md`](AGENT.md) instead — it has everything this file has, structured for you.**

## Business purpose

<!-- BEGIN GENERATED: purpose -->
Learners must be able to create an account and return to it safely from any device. Administrators must be held to a higher standard (mandatory MFA) because they can see and change everyone's data. Every authentication event must be reconstructable after the fact.
<!-- END GENERATED: purpose -->

## Responsibilities

<!-- BEGIN GENERATED: readme-resp -->
- Registration and email verification by one-time code (OTP)
- The generic challenge subsystem: issue, resend, verify and burn short-lived codes for any purpose
- Password hashing (Argon2id), verification, and policy enforcement including breached-password checks
- Access token (JWT, 15 min) issuance and validation
- Refresh token issuance, **sliding** rotation, reuse detection and family revocation
- Persistent sign-in: trusted devices, sliding idle window, absolute re-authentication cap
- Session records: creation, listing, revocation (by the user and by an admin)
- TOTP multi-factor enrolment, verification and recovery codes
- Password reset and change flows
- Google OAuth sign-in (authorization code + PKCE) and account linking by verified email only
- Brute-force protection: per-IP, per-account and per-challenge lockout
- Emitting security events for the audit trail
<!-- END GENERATED: readme-resp -->

## Where things are

<!-- BEGIN GENERATED: readme-folders -->
| Path | Contains |
|---|---|
| `contract/` | Interfaces, DTOs and event types other modules may import — the only public package |
| `domain/` | Entities, value objects, invariants, domain errors. Pure Go, no I/O |
| `service/` | Use cases, orchestration, transactions, event publishing |
| `repository/` | sqlc-generated queries and row↔domain mappers |
| `transport/http/` | Handlers, request/response DTOs, route registration |
| `job/` | Background job handlers owned by this module |
| `module.go` | `New(deps)` — wiring; the only symbol `cmd/` imports |
<!-- END GENERATED: readme-folders -->

## Documentation set

| File | Contents |
|---|---|
| [AGENT.md](AGENT.md) | Complete AI-agent context (start here) |
| [API.md](API.md) | Endpoint reference |
| [FLOW.md](FLOW.md) | Sequence and state diagrams |
| [TESTING.md](TESTING.md) | Test plan |
| [DECISIONS.md](DECISIONS.md) | Module-local decisions |
| [PROMPTS.md](PROMPTS.md) | Prompts for and from this module |
| [TODO.md](TODO.md) | Backlog |

## Status

**PLANNED** — planned for delivery phase 1. See [/ROADMAP.md](../../../ROADMAP.md).
